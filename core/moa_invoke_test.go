package core

import (
	"reflect"
	"testing"
)

type Demo struct {
}

func (self Demo) Hello(text string) string {
	return ""
}

func TestInvocationHandler(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo", Instance: Demo{}}})
	m, ok := handler.instances["demo"].methods["Hello"]
	if !ok {
		t.Fail()
	}

	for _, f := range m.Fields {
		if f.Kind() != reflect.String {
			t.Fail()
		}
	}
}
