package core

import (
	"encoding/json"
	"reflect"
	"time"
)

type MethodMeta struct {
	Name   string
	Method reflect.Method
	Fields []reflect.Type
}

type Service struct {
	ServiceUri string
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
		v := reflect.TypeOf(s.Instance)
		numMethod := v.NumMethod()
		s.methods = make(map[string]MethodMeta, numMethod)
		for i := 0; i < numMethod; i++ {
			mm := MethodMeta{}
			m := v.Method(i)
			mm.Method = m
			mm.Name = m.Name
			s.methods[m.Name] = mm
			fn := m.Type.NumIn()
			mm.Fields = make([]reflect.Type, 0, fn)
			for j := 0; j < fn; j++ {
				f := m.Type.In(j)
				mm.Fields = append(mm.Fields, f)

			}
		}
		instances[s.ServiceUri] = s
	}

	return &InvocationHandler{instances}

}

//moa请求协议的包
type MoaReqPacket struct {
	ServiceUri string        `json:"action"`
	Method     string        `json:"m"`
	Params     []interface{} `json:"args"`
}

//moa响应packet
type MoaRespPacket struct {
	Code   int         `json:"code"`
	Result interface{} `json:"result"`
}

//执行结果
func (self InvocationHandler) Invoke(packet MoaReqPacket) (MoaRespPacket, error) {
	resp := MoaRespPacket{}
	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[packet.ServiceUri]
	if !ok {

	} else {
		m, mok := instance.methods[packet.Method]
		if !mok {

		} else {
			//参数数量不对应
			if len(packet.Params) != len(m.Fields) {

			} else {
				params := make([]reflect.Value, 0, len(m.Fields))
				//参数数量OK逐个转换为reflect.Value类型
				for i, f := range m.Fields {
					arg := packet.Params[i]
					vl := reflect.ValueOf(arg)
					if vl.Type() == f {
						params = append(params, vl)
					} else if vl.Kind() == reflect.Map {
						//可能是对象类型则需要序列化为该对象
						data, err := json.Marshal(arg)
						if nil != err {

						} else {
							inst := reflect.New(f)
							uerr := json.Unmarshal(data, inst.Pointer())
							if nil != uerr {

							} else {
								params = append(params, inst)
							}
						}
					}
				}
				succ := make(chan bool, 1)
				go func() {
					results := m.Method.Func.Call(params)
					if len(results) > 1 {
						//返回值不能大于1个
					} else {
						resp.Code = 200
						resp.Result = results[0]
					}
					succ <- false
				}()

				select {
				case <-succ:
				case <-time.After(5 * time.Second):
					resp.Code = 520
				}
			}
		}
	}

	return resp, nil
}
