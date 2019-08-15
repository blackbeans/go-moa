package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
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
							s.ServiceUri, m.Name, t.Out(t.NumOut() - 1).String()))
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
	return &InvocationHandler{instances: instances,
		moaStat: moaStat}

}

//执行结果
func (self InvocationHandler) Invoke(req MoaRawReqPacket,tctx *turbo.TContext){
	self.moaStat.IncreaseRecv()
	now := time.Now()
	resp := MoaRespPacket{}
	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[req.ServiceUri]
	if !ok {
		self.moaStat.IncreaseError()
		resp.ErrCode = CODE_SERVICE_NOT_FOUND
		resp.Message = fmt.Sprintf(MSG_NO_URI_FOUND, req.ServiceUri)
	} else {
		m, mok := instance.methods[strings.ToLower(req.Params.Method)]
		if !mok {
			self.moaStat.IncreaseError()
			resp.ErrCode = CODE_METHOD_NOT_FOUND
			resp.Message = fmt.Sprintf(MSG_METHOD_NOT_FOUND, req.Params.Method)
		} else {
			//参数数量不对应
			if len(req.Params.Args) != len(m.ParamTypes) {
				self.moaStat.IncreaseError()
				resp.ErrCode = CODE_SERIALIZATION
				resp.Message = fmt.Sprintf(MSG_PARAMS_NOT_MATCHED,
					len(req.Params.Args), len(m.ParamTypes))
			} else {
				params := make([]reflect.Value, 0, len(m.ParamTypes))
				//参数数量OK逐个转换为reflect.Value类型
				for i, f := range m.ParamTypes {
					arg := req.Params.Args[i]
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
					self.moaStat.IncreaseError()
				} else {
					work := self.invoke(m, params...)
					if nil != work.err {
						log.ErrorLog("moa-server", "InvocationHandler|Invoke|Call|FAIL|%v|Source:%s|%s|%s|%s|%s",
							work.err, req.Source, req.ServiceUri, m.Name, params)
						self.moaStat.IncreaseError()
						resp.ErrCode = CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf(MSG_INVOCATION_TARGET, work.err)
					} else if r := work.values; nil != r {
						self.moaStat.IncreaseProc()
						resp.ErrCode = CODE_SERVER_SUCC
						resp.Result = r[0].Interface()
						//则肯定会有error
						if len(r) > 1 && !r[1].IsNil() {
							resp.Message = fmt.Sprintf("Method Invoke Error %v", r[1].Interface())
						}
					} else {
						//如果为空、说明是取消的任务
						self.moaStat.IncreaseError()
						resp.ErrCode = CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf("NO Result ...")
					}
				}

				//超时了
				if time.Now().Sub(now) >=  req.Timeout{
					//丢弃结果
					log.WarnLog("moa-server", "InvocationHandler|Invoke|Call|Source:%s|Timeout[%d]ms|%s|%s|%v",
						req.Source, req.Timeout/time.Millisecond, req.ServiceUri, m.Name, params)
				}else{
					//正常
					respPacker := turbo.NewRespPacket(tctx.Message.Header.Opaque, RESP, nil)
					respPacker.PayLoad = resp
					if  (resp.ErrCode != 0 && resp.ErrCode != CODE_SERVER_SUCC) {
						//需要发送调用的错误给客户端
						log.ErrorLog("moa-server", "Application|Invoke|FAIL|%v", tctx.Message)
						tctx.Client.Write(*respPacker)
						return
					}
					tctx.Client.Write(*respPacker)
				}
			}
		}
	}
}

func (self *InvocationHandler) invoke(m MethodMeta, params ...reflect.Value) invokeResult {
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
