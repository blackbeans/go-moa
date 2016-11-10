package proxy

import (
	"encoding/json"
	"github.com/blackbeans/go-moa/log4moa"
	"github.com/blackbeans/go-moa/protocol"
	"github.com/blackbeans/turbo"
	"reflect"
	"testing"
	"time"

	_ "fmt"
)

type DemoResult struct {
	Name string
	Text string
}

type IHello interface {
	Hello(text string, param DemoParam) (DemoResult, error)
	HelloSlice(text string, arr []string, param DemoParam) (DemoResult, error)
	HelloComplexSlice(text string, arg2 map[string]DemoParam, arr []*DemoParam) (DemoResult, error)
}

type DemoParam struct {
	Name string
}

type Demo struct {
}

func (self Demo) Hello(text string, param DemoParam) (DemoResult, error) {
	// fmt.Println("----------Hello")
	return DemoResult{param.Name, text}, nil
}

func (self Demo) HelloSlice(text string, arr []string, param DemoParam) (DemoResult, error) {
	// fmt.Println("----------Hello")
	return DemoResult{param.Name, text}, nil
}

func (self Demo) HelloComplexSlice(text string, arg2 map[string]DemoParam, arr []*DemoParam) (DemoResult, error) {
	// fmt.Println("----------Hello")
	return DemoResult{"test", text}, nil
}

var stat = log4moa.NewMoaStat("hostname", "serviceUri",
	func(serviceUri, host string, moainfo log4moa.MoaInfo) {}, func() turbo.NetworkStat { return turbo.NetworkStat{} })

func TestInvocationHandler(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{
		ServiceUri: "demo",
		Instance:   Demo{}, Interface: (*IHello)(nil)}}, stat)
	m, ok := handler.instances["demo"].methods["hello"]
	if !ok {
		t.Fail()
	}
	t.Logf("TestInvocationHandler|Method Fields|%s", m.ParamTypes)
	for _, f := range m.ParamTypes {
		t.Logf("TestInvocationHandler|Hello|%s", f.Kind().String())
		if f.Kind() != reflect.String && f.Kind() != reflect.Struct {
			t.Fail()
		}
	}
}

func TestInvocationInvoke(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: (*IHello)(nil)}}, stat)
	req := &protocol.MoaReqPacket{}
	req.ServiceUri = "demo"
	req.Channel = make(chan interface{}, 10)
	req.Params.Args = []interface{}{"fuck", DemoParam{"you"}}
	req.Params.Method = "Hello"
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(protocol.MoaRequest2Raw(req))
	t.Logf("TestInvocationInvoke|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvocationInvoke|Invoke|Result|%s\n", string(data))
	}

}

func TestInvokeHelloSlice(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: (*IHello)(nil)}}, stat)
	req := &protocol.MoaReqPacket{}
	req.Channel = make(chan interface{}, 10)
	req.ServiceUri = "demo"
	req.Params.Args = []interface{}{"fuck", []string{"a", "b"}, DemoParam{"you"}}
	req.Params.Method = "HelloSlice"
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(protocol.MoaRequest2Raw(req))
	t.Logf("TestInvokeHelloSlice|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeHelloSlice|Invoke|Result|%s\n", string(data))
	}
}

func TestInvokeJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: (*IHello)(nil)}}, stat)

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"HelloSlice\",\"args\":[\"fuck\",[\"a\", \"b\"],{\"Name\":\"you\"}]}}"
	var req protocol.MoaRawReqPacket
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}
	t.Log(req)
	req.Channel = make(chan interface{}, 10)
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(&req)
	t.Logf("TestInvokeHelloSlice|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeHelloSlice|Invoke|Result|%s\n", string(data))
	}
}

func TestComplexSliceJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: (*IHello)(nil)}}, stat)

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"HelloComplexSlice\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	var req protocol.MoaRawReqPacket
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}

	t.Log(req)
	req.Channel = make(chan interface{}, 10)
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(&req)
	t.Logf("TestInvokeHelloSlice|Invoke|%s\n", resp)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeHelloSlice|Invoke|Result|%s\n", string(data))
	}
}
