package core

import (
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"github.com/blackbeans/turbo/server"
)

type Service struct {
	serviceUri string
	instance   interface{}
}

type ServiceBundle func() []Service

type Application struct {
	remoting *server.RemotingServer
}

func NewApplcation(configPath string, bundle ServiceBundle) *Application {
	// services := bundle()
	options, err := LoadConfiruation(configPath)
	if nil != err {
		panic(err)
	}
	rc := turbo.NewRemotingConfig(options.name+"/"+options.hostport,
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

	//需要开发对应的分包
	packetDispatcher := func(remoteClient *client.RemotingClient, p *packet.Packet) {
		//这里面根据解析包的内容得到调用不同的service获得结果
		remoteClient.Write(*p)
	}

	//启动remoting
	remoting := server.NewRemotionServerWithCodec(options.hostport, rc, cf, packetDispatcher)
	remoting.ListenAndServer()

	//注册服务
	app := &Application{remoting}

	return app
}

func (self Application) DestoryApplication() {
	//关闭remoting
	self.remoting.Shutdown()
}
