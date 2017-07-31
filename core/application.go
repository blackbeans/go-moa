package core

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"

	"github.com/blackbeans/go-moa/proto"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"github.com/blackbeans/turbo/server"
)

type ServiceBundle func() []Service

type Application struct {
	remoting      *server.RemotingServer
	invokeHandler *InvocationHandler
	options       *MOAOption
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

	cloneServs := make([]Service, 0, len(services))

	for i, s := range services {
		s.GroupId = options.groupId
		services[i] = s
		cloneServs = append(cloneServs, s)
	}

	name := options.name + "/" + options.hostport
	rc := turbo.NewRemotingConfig(name,
		options.maxDispatcherSize,
		options.readBufferSize,
		options.readBufferSize,
		options.writeChannelSize,
		options.readChannelSize,
		options.idleDuration,
		50*10000)

	//需要开发对应的codec
	cf := func() codec.ICodec {
		return proto.BinaryCodec{64 * 1024}
	}

	//创建注册服务
	configCenter := NewConfigCenter(options.registryHosts, options.hostport, options.groupId, services)

	app := &Application{}
	app.options = options
	app.configCenter = configCenter
	//moastat
	moaStat := NewMoaStat(options.hostport, services[0].ServiceUri, monitor, func() turbo.NetworkStat {
		return app.remoting.NetworkStat()

	})
	app.moaStat = moaStat

	app.invokeHandler = NewInvocationHandler(services, moaStat)

	//启动remoting
	remoting := server.NewRemotionServerWithCodec(options.hostport, rc, cf,
		func(remoteClient *client.RemotingClient, p *packet.Packet) {
			packetDispatcher(app, remoteClient, p)
		})
	app.remoting = remoting
	remoting.ListenAndServer()
	moaStat.StartLog()

	//------------启动pprof
	go func() {
		hp, _ := net.ResolveTCPAddr("tcp4", options.hostport)
		pprof := fmt.Sprintf("%s:%d", hp.IP, (hp.Port + 1000))
		log.ErrorLog("moa-server", http.ListenAndServe(pprof, nil))

	}()

	//注册服务
	configCenter.RegisteAllServices()
	log.InfoLog("moa-server", "Application|Start|SUCC|%s|%s", name, options.hostport)
	return app
}

func (self Application) DestroyApplication() {

	//取消注册服务
	self.configCenter.Destroy()
	//关闭remoting
	self.remoting.Shutdown()
}

//需要开发对应的分包
func packetDispatcher(self *Application, remoteClient *client.RemotingClient, p *packet.Packet) {

	defer func() {
		if err := recover(); nil != err {
			log.ErrorLog("moa-server", "Application|packetDispatcher|FAIL|%s", err)
		}
	}()

	//如果是get命令
	if p.Header.CmdType == proto.REQ {

		req := p.PayLoad.(proto.MoaRawReqPacket)

		//这里面根据解析包的内容得到调用不同的service获得结果
		req.Source = remoteClient.RemoteAddr()
		req.Channel = remoteClient.AttachChannel
		req.Timeout = self.options.processTimeout
		result := self.invokeHandler.Invoke(&req)

		resp := packet.NewRespPacket(p.Header.Opaque, proto.RESP, nil)
		resp.PayLoad = *result
		if nil != result && result.ErrCode == proto.CODE_TIMEOUT_SERVER {
			//如果是超时的结果那么久直接
			log.ErrorLog("moa-server", "Application|Invoke|Timeout|%v", req)
			remoteClient.Write(*resp)
			return
		}

		remoteClient.Write(*resp)
		//log.DebugLog("moa-server", "Application|packetDispatcher|SUCC|%s", *resp)

	} else if p.Header.CmdType == proto.PING {
		//PING 协议
		resp, err := proto.Wrap2ResponsePacket(p, "PONG")
		if nil != err {
			log.ErrorLog("moa-server", "Application|PongResponse|FAIL|%v|%v|%s",
				err, p.Header, remoteClient.RemoteAddr())
			return
		}
		remoteClient.Write(*resp)
	} else if p.Header.CmdType == proto.INFO {
		//INFO 协议，返回服务端信息
		stat := make(map[string]interface{}, 2)
		stat["network"] = self.remoting.NetworkStat()
		stat["moa"] = self.moaStat.GetMoaInfo()
		resp, err := proto.Wrap2ResponsePacket(p, stat)
		if nil != err {
			log.ErrorLog("moa-server", "Application|PongResponse|FAIL|%v|%v|%s",
				err, p.Header, remoteClient.RemoteAddr())
			remoteClient.Shutdown()
			return
		}
		remoteClient.Write(*resp)
	}

}
