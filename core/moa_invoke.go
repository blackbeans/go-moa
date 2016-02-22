package core

import (
	"encoding/json"
	"fmt"
	log "github.com/blackbeans/log4go"
	"go-moa/protocol"
	"reflect"
	"time"
)

type MethodMeta struct {
	Name       string
	Method     reflect.Value
	ReturnType reflect.Type
	ParamTypes []reflect.Type
}

type Service struct {
	ServiceUri string
	Interface  reflect.Type
	Instance   interface{}
	//方法名称反射对应的方法
	methods map[string]MethodMeta
}

type InvocationHandler struct {
	instances map[string]Service
}

func NewInvocationHandler(services []Service) *InvocationHandler {

	instances := make(map[string]Service, len(services))
	//对instace进行反射获得方法
	for _, s := range services {
		rv := reflect.ValueOf(s.Instance)
		v := reflect.TypeOf(s.Instance)
		impl := v.Implements(s.Interface)
		if !impl {
			panic(fmt.Sprintf("InvocationHandler|Not Implements|%s|%s",
				v.String(), s.Interface.String()))
		}
		numMethod := s.Interface.NumMethod()
		s.methods = make(map[string]MethodMeta, numMethod)
		for i := 0; i < numMethod; i++ {
			mm := MethodMeta{}
			m := s.Interface.Method(i)
			im := rv.MethodByName(m.Name)
			mm.Method = im
			mm.Name = m.Name
			fn := m.Type.NumIn()
			//如果方法返回值个数大于1则不允许
			if m.Type.NumOut() > 1 {
				panic(fmt.Sprintf("InvocationHandler|Return Args Num %d>%d",
					m.Type.NumOut(), 1))
			} else {
				if m.Type.NumOut() == 1 {
					mm.ReturnType = m.Type.Out(0)
				}
			}
			mm.ParamTypes = make([]reflect.Type, 0, fn)
			for j := 0; j < fn; j++ {
				f := m.Type.In(j)
				mm.ParamTypes = append(mm.ParamTypes, f)

			}

			s.methods[m.Name] = mm
		}
		instances[s.ServiceUri] = s
	}

	return &InvocationHandler{instances}

}

//执行结果
func (self InvocationHandler) Invoke(packet protocol.MoaReqPacket) protocol.MoaRespPacket {
	resp := protocol.MoaRespPacket{}
	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[packet.ServiceUri]
	if !ok {
		resp.ErrCode = protocol.CODE_SERVICE_NOT_FOUND
		resp.Message = fmt.Sprintf(protocol.MSG_NO_URI_FOUND, packet.ServiceUri)
	} else {
		m, mok := instance.methods[packet.Method]
		if !mok {
			resp.ErrCode = protocol.CODE_METHOD_NOT_FOUND
			resp.Message = fmt.Sprintf(protocol.MSG_METHOD_NOT_FOUND, packet.Method)
		} else {
			//参数数量不对应
			if len(packet.Params) != len(m.ParamTypes) {
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
						resp.ErrCode = protocol.CODE_SERIALIZATION_SERVER
						resp.Message = fmt.Sprintf(protocol.MSG_SERIALIZATION, "Unsupport ParamType "+vl.Kind().String())
					}
				}

				if resp.ErrCode != 0 && resp.ErrCode != protocol.CODE_SERVER_SUCC {
					return resp
				}

				r := make(chan []reflect.Value, 1)
				go func() {
					defer func() {
						if err := recover(); nil != err {
							//TODO LOG ERROR
							log.ErrorLog("moa_handler", "InvocationHandler|Invoke|Call|FAIL|%s|%s|%s",
								err, m.Method, params)
							resp.Message = fmt.Sprintf(protocol.MSG_INVOCATION_TARGET, err)
							r <- nil
						}
					}()
					r <- m.Method.Call(params)
				}()

				select {
				case result := <-r:
					if nil == result {
						resp.ErrCode = protocol.CODE_INVOCATION_TARGET
					} else {
						resp.ErrCode = protocol.CODE_SERVER_SUCC
						resp.Result = result[0].Interface()
					}

				case <-time.After(packet.Timeout):
					resp.ErrCode = protocol.CODE_TIMEOUT_SERVER
				}
			}
		}
	}

	return resp
}
