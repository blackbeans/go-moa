package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/blackbeans/turbo"
	"github.com/opentracing/opentracing-go"
	"reflect"
	"strings"
	"sync"
	"time"

	log "github.com/blackbeans/log4go"
)

type MethodMeta struct {
	Name       string
	Method     reflect.Value
	ReturnType []reflect.Type
	ParamTypes []reflect.Type
}

type ServiceMeta struct {
	ServiceUri   string `json:"service_uri"` //serviceUr对应的服务名称
	GroupId      string `json:"gid"`         //该服务的分组
	HostPort     string `json:"hostport"`    //节点
	ProtoVersion string `json:"proto_ver"`   //协议版本
	IsPre        bool   `json:"isPre"`       //是否是预发环境
}

type Service struct {
	ServiceUri string      `json:"service_uri"` //serviceUr对应的服务名称
	GroupId    string      `json:"gid"`         //该服务的分组
	IsPre      bool        `json:"isPre"`       //是否是预发环境
	Interface  interface{} `json:"-"`
	Instance   interface{} `json:"-"`
	//方法名称反射对应的方法
	methods map[string]MethodMeta

	InvokesPerClient *sync.Map `json:"-"` //key: remoteip:port values:map[method]Count
}

type InvocationHandler struct {
	instances map[string]Service
	moaStat   *MoaStat
}

var errorType = reflect.TypeOf(make([]error, 1)).Elem()

func NewInvocationHandler(services []Service, moaStat *MoaStat) *InvocationHandler {
	instances := make(map[string]Service, len(services))
	//对instace进行反射获得方法
	for _, s := range services {
		inter := reflect.TypeOf(s.Interface).Elem()
		rv := reflect.ValueOf(s.Instance)
		v := reflect.TypeOf(s.Instance)
		impl := v.Implements(inter)
		if !impl {
			panic(fmt.Sprintf("InvocationHandler|Not Implements|%s|%s",
				v.String(), inter.String()))
		}
		numMethod := inter.NumMethod()
		s.methods = make(map[string]MethodMeta, numMethod)
		for i := 0; i < numMethod; i++ {
			mm := MethodMeta{}
			m := inter.Method(i)
			im := rv.MethodByName(m.Name)
			mm.Method = im
			mm.Name = m.Name
			t := m.Type
			fn := t.NumIn()
			outType := make([]reflect.Type, 0, 2)
			for idx := 0; idx < t.NumOut(); idx++ {
				outType = append(outType, t.Out(idx))
			}
			//返回值必须大于等于1个并且小于2，并且其中一个必须为error类型
			if t.NumOut() >= 1 && t.NumOut() <= 2 {
				if !t.Out(t.NumOut() - 1).Implements(errorType) {
					panic(
						fmt.Errorf("%s Method  %s Last Return Type Must Be An Error! [%s]",
							s.ServiceUri, m.Name, t.Out(t.NumOut()-1).String()))
				}
			} else {
				panic(
					fmt.Errorf("%s Method  %s Last Return Count (1<=n<=2) Type "+
						"Must Be More Than An Error! ",
						s.ServiceUri, m.Name))
			}
			mm.ReturnType = outType
			mm.ParamTypes = make([]reflect.Type, 0, fn)
			for j := 0; j < fn; j++ {
				f := t.In(j)
				mm.ParamTypes = append(mm.ParamTypes, f)
			}
			s.methods[strings.ToLower(m.Name)] = mm
			//单个客户端调用的情况
			s.InvokesPerClient = &sync.Map{}
		}
		instances[s.ServiceUri] = s
		log.InfoLog("moa", "NewInvocationHandler|InitService|SUCC|%s", s.ServiceUri)
	}
	return &InvocationHandler{instances: instances,
		moaStat: moaStat}

}

//服务调用情况
func (self InvocationHandler) ListInvokes(servicename string) []InvokePerClient {

	service, ok := self.instances[servicename]
	if ok {
		clients := make([]InvokePerClient, 0, 10)

		service.InvokesPerClient.Range(func(key, value interface{}) bool {
			s := InvokePerClient{
				Client:      key.(string),
				ServiceName: service.ServiceUri,
				Methods:     make([]Method, 0, len(service.methods))}
			//clientip调用的方法及数量
			value.(*sync.Map).Range(func(key, value interface{}) bool {
				methodName := key.(string)
				count := value.(*turbo.Flow).Count()
				s.Methods = append(s.Methods, Method{Name: methodName, Count: int64(count)})
				return true
			})
			clients = append(clients, s)
			return true
		})

		return clients
	}
	return []InvokePerClient{}
}

var typeOfContext = reflect.TypeOf(new(context.Context)).Elem()

//执行结果
func (self InvocationHandler) Invoke(ctx context.Context, req MoaRawReqPacket, onCallback func(resp MoaRespPacket) error) {

	// tracing
	// 请求开始时的一些 set
	var (
		isTracing bool
		childSpan opentracing.Span
	)
	parentSpanCtx := GetSpanCtx(ctx) // 从当前的请求中获取 parent span
	if parentSpanCtx != nil {        // 有parent span时我们才开启child span，否则说明调用端没有开启 tracing
		isTracing = true
		childSpan = opentracing.GlobalTracer().StartSpan(req.Params.Method, opentracing.ChildOf(parentSpanCtx)) // 从parent span中生成 child span
		defer childSpan.Finish()                                                                                // Invoke结束时停止当前span
		ctx = WithSpanCtx(ctx, childSpan.Context())                                                             // 将 child span 写入 ctx

		// 将入参写到 span log 中
		for i, arg := range req.Params.Args {
			v, err := json.Marshal(arg)
			if err == nil {
				childSpan.LogKV(fmt.Sprintf("param.%d", i), string(v))
			}
		}
		// 将 moaCtx 有关信息写到 span tag 中
		if props := ctx.Value(KEY_MOA_PROPERTIES); props != nil {
			// 从 moa.props 中获取 key value 设置到 child span 的 tag
			for k, v := range props.(map[string]string) {
				childSpan.SetTag("moa."+k, v)
			}
		}
	}

	self.moaStat.IncrRecv()
	now := time.Now()
	resp := MoaRespPacket{}

	//捕获运行时异常
	defer func() {
		if crash := recover(); nil != crash {
			self.moaStat.IncrError()
			resp.ErrCode = CODE_INVOCATION_TARGET
			resp.Message = fmt.Sprintf(MSG_INVOCATION_TARGET, fmt.Sprintf("%v", crash))
			log.ErrorLog("moa", "InvocationHandler|Invoke|Panic|%v|Source:%s|Timeout[%d]ms|%s|%s|%v", crash,
				req.Source, req.Timeout/time.Millisecond, req.ServiceUri, req.ServiceUri, req.Source, req.Params.Method)
		}

		// 记录耗时
		cost := time.Now().Sub(now)
		self.moaStat.MoaMetrics.RpcInvokeDurationSummary.WithLabelValues(req.Params.Method).Observe(cost.Seconds())
		// 长耗时
		if cost/time.Millisecond >= 1000 {
			log.WarnLog("moa", "InvocationHandler|Invoke|Call|Slow|Source:%s|Cost[%d]ms|%s|%s|%v",
				req.Source, cost/time.Millisecond, req.ServiceUri, req.Source, req.Params.Method)
		}
		// 超时了
		if cost >= req.Timeout {
			//丢弃结果
			log.WarnLog("moa", "InvocationHandler|Invoke|Call|Source:%s|Timeout[%d]ms|Cost:%d|%s|%s|%v",
				req.Source, req.Timeout/time.Millisecond, cost/time.Millisecond, req.ServiceUri, req.Source, req.Params.Method)
		} else {
			// tracing
			// 请求响应时的一些 set
			if isTracing {
				childSpan.SetTag("resp.ec", resp.ErrCode)
				childSpan.SetTag("resp.em", resp.Message)
				rawJson, err := json.Marshal(resp.Result)
				if err == nil {
					childSpan.LogKV("resp.result", string(rawJson))
				}
				if resp.ErrCode != CODE_SERVER_SUCC {
					childSpan.SetTag("error", true)
				}
			}

			// 根据errCode设置error
			err := onCallback(resp)
			if nil != err {
				log.ErrorLog("moa", "InvocationHandler|Invoke|onCallback|%v|Source:%s|Timeout[%d]ms|%s|%s|%v", err,
					req.Source, req.Timeout/time.Millisecond, req.ServiceUri, req.ServiceUri, req.Source, req.Params.Method)
			}
		}
	}()

	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[req.ServiceUri]
	if !ok {
		self.moaStat.IncrError()
		resp.ErrCode = CODE_SERVICE_NOT_FOUND
		resp.Message = fmt.Sprintf(MSG_NO_URI_FOUND, req.ServiceUri)
	} else {
		m, mok := instance.methods[strings.ToLower(req.Params.Method)]
		if !mok {
			self.moaStat.IncrError()
			resp.ErrCode = CODE_METHOD_NOT_FOUND
			resp.Message = fmt.Sprintf(MSG_METHOD_NOT_FOUND, req.Params.Method)
		} else {

			countPerMethod, ok := instance.InvokesPerClient.Load(req.Source)
			if !ok {
				tmp := &sync.Map{}
				exist, ok := instance.InvokesPerClient.LoadOrStore(req.Source, tmp)
				if !ok {
					//么有load到则是缓存放入的
					countPerMethod = tmp
				} else {
					countPerMethod = exist
				}
			}
			flow := countPerMethod.(*sync.Map)
			counter, ok := flow.Load(m.Name)
			if !ok {
				tmp := &turbo.Flow{}
				exist, ok := flow.LoadOrStore(m.Name, tmp)
				if !ok {
					counter = tmp
				} else {
					counter = exist
				}
			}

			counter.(*turbo.Flow).Incr(1)

			paramTypes := m.ParamTypes
			params := make([]reflect.Value, 0, len(m.ParamTypes))
			if len(m.ParamTypes) > 0 {
				//第一个参数类型判断下是否是context，如果是那么直接使用ctx
				if m.ParamTypes[0].Implements(typeOfContext) {
					params = append(params, reflect.ValueOf(ctx))
					paramTypes = paramTypes[1:]
				}
			}

			//参数数量不对应
			if len(req.Params.Args) != len(paramTypes) {
				self.moaStat.IncrError()
				resp.ErrCode = CODE_SERIALIZATION
				resp.Message = fmt.Sprintf(MSG_PARAMS_NOT_MATCHED,
					len(req.Params.Args), len(m.ParamTypes))
			} else {
				//参数数量OK逐个转换为reflect.Value类型
				for i, arg := range req.Params.Args {
					f := paramTypes[i]
					inst := reflect.New(f)
					uerr := json.Unmarshal(arg, inst.Interface())
					if nil != uerr {
						resp.ErrCode = CODE_SERIALIZATION_SERVER
						resp.Message = fmt.Sprintf(MSG_SERIALIZATION, uerr)
						log.ErrorLog("moa", "InvocationHandler|Invoke|Unmarshal|Source:%s|%s|%s|%s|%v",
							req.Source, req.ServiceUri, m.Name, string(arg), uerr)
						break
					} else {
						params = append(params, inst.Elem())
					}
				}

				if resp.ErrCode != 0 && resp.ErrCode != CODE_SERVER_SUCC {
					self.moaStat.IncrError()
				} else {
					work := invoke(m, params...)
					if nil != work.err {
						log.ErrorLog("moa", "InvocationHandler|Invoke|Call|FAIL|%v|Source:%s|%s|%s|%s|%s",
							work.err, req.Source, req.ServiceUri, m.Name, params)
						self.moaStat.IncrError()
						resp.ErrCode = CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf(MSG_INVOCATION_TARGET, work.err)
					} else if r := work.values; nil != r {
						self.moaStat.IncrProc()
						resp.ErrCode = CODE_SERVER_SUCC
						resp.Result = r[0].Interface()
						//则肯定会有error
						if len(r) > 1 && !r[1].IsNil() {
							resp.Message = fmt.Sprintf("Method Invoke Error %v", r[1].Interface())
						}
					} else {
						//如果为空、说明是取消的任务
						self.moaStat.IncrError()
						resp.ErrCode = CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf("NO Result ...")
					}
				}
			}
		}
	}
}

func invoke(m MethodMeta, params ...reflect.Value) invokeResult {
	ir := invokeResult{}
	ir.values = m.Method.Call(params)
	if len(m.ReturnType) <= 1 {
		if !ir.values[0].IsNil() {
			//其实就是个err
			ir.err = ir.values[0]
		}
	}

	return ir
}

type invokeResult struct {
	err    interface{}
	values []reflect.Value
}
