package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/gops/agent"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"html/template"
	"net"
	"net/http"
	"net/http/pprof"
	"sort"
	"strings"

	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"time"
)

type MoaProfile struct {
	Href string
	Name string
	Desc string
}

var profiles []MoaProfile

func init() {
	profiles = []MoaProfile{
		MoaProfile{Name: "index", Href: "/debug/moa", Desc: "MOA首页"},
		MoaProfile{Name: "stat", Href: "/debug/moa/stat", Desc: "MOA系统状态指标"},
		MoaProfile{Name: "list.clients", Href: "/debug/moa/list/clients", Desc: "MOA当前所有连接"},
		MoaProfile{Name: "list.services", Href: "/debug/moa/list/services", Desc: "MOA发布的服务列表"},
		MoaProfile{Name: "list.methods", Href: "/debug/moa/list/methods", Desc: "MOA来源调用统计信息"},
		MoaProfile{Name: "metrics", Href: "/metrics", Desc: "prometheus metrics"},
	}
}

type ServiceBundle func() []Service

type Application struct {
	http.Handler
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
		//是否是预发环境
		s.IsPre = serverOp.Server.IsPre
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

	// tracing
	if !opentracing.IsGlobalTracerRegistered() {
		// 如果 global tracer 还没有注册，我们就生成一个
		// 默认发送 span 到有可能不存在的 localhost上的 agent
		cfg := jaegercfg.Configuration{
			ServiceName: "moa-server",
			Sampler: &jaegercfg.SamplerConfig{
				Type:  jaeger.SamplerTypeConst,
				Param: 1,
			},
			Reporter: &jaegercfg.ReporterConfig{
				//LogSpans: true,
			},
		}
		//jLogger := jaegerlog.StdLogger
		tracer, _, _ := cfg.NewTracer(
		//jaegercfg.Logger(jLogger),
		)
		opentracing.SetGlobalTracer(tracer)
	}

	ctx, cancel := context.WithCancel(context.Background())
	invokePool := turbo.NewLimitPool(
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
	app.invokePool = invokePool
	app.stop = cancel
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

	//moastat
	moaStat := NewMoaStat(serverOp.Server.BindAddress,
		services[0].ServiceUri, invokePool,
		monitor,
		func() turbo.NetworkStat {
			return app.remoting.NetworkStat()

		})
	app.moaStat = moaStat

	app.invokeHandler = NewInvocationHandler(services, moaStat)
	moaStat.StartLog()

	//------------启动pprof
	// 启动 moa 系统指标状态 暴露http接口
	// 包括 pprof、自定义moa状态信息、prometheus metrics
	go func() {
		hp, _ := net.ResolveTCPAddr("tcp4", serverOp.Server.BindAddress)
		pprofListen := fmt.Sprintf("%s:%d", hp.IP, hp.Port+1000)

		for _, pro := range profiles {
			http.HandleFunc(pro.Href, app.ServeHTTP)
		}
		log.ErrorLog("moa", http.ListenAndServe(pprofListen, nil))
		if err := agent.Listen(agent.Options{ShutdownCleanup: true}); err != nil {
			log.ErrorLog("handler", "Gops Start  FAIL%s ...")
		}
	}()

	//注册服务
	configCenter.RegisteAllServices()
	log.InfoLog("moa", "Application|Start|SUCC|%s|%s", name, serverOp.Server.BindAddress)

	config.TW.RepeatedTimer(60*time.Second, func(t time.Time) {
		allclients := remoting.ListClients()
		sort.Strings(allclients)
		for _, inst := range app.invokeHandler.instances {
			removeClients := make([]string, 0, 2)
			inst.InvokesPerClient.Range(func(key, value interface{}) bool {
				clientip := key.(string)
				idx := sort.SearchStrings(allclients, clientip)
				if idx == len(allclients) || allclients[idx] != clientip {
					removeClients = append(removeClients, clientip)
				}
				return true
			})
			//移除
			for _, clientip := range removeClients {
				inst.InvokesPerClient.Delete(clientip)
			}
		}
	}, nil)
	return app
}

func (self Application) DestroyApplication() {

	//取消注册服务
	self.configCenter.Destroy()

	// 通过查看 moaStat 中的Connections数量来确保当前没有连接
	// 每秒检查一次，等待 10s
	checkTimes := 0
	for checkTimes < 9 {
		log.InfoLog("moa", "Application|DestroyApplication|WaitProcess|Times:%d|Conns:%d", checkTimes, self.moaStat.preMoaInfo.Connections)
		if self.moaStat.preMoaInfo.Connections == 0 {
			log.InfoLog("moa", "Application|DestroyApplication|WaitProcess|Done")
			break
		}
		time.Sleep(time.Second)
		checkTimes += 1
	}

	self.stop()
	time.Sleep(500 * time.Millisecond)

	//关闭remoting
	self.remoting.Shutdown()
}

//需要开发对应的分包
func dis(self *Application, ctx *turbo.TContext) {

	defer func() {
		if err := recover(); nil != err {
			log.ErrorLog("moa", "Application|packetDispatcher|FAIL|%s", err)
		}
	}()

	p := ctx.Message
	//如果是错误的，那么久直接写出错误的响应给客户端
	if nil != ctx.Err {
		resp := turbo.NewRespPacket(p.Header.Opaque, RESP, nil)
		resp.PayLoad = MoaRespPacket{ErrCode: CODE_THROWABLE, Message: fmt.Sprintf("%v", ctx.Err)}
		//需要发送调用的错误给客户端
		log.ErrorLog("moa", "Application|Err|Process|%v", resp)
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
			log.WarnLog("moa", "InvocationHandler|Invoke|Timeout|Source:%s|Timeout[%d]ms|%s|%s",
				req.Source, req.Timeout/time.Millisecond, req.ServiceUri, req.Params.Method)
		} else {
			//全异步
			self.invokePool.Queue(func(cctx context.Context) (interface{}, error) {
				//设置当前的调用的属性线程上下文
				invokeCtx := context.WithValue(cctx, KEY_MOA_PROPERTIES, req.Properties)
				self.invokeHandler.Invoke(invokeCtx, req, func(resp MoaRespPacket) error {
					respPacker := turbo.NewRespPacket(ctx.Message.Header.Opaque, RESP, nil)
					respPacker.PayLoad = resp
					if resp.ErrCode != 0 && resp.ErrCode != CODE_SERVER_SUCC {
						//需要发送调用的错误给客户端
						log.ErrorLog("moa", "InvocationHandler|Invoke|FAIL|Source:%s|Timeout[%d]ms|%s|%s|%d",
							req.Source, req.Timeout/time.Millisecond, req.ServiceUri, req.Params.Method, resp.ErrCode)
					}
					return ctx.Client.Write(*respPacker)
				})
				return nil, nil
			}, req.Timeout)

		}
		//log.DebugLog("moa", "Application|packetDispatcher|SUCC|%s", *resp)

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

//处理Moa的状态信息
func (self *Application) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	//moa的处理
	if strings.HasPrefix(r.RequestURI, "/debug/moa") {

		if len(strings.TrimPrefix(r.RequestURI, "/debug/moa")) <= 0 {
			if err := indexTmpl.Execute(w, profiles); err != nil {
				log.WarnLog("moa", "ServeHTTP|Execute|FAIL|%v|%s", err, r.RequestURI)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			return
		}

		//moa的状态信息
		if strings.HasPrefix(r.RequestURI, "/debug/moa/stat") {

			moaInfo := self.moaStat.GetMoaInfo()
			rawMoaInfo, _ := json.Marshal(moaInfo)
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/json")
			w.Write(rawMoaInfo)
			return

		} else if strings.HasPrefix(r.RequestURI, "/debug/moa/list/clients") {
			//列出所有的客户端
			clients := self.remoting.ListClients()
			rawClients, _ := json.Marshal(clients)
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/json")
			w.Write(rawClients)
			return
		} else if strings.HasPrefix(r.RequestURI, "/debug/moa/list/services") {
			//列出所有的services
			serviceNames := make([]string, 0, 1)
			for serviceName := range self.invokeHandler.instances {
				serviceNames = append(serviceNames, serviceName)
			}

			rawServices, _ := json.Marshal(serviceNames)
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/json")
			w.Write(rawServices)
			return
		} else if strings.HasPrefix(r.RequestURI, "/debug/moa/list/methods") {
			//列出所有的方法 /moa/list/methods?serviceName=user-profile
			serviceName := r.FormValue("service")
			if len(serviceName) > 0 {
				serviceInvoke := self.invokeHandler.ListInvokes(serviceName)
				//返回发布的方法
				rawMethods, _ := json.Marshal(serviceInvoke)

				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "text/json")
				w.Write(rawMethods)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "text/json")
				w.Write([]byte("{}"))
			}
			return
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else if strings.HasPrefix(r.RequestURI, "/metrics") {
		promhttp.Handler().ServeHTTP(w, r)
	} else {
		pprof.Index(w, r)
	}
}

var indexTmpl = template.Must(template.New("index").Parse(`<html>
<head>
<title>/debug/moa/</title>
<style>
.profile-name{
	display:inline-block;
	width:6rem;
}
</style>
</head>
<body>
/debug/moa/<br>
<br>
Types of moaprofiles available:
<table>
<thead><td>moa</td></thead>
{{range .}}
	<tr>
		<td><a href={{.Href}}>{{.Name}}</a></td>
	   <td><p>{{.Desc}}</p></td>
	</tr>
{{end}}
</table>
</body>
</html>
`))
