package proxy

import (
	"encoding/json"
	"fmt"
	"github.com/blackbeans/go-moa/log4moa"
	"github.com/blackbeans/go-moa/protocol"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/go-errors/errors"
	"reflect"
	"strings"
	"time"
)

type MethodMeta struct {
	Name       string
	Method     reflect.Value
	ReturnType []reflect.Type
	ParamTypes []reflect.Type
}

type Service struct {
	ServiceUri string
	Interface  interface{}
	Instance   interface{}
	//方法名称反射对应的方法
	methods map[string]MethodMeta
}

type InvocationHandler struct {
	instances map[string]Service
	moaStat   *log4moa.MoaStat
	tw        *turbo.TimeWheel
}

var errorType = reflect.TypeOf(make([]error, 1)).Elem()

func NewInvocationHandler(services []Service, moaStat *log4moa.MoaStat) *InvocationHandler {

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
					panic(errors.New(
						fmt.Sprintf("%s Method  %s Last Return Type Must Be An Error! [%s]",
							s.ServiceUri, m.Name, t.Out(t.NumOut()-1).String())))
				}
			} else {
				panic(errors.New(
					fmt.Sprintf("%s Method  %s Last Return Count (1<=n<=2) Type "+
						"Must Be More Than An Error! ",
						s.ServiceUri, m.Name)))
			}
			mm.ReturnType = outType
			mm.ParamTypes = make([]reflect.Type, 0, fn)
			for j := 0; j < fn; j++ {
				f := t.In(j)
				mm.ParamTypes = append(mm.ParamTypes, f)

			}
			s.methods[strings.ToLower(m.Name)] = mm
		}
		instances[s.ServiceUri] = s
		log.InfoLog("moa-server", "NewInvocationHandler|InitService|SUCC|%s", s.ServiceUri)
	}
	tw := turbo.NewTimeWheel(1*time.Second, 10, 10)
	return &InvocationHandler{instances, moaStat, tw}

}

//执行结果
func (self InvocationHandler) Invoke(packet protocol.MoaReqPacket) protocol.MoaRespPacket {
	self.moaStat.IncreaseRecv()
	resp := protocol.MoaRespPacket{}
	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[packet.ServiceUri]
	if !ok {
		self.moaStat.IncreaseError()
		resp.ErrCode = protocol.CODE_SERVICE_NOT_FOUND
		resp.Message = fmt.Sprintf(protocol.MSG_NO_URI_FOUND, packet.ServiceUri)
	} else {
		m, mok := instance.methods[strings.ToLower(packet.Method)]
		if !mok {
			self.moaStat.IncreaseError()
			resp.ErrCode = protocol.CODE_METHOD_NOT_FOUND
			resp.Message = fmt.Sprintf(protocol.MSG_METHOD_NOT_FOUND, packet.Method)
		} else {
			//参数数量不对应
			if len(packet.Params) != len(m.ParamTypes) {
				self.moaStat.IncreaseError()
				resp.ErrCode = protocol.CODE_SERIALIZATION
				resp.Message = fmt.Sprintf(protocol.MSG_PARAMS_NOT_MATCHED, len(packet.Params), len(m.ParamTypes))
			} else {
				params := make([]reflect.Value, 0, len(m.ParamTypes))
				//参数数量OK逐个转换为reflect.Value类型
				for i, f := range m.ParamTypes {
					arg := packet.Params[i]
					vl := reflect.ValueOf(arg)
					//类型相等应该就是原则类型了吧、不是数组并且两个类型一样
					if vl.Type() == f {
						params = append(params, vl)
					} else if vl.Kind() == reflect.Map ||
						vl.Kind() == reflect.Slice {
						//可能是对象类型则需要序列化为该对象
						data, err := json.Marshal(arg)
						if nil != err {
							resp.ErrCode = protocol.CODE_SERIALIZATION_SERVER
							resp.Message = fmt.Sprintf(protocol.MSG_SERIALIZATION, err)
						} else {
							inst := reflect.New(f)
							uerr := json.Unmarshal(data, inst.Interface())
							if nil != uerr {
								resp.ErrCode = protocol.CODE_SERIALIZATION_SERVER
								resp.Message = fmt.Sprintf(protocol.MSG_SERIALIZATION, uerr)
							} else {
								params = append(params, inst.Elem())
							}
						}
					} else {
						if vl.Type().Kind() == reflect.Float32 || vl.Type().Kind() == reflect.Float64 {
							var val float64
							v, ok := arg.(float32)
							if !ok {
								val = arg.(float64)
							} else {
								val = float64(v)
							}

							switch f.Kind() {
							case reflect.Int8:
								params = append(params, reflect.ValueOf(int8(val)))
							case reflect.Int:
								params = append(params, reflect.ValueOf(int(val)))
							case reflect.Int16:
								params = append(params, reflect.ValueOf(int16(val)))
							case reflect.Int32:
								params = append(params, reflect.ValueOf(int32(val)))
							case reflect.Int64:
								params = append(params, reflect.ValueOf(int64(val)))
							case reflect.Uint8:
								params = append(params, reflect.ValueOf(uint8(val)))
							case reflect.Uint:
								params = append(params, reflect.ValueOf(uint(val)))
							case reflect.Uint16:
								params = append(params, reflect.ValueOf(uint16(val)))
							case reflect.Uint32:
								params = append(params, reflect.ValueOf(uint32(val)))
							case reflect.Uint64:
								params = append(params, reflect.ValueOf(uint64(val)))
							default:
								resp.ErrCode = protocol.CODE_SERIALIZATION_SERVER
								resp.Message = fmt.Sprintf(protocol.MSG_SERIALIZATION, "Unsupport ParamType "+vl.Kind().String())
							}
						} else {
							resp.ErrCode = protocol.CODE_SERIALIZATION_SERVER
							resp.Message = fmt.Sprintf(protocol.MSG_SERIALIZATION, "Unsupport ParamType "+vl.Kind().String())
						}
					}
				}

				if resp.ErrCode != 0 && resp.ErrCode != protocol.CODE_SERVER_SUCC {
					self.moaStat.IncreaseError()
					return resp
				}
				// errChan := make(chan error, 1)
				ch := make(chan *invokeResult, 1)
				go func() {
					ir := &invokeResult{}
					defer func() {
						if err := recover(); nil != err {
							var e error
							er, ok := err.(*errors.Error)
							if ok {
								stack := er.ErrorStack()
								e = errors.New(stack)
							} else {
								e = errors.New(fmt.Sprintf("Method Call Err %s", err))
							}

							//TODO LOG ERROR
							log.ErrorLog("moa-server", "InvocationHandler|Invoke|Call|FAIL|%s|%s|%s|%s",
								e, packet.ServiceUri, m.Name, params)
							resp.Message = fmt.Sprintf(protocol.MSG_INVOCATION_TARGET, err)
							ir.err = er
							ch <- ir
						}
					}()
					ir.values = m.Method.Call(params)
					ch <- ir
				}()

				timerId, timeout := self.tw.After(packet.Timeout, func() {})
				select {
				case result := <-ch:
					values := result.values
					if nil != result.err {
						self.moaStat.IncreaseError()
						resp.ErrCode = protocol.CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf("Invoke FAIL %s", result.err)
					} else if nil == values {
						self.moaStat.IncreaseError()
						resp.ErrCode = protocol.CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf("NO Result ...")
					} else {
						self.moaStat.IncreaseProc()
						resp.ErrCode = protocol.CODE_SERVER_SUCC
						resp.Result = values[0].Interface()
						//则肯定会有error
						if len(values) > 1 {
							resp.Message = fmt.Sprintf("%v", values[1].Interface())
						}

					}
				case <-timeout:
					self.moaStat.IncreaseError()
					resp.ErrCode = protocol.CODE_TIMEOUT_SERVER
					resp.Message = fmt.Sprintf(protocol.MSG_TIMEOUT, packet.ServiceUri+"#"+packet.Method)
				}
				self.tw.Remove(timerId)
			}
		}
	}

	return resp
}

type invokeResult struct {
	err    interface{}
	values []reflect.Value
}
