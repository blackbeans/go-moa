package core

import (
	"encoding/json"
	"fmt"
	"github.com/blackbeans/go-moa/proto"
	log "github.com/blackbeans/log4go"
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
	ServiceUri string //serviceUr对应的服务名称
	GroupId    string //该服务的分组
	Interface  interface{}
	Instance   interface{}
	//方法名称反射对应的方法
	methods map[string]MethodMeta
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
		}
		instances[s.ServiceUri] = s
		log.InfoLog("moa-server", "NewInvocationHandler|InitService|SUCC|%s", s.ServiceUri)
	}
	return &InvocationHandler{instances, moaStat}

}

//执行结果
func (self InvocationHandler) Invoke(packet *proto.MoaRawReqPacket) *proto.MoaRespPacket {
	self.moaStat.IncreaseRecv()
	resp := &proto.MoaRespPacket{}
	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[packet.ServiceUri]
	if !ok {
		self.moaStat.IncreaseError()
		resp.ErrCode = proto.CODE_SERVICE_NOT_FOUND
		resp.Message = fmt.Sprintf(proto.MSG_NO_URI_FOUND, packet.ServiceUri)
	} else {
		m, mok := instance.methods[strings.ToLower(packet.Params.Method)]
		if !mok {
			self.moaStat.IncreaseError()
			resp.ErrCode = proto.CODE_METHOD_NOT_FOUND
			resp.Message = fmt.Sprintf(proto.MSG_METHOD_NOT_FOUND, packet.Params.Method)
		} else {
			//参数数量不对应
			if len(packet.Params.Args) != len(m.ParamTypes) {
				self.moaStat.IncreaseError()
				resp.ErrCode = proto.CODE_SERIALIZATION
				resp.Message = fmt.Sprintf(proto.MSG_PARAMS_NOT_MATCHED,
					len(packet.Params.Args), len(m.ParamTypes))
			} else {
				params := make([]reflect.Value, 0, len(m.ParamTypes))
				//参数数量OK逐个转换为reflect.Value类型
				for i, f := range m.ParamTypes {
					arg := packet.Params.Args[i]
					inst := reflect.New(f)
					uerr := json.Unmarshal(arg, inst.Interface())
					if nil != uerr {
						resp.ErrCode = proto.CODE_SERIALIZATION_SERVER
						resp.Message = fmt.Sprintf(proto.MSG_SERIALIZATION, uerr)
					} else {
						params = append(params, inst.Elem())
					}
				}

				if resp.ErrCode != 0 && resp.ErrCode != proto.CODE_SERVER_SUCC {
					self.moaStat.IncreaseError()
					return resp
				}

				go func() {
					ir := &invokeResult{}
					defer func() {
						if err := recover(); nil != err {
							//TODO LOG ERROR
							log.ErrorLog("moa-server", "InvocationHandler|Invoke|Call|FAIL|%v|Source:%s|%s|%s|%s|%s",
								err, packet.Source, packet.ServiceUri, m.Name, params)
							resp.Message = fmt.Sprintf(proto.MSG_INVOCATION_TARGET, err)
							ir.err = err
							packet.Channel <- ir
						}
					}()
					ir.values = m.Method.Call(params)
					packet.Channel <- ir
				}()

				func() {
					select {
					case r := <-packet.Channel:
						result := r.(*invokeResult)
						values := result.values
						if nil != result.err {
							self.moaStat.IncreaseError()
							resp.ErrCode = proto.CODE_INVOCATION_TARGET
							resp.Message = fmt.Sprintf("Invoke FAIL (%v) ", result.err)
						} else if nil == values {
							self.moaStat.IncreaseError()
							resp.ErrCode = proto.CODE_INVOCATION_TARGET
							resp.Message = fmt.Sprintf("NO Result ...")
						} else {
							self.moaStat.IncreaseProc()
							resp.ErrCode = proto.CODE_SERVER_SUCC
							resp.Result = values[0].Interface()
							//则肯定会有error
							if len(values) > 1 && !values[1].IsNil() {
								resp.Message = fmt.Sprintf("Method Invoke Error %v", values[1].Interface())
							}

						}
					case <-time.After(packet.Timeout):
						self.moaStat.IncreaseTimeout()
						resp.ErrCode = proto.CODE_TIMEOUT_SERVER
						resp.Message = fmt.Sprintf(proto.MSG_TIMEOUT,
							packet.ServiceUri+"#"+packet.Params.Method)
						log.WarnLog("moa-server", "InvocationHandler|Invoke|Call|Source:%s|Timeout[%d]ms|%s|%s|%v",
							packet.Source, packet.Timeout/time.Millisecond, packet.ServiceUri, m.Name, params)
					}

				}()

			}
		}
	}

	return resp
}

type invokeResult struct {
	err    interface{}
	values []reflect.Value
}
