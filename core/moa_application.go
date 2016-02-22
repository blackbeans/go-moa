package core

import (
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"github.com/blackbeans/turbo/server"
	"go-moa/protocol"
)

type ServiceBundle func() []Service

type Application struct {
	remoting      *server.RemotingServer
	invokeHandler *InvocationHandler
	options       *MOAOption
}

func NewApplcation(configPath string, bundle ServiceBundle) *Application {
	services := bundle()
	options, err := LoadConfiruation(configPath)
	if nil != err {
		panic(err)
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
		return RedisGetCodec{32 * 1024}
	}
	//注册服务
	app := &Application{}
	app.options = options
	app.invokeHandler = NewInvocationHandler(services)
	//启动remoting
	remoting := server.NewRemotionServerWithCodec(options.hostport, rc, cf, app.packetDispatcher)
	app.remoting = remoting

	remoting.ListenAndServer()
	log.InfoLog("moa-server", "Application|Start|SUCC|%s", name)
	return app
}

func (self Application) DestoryApplication() {
	//关闭remoting
	self.remoting.Shutdown()
}

//需要开发对应的分包
func (self Application) packetDispatcher(remoteClient *client.RemotingClient, p *packet.Packet) {

	defer func() {
		if err := recover(); nil != err {
			log.ErrorLog("moa-server", "Application|packetDispatcher|FAIL|%s", err)
		}
	}()

	//这里面根据解析包的内容得到调用不同的service获得结果
	req, err := protocol.Wrap2MoaRequest(p.Data)
	if nil != err {
		log.ErrorLog("moa-server", "Application|packetDispatcher|Wrap2MoaRequest|FAIL|%s|%s", err, string(p.Data))
	} else {

		req.Timeout = self.options.processTimeout
		result := self.invokeHandler.Invoke(*req)
		resp, err := protocol.Wrap2ResponsePacket(p, result)
		if nil != err {
			log.ErrorLog("moa-server", "Application|packetDispatcher|Wrap2ResponsePacket|FAIL|%s|%s", err, result)
		} else {
			log.DebugLog("moa-server", "Application|packetDispatcher|SUCC|%s", *resp)
			remoteClient.Write(*resp)
		}

	}
}
