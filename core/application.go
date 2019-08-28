package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"

	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"time"
)

type ServiceBundle func() []Service

type Application struct {
	remoting      *turbo.TServer
	invokeHandler *InvocationHandler
	options       Option
	//任务处理
	invokePool   *turbo.GPool
	configCenter *ConfigCenter
	moaStat      *MoaStat
	stop         context.CancelFunc
}

func NewApplication(configPath string, bundle ServiceBundle) *Application {
	return NewApplicationWithAlarm(configPath, bundle,
		func(serviceUri, hostname string, moaInfo MoaInfo) {
			//do nothing
		})
}

//with alarm
func NewApplicationWithAlarm(configPath string, bundle ServiceBundle,
	monitor func(serviceUri, host string, moainfo MoaInfo)) *Application {
	services := bundle()

	options, err := LoadConfiguration(configPath)
	if nil != err {
		panic(err)
	}

	serverOp := InitServerOption(options)

	cloneServs := make([]Service, 0, len(services))

	for i, s := range services {
		//服务分默认不配置是使用*分组
		if len(s.GroupId) <= 0 {
			s.GroupId = "*"
		}
		services[i] = s
		cloneServs = append(cloneServs, s)
	}

	name := serverOp.Server.BindAddress
	cluster := serverOp.Clusters[serverOp.Server.RunMode]
	config := turbo.NewTConfig(name,
		cluster.MaxDispatcherSize,
		cluster.ReadBufferSize,
		cluster.ReadBufferSize,
		cluster.WriteChannelSize,
		cluster.ReadChannelSize,
		cluster.IdleTimeout,
		50*10000)

	ctx, cancel := context.WithCancel(context.Background())
	gopool := turbo.NewLimitPool(
		ctx,
		config.TW,
		cluster.MaxDispatcherSize)

	//是否启用snappy
	snappy := false
	if strings.ToLower(serverOp.Server.Compress) == "snappy" {
		snappy = true
	}

	//需要开发对应的codec
	codec := func() turbo.ICodec {
		return BinaryCodec{MaxFrameLength: turbo.MAX_PACKET_BYTES, SnappyCompress: snappy}
	}

	//创建注册服务
	configCenter := NewConfigCenter(cluster.Registry,
		serverOp.Server.BindAddress,
		services)

	app := &Application{}
	app.options = options
	app.configCenter = configCenter
	app.invokePool = gopool
	app.stop = cancel

	//moastat
	moaStat := NewMoaStat(serverOp.Server.BindAddress,
		services[0].ServiceUri, gopool,
		monitor,
		func() turbo.NetworkStat {
			return app.remoting.NetworkStat()

		})
	app.moaStat = moaStat

	app.invokeHandler = NewInvocationHandler(services, moaStat)

	//启动remoting
	remoting := turbo.NewTServerWithCodec(
		serverOp.Server.BindAddress,
		config,
		codec,
		func(ctx *turbo.TContext) error {
			dis(app, ctx)
			return nil
		})
	app.remoting = remoting
	err = remoting.ListenAndServer()
	if nil != err {
		panic(err)
	}
	moaStat.StartLog()

	//------------启动pprof
	go func() {
		hp, _ := net.ResolveTCPAddr("tcp4", serverOp.Server.BindAddress)
		pprof := fmt.Sprintf("%s:%d", hp.IP, (hp.Port + 1000))
		log.ErrorLog("moa_server", http.ListenAndServe(pprof, nil))

	}()

	//注册服务
	configCenter.RegisteAllServices()
	log.InfoLog("moa_server", "Application|Start|SUCC|%s|%s", name, serverOp.Server.BindAddress)
	return app
}

func (self Application) DestroyApplication() {

	self.stop()

	//取消注册服务
	self.configCenter.Destroy()
	//关闭remoting
	self.remoting.Shutdown()
}

//需要开发对应的分包
func dis(self *Application, ctx *turbo.TContext) {

	defer func() {
		if err := recover(); nil != err {
			log.ErrorLog("moa_server", "Application|packetDispatcher|FAIL|%s", err)
		}
	}()

	p := ctx.Message
	//如果是错误的，那么久直接写出错误的响应给客户端
	if nil != ctx.Err {
		resp := turbo.NewRespPacket(p.Header.Opaque, RESP, nil)
		resp.PayLoad = MoaRespPacket{ErrCode: CODE_THROWABLE, Message: fmt.Sprintf("%v", ctx.Err)}
		//需要发送调用的错误给客户端
		log.ErrorLog("moa_server", "Application|Err|Process|%v", resp)
		ctx.Client.Write(*resp)
		return
	}

	//如果是get命令
	if p.Header.CmdType == REQ {

		req := p.PayLoad.(MoaRawReqPacket)

		//这里面根据解析包的内容得到调用不同的service获得结果
		req.Source = ctx.Client.RemoteAddr()
		req.Timeout = self.options.Clusters[self.options.Server.RunMode].ProcessTimeout
		//是否已经超时过期了，那么久不用执行调用了
		if req.CreateTime > 0 && (time.Now().UnixNano()-req.CreateTime*int64(time.Millisecond)) >= int64(req.Timeout) {
			log.WarnLog("moa_server", "InvocationHandler|Invoke|Timeout|Source:%s|Timeout[%d]ms|%s|%s",
				req.Source, req.Timeout/time.Millisecond, req.ServiceUri, req.Params.Method)
		} else {
			//全异步
			self.invokePool.Queue(func(cctx context.Context) (interface{}, error) {

				self.invokeHandler.Invoke(req, func(resp MoaRespPacket) error {
					//正常
					respPacker := turbo.NewRespPacket(ctx.Message.Header.Opaque, RESP, nil)
					respPacker.PayLoad = resp
					if resp.ErrCode != 0 && resp.ErrCode != CODE_SERVER_SUCC {
						//需要发送调用的错误给客户端
						log.ErrorLog("moa_server", "Application|Invoke|FAIL|%v", ctx.Message)
					}
					return ctx.Client.Write(*respPacker)
				})
				return nil, nil
			}, req.Timeout)

		}
		//log.DebugLog("moa_server", "Application|packetDispatcher|SUCC|%s", *resp)

	} else if p.Header.CmdType == PING {
		//PING 协议
		pipo, ok := p.PayLoad.(PiPo)
		if ok {
			ctx.Client.Pong(p.Header.Opaque, pipo.Timestamp)
		}
		resp := turbo.NewRespPacket(p.Header.Opaque, PONG, nil)
		resp.PayLoad = pipo
		ctx.Client.Write(*resp)
	} else if p.Header.CmdType == INFO {
		//INFO 协议，返回服务端信息
		stat := make(map[string]interface{}, 2)
		stat["network"] = self.remoting.NetworkStat()
		stat["moa"] = self.moaStat.GetMoaInfo()
		resp := turbo.NewRespPacket(p.Header.Opaque, INFO, nil)
		resp.PayLoad = stat
		ctx.Client.Write(*resp)
	}

}
