package core

import (
	"reflect"
)

type MethodMeta struct {
	Name   string
	Method reflect.Value
	Fields []reflect.Kind
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
		v := reflect.ValueOf(s.Instance)
		numMethod := v.NumMethod()
		s.methods = make(map[string]MethodMeta, numMethod)
		for i := 0; i < numMethod; i++ {
			mm := MethodMeta{}
			m := v.Method(i)
			mm.Method = m
			methodName := m.String()
			mm.Name = methodName
			s.methods[methodName] = mm
			fn := m.NumField()
			mm.Fields = make([]reflect.Kind, 0, fn)
			for j := 0; j < fn; j++ {
				f := m.Field(j)
				mm.Fields = append(mm.Fields, f.Kind())
			}
		}
		instances[s.ServiceUri] = s
	}

	return &InvocationHandler{instances}

}
