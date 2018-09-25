package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/blackbeans/turbo"
	log "github.com/blackbeans/log4go"
	"context"
	"github.com/blackbeans/pool"
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
	gopool pool.Pool
	tw        *turbo.TimerWheel
}

var errorType = reflect.TypeOf(make([]error, 1)).Elem()

func NewInvocationHandler(services []Service, moaStat *MoaStat,gopool pool.Pool) *InvocationHandler {

	tw := turbo.NewTimerWheel(10*time.Millisecond, 100)
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
	return &InvocationHandler{instances: instances,
		moaStat: moaStat,
		tw: tw,
		gopool: gopool}

}

//执行结果
func (self InvocationHandler) Invoke(packet *MoaRawReqPacket) *MoaRespPacket {
	self.moaStat.IncreaseRecv()
	resp := &MoaRespPacket{}
	//需要对包的内容解析进行反射调用
	instance, ok := self.instances[packet.ServiceUri]
	if !ok {
		self.moaStat.IncreaseError()
		resp.ErrCode = CODE_SERVICE_NOT_FOUND
		resp.Message = fmt.Sprintf(MSG_NO_URI_FOUND, packet.ServiceUri)
	} else {
		m, mok := instance.methods[strings.ToLower(packet.Params.Method)]
		if !mok {
			self.moaStat.IncreaseError()
			resp.ErrCode = CODE_METHOD_NOT_FOUND
			resp.Message = fmt.Sprintf(MSG_METHOD_NOT_FOUND, packet.Params.Method)
		} else {
			//参数数量不对应
			if len(packet.Params.Args) != len(m.ParamTypes) {
				self.moaStat.IncreaseError()
				resp.ErrCode = CODE_SERIALIZATION
				resp.Message = fmt.Sprintf(MSG_PARAMS_NOT_MATCHED,
					len(packet.Params.Args), len(m.ParamTypes))
			} else {
				params := make([]reflect.Value, 0, len(m.ParamTypes))
				//参数数量OK逐个转换为reflect.Value类型
				for i, f := range m.ParamTypes {
					arg := packet.Params.Args[i]
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
					return resp
				}

				ctx,cancel := context.WithTimeout(context.Background(),packet.Timeout)
				defer cancel()
				future := make(chan *interface{},1)

				work :=self.gopool.Queue(func(wu pool.WorkUnit) (interface{}, error) {
					ir := &invokeResult{}
					defer func() {
						future <- nil
					}()
					if wu.IsCancelled(){
						return ir,nil
					}
					ir.values = m.Method.Call(params)
					if len(m.ReturnType) <= 1 {
						if !ir.values[0].IsNil() {
							//其实就是个err
							ir.err = ir.values[0]
						}
					}
					return ir,nil
				})

				select {
				case  <-future:
					//等待结果
					work.Wait()
					if nil != work.Error() {
						log.ErrorLog("moa-server", "InvocationHandler|Invoke|Call|FAIL|%v|Source:%s|%s|%s|%s|%s",
							work.Error(), packet.Source, packet.ServiceUri, m.Name, params)
						self.moaStat.IncreaseError()
						resp.ErrCode = CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf(MSG_INVOCATION_TARGET, work.Error())
					}else if nil!=work.Value() {
						r := work.Value().(*invokeResult)
						if nil!=r.values {
							self.moaStat.IncreaseProc()
							resp.ErrCode = CODE_SERVER_SUCC
							resp.Result = r.values[0].Interface()
							//则肯定会有error
							if len(r.values) > 1 && !r.values[1].IsNil() {
								resp.Message = fmt.Sprintf("Method Invoke Error %v", r.values[1].Interface())
							}
						}else{
							//如果为空、说明是取消的任务
							self.moaStat.IncreaseError()
							resp.ErrCode = CODE_INVOCATION_TARGET
							resp.Message = fmt.Sprintf("NO Result ...")
						}
					} else{
						self.moaStat.IncreaseError()
						resp.ErrCode = CODE_INVOCATION_TARGET
						resp.Message = fmt.Sprintf("NO Result ...")
					}
				case <-ctx.Done():
					//cancel
					work.Cancel()

					self.moaStat.IncreaseTimeout()
					resp.ErrCode = CODE_TIMEOUT_SERVER
					resp.Message = fmt.Sprintf(MSG_TIMEOUT,
						packet.ServiceUri+"#"+packet.Params.Method)
					log.WarnLog("moa-server", "InvocationHandler|Invoke|Call|Source:%s|Timeout[%d]ms|%s|%s|%v",
						packet.Source, packet.Timeout/time.Millisecond, packet.ServiceUri, m.Name, params)
				}



			}
		}
	}

	return resp
}

type invokeResult struct {
	err    interface{}
	values []reflect.Value
}
