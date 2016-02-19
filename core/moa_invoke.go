package core

import (
	"reflect"
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
