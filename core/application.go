package core

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/blackbeans/go-moa/proto"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
)

type ServiceBundle func() []Service

type Application struct {
	remoting      *turbo.TServer
	invokeHandler *InvocationHandler
	options       Option
	configCenter  *ConfigCenter
	moaStat       *MoaStat
}

func NewApplcation(configPath string, bundle ServiceBundle) *Application {
	return NewApplicationWithAlarm(configPath, bundle,
		func(serviceUri, hostname string, moaInfo MoaInfo) {
			//do nothing
		})
}

//with alarm
func NewApplicationWithAlarm(configPath string, bundle ServiceBundle,
	monitor func(serviceUri, host string, moainfo MoaInfo)) *Application {
	services := bundle()

	options, err := LoadConfiruation(configPath)
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

	//是否启用snappy
	snappy := false
	if strings.ToLower(serverOp.Server.Compress) == "snappy" {
		snappy = true
	}

	//需要开发对应的codec
	codec := func() turbo.ICodec {
		return proto.BinaryCodec{MaxFrameLength: 64 * 1024, SnappyCompress: snappy}
	}

	//创建注册服务
	configCenter := NewConfigCenter(cluster.Registry,
		serverOp.Server.BindAddress,
		services)

	app := &Application{}
	app.options = options
	app.configCenter = configCenter
	//moastat
	moaStat := NewMoaStat(serverOp.Server.BindAddress,
		services[0].ServiceUri, monitor, func() turbo.NetworkStat {
			return app.remoting.NetworkStat()

		})
	app.moaStat = moaStat

	app.invokeHandler = NewInvocationHandler(services, moaStat)

	//启动remoting
	remoting := turbo.NewTServerWithCodec(
		serverOp.Server.BindAddress,
		config,
		codec,
		func(ctx *turbo.TContext) error{
			packetDispatcher(app,ctx)
			return nil
		},)
	app.remoting = remoting
	remoting.ListenAndServer()
	moaStat.StartLog()

	//------------启动pprof
	go func() {
		hp, _ := net.ResolveTCPAddr("tcp4", serverOp.Server.BindAddress)
		pprof := fmt.Sprintf("%s:%d", hp.IP, (hp.Port + 1000))
		log.ErrorLog("moa-server", http.ListenAndServe(pprof, nil))

	}()

	//注册服务
	configCenter.RegisteAllServices()
	log.InfoLog("moa-server", "Application|Start|SUCC|%s|%s", name, serverOp.Server.BindAddress)
	return app
}

func (self Application) DestroyApplication() {

	//取消注册服务
	self.configCenter.Destroy()
	//关闭remoting
	self.remoting.Shutdown()
}

//需要开发对应的分包
func packetDispatcher(self *Application, ctx *turbo.TContext) {

	defer func() {
		if err := recover(); nil != err {
			log.ErrorLog("moa-server", "Application|packetDispatcher|FAIL|%s", err)
		}
	}()

	p := ctx.Message
	//如果是get命令
	if p.Header.CmdType == proto.REQ {

		req := p.PayLoad.(proto.MoaRawReqPacket)

		//这里面根据解析包的内容得到调用不同的service获得结果
		req.Source = ctx.Client.RemoteAddr()
		req.Timeout = self.options.Clusters[self.options.Server.RunMode].ProcessTimeout
		result := self.invokeHandler.Invoke(&req)

		resp := turbo.NewRespPacket(p.Header.Opaque, proto.RESP, nil)
		resp.PayLoad = *result
		if nil != result && result.ErrCode == proto.CODE_TIMEOUT_SERVER {
			//如果是超时的结果那么久直接
			log.ErrorLog("moa-server", "Application|Invoke|Timeout|%v", req)
			ctx.Client.Write(*resp)
			return
		}

		ctx.Client.Write(*resp)
		//log.DebugLog("moa-server", "Application|packetDispatcher|SUCC|%s", *resp)

	} else if p.Header.CmdType == proto.PING {
		//PING 协议
		pipo, ok := p.PayLoad.(proto.PiPo)
		if ok {
			ctx.Client.Pong(p.Header.Opaque, pipo.Timestamp)
		}
		resp := turbo.NewRespPacket(p.Header.Opaque, proto.PONG, nil)
		resp.PayLoad = pipo
		ctx.Client.Write(*resp)
	} else if p.Header.CmdType == proto.INFO {
		//INFO 协议，返回服务端信息
		stat := make(map[string]interface{}, 2)
		stat["network"] = self.remoting.NetworkStat()
		stat["moa"] = self.moaStat.GetMoaInfo()
		resp, err := proto.Wrap2ResponsePacket(p, stat)
		if nil != err {
			log.ErrorLog("moa-server", "Application|PongResponse|FAIL|%v|%v|%s",
				err, p.Header, ctx.Client.RemoteAddr())
			ctx.Client.Shutdown()
			return
		}
		ctx.Client.Write(*resp)
	}

}
