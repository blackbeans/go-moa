package core

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/blackbeans/turbo"
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

type Service struct {
	ServiceUri string //serviceUr对应的服务名称
	GroupId    string //该服务的分组
	Interface  interface{}
	Instance   interface{}
	//方法名称反射对应的方法
	methods map[string]MethodMeta

	InvokesPerClient *sync.Map //key: remoteip:port values:map[method]Count
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
		log.InfoLog("moa_server", "NewInvocationHandler|InitService|SUCC|%s", s.ServiceUri)
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

var typeOfContext = reflect.TypeOf(context.TODO())


//执行结果
func (self InvocationHandler) Invoke(ctx context.Context,req MoaRawReqPacket, onCallback func(resp MoaRespPacket) error) {
	self.moaStat.IncrRecv()
	now := time.Now()
	resp := MoaRespPacket{}

	//请求给到pool执行延迟
	if (now.UnixNano()/int64(time.Millisecond) - req.CreateTime) >= 500 {
		log.WarnLog("moa_server", "InvocationHandler|Invoke|Call|Delay|Source:%s|Cost[%d]ms|%s|%s",
			req.Source, (now.UnixNano()/int64(time.Millisecond) - req.CreateTime), req.ServiceUri, req.Params.Method)
	}

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

			//参数数量不对应
			if len(req.Params.Args) != len(m.ParamTypes) {
				self.moaStat.IncrError()
				resp.ErrCode = CODE_SERIALIZATION
				resp.Message = fmt.Sprintf(MSG_PARAMS_NOT_MATCHED,
					len(req.Params.Args), len(m.ParamTypes))
			} else {
				params := make([]reflect.Value, 0, len(m.ParamTypes))

				if len(m.ParamTypes) >0{
					//第一个参数类型判断下是否是context，如果是那么直接使用ctx
					if typeOfContext.AssignableTo(m.ParamTypes[0]) {
						params = append(params, reflect.ValueOf(ctx))
					}
				}

				//参数数量OK逐个转换为reflect.Value类型
				for i, arg := range req.Params.Args {
					f:= m.ParamTypes[i]
					inst := reflect.New(f)
					uerr := json.Unmarshal(arg, inst.Interface())
					if nil != uerr {
						resp.ErrCode = CODE_SERIALIZATION_SERVER
						resp.Message = fmt.Sprintf(MSG_SERIALIZATION, uerr)
					} else {
						params = append(params, inst.Elem())
					}
				}

				if resp.ErrCode != 0 && resp.ErrCode != CODE_SERVER_SUCC {
					self.moaStat.IncrError()
				} else {
					work := invoke(m, params...)
					if nil != work.err {
						log.ErrorLog("moa_server", "InvocationHandler|Invoke|Call|FAIL|%v|Source:%s|%s|%s|%s|%s",
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
				//超时了
				cost := time.Now().Sub(now)
				if cost/time.Millisecond >= 1000 {
					log.WarnLog("moa_server", "InvocationHandler|Invoke|Call|Slow|Source:%s|Cost[%d]ms|%s|%s|%v",
						req.Source, cost/time.Millisecond, req.ServiceUri, m.Name, params)
				}

				if cost >= req.Timeout {
					//丢弃结果
					log.WarnLog("moa_server", "InvocationHandler|Invoke|Call|Source:%s|Timeout[%d]ms|Cost:%d|%s|%s|%v",
						req.Source, req.Timeout/time.Millisecond, cost/time.Millisecond, req.ServiceUri, m.Name, params)
				} else {
					err := onCallback(resp)
					if nil != err {
						log.ErrorLog("moa_server", "InvocationHandler|Invoke|onCallback|%v|Source:%s|Timeout[%d]ms|%s|%s|%v", err,
							req.Source, req.Timeout/time.Millisecond, req.ServiceUri, m.Name, params)
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
