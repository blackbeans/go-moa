package core

import (
	"encoding/json"
	"github.com/blackbeans/go-moa/proto"
	"github.com/blackbeans/turbo"
	"reflect"
	"testing"
	"time"

	_ "fmt"
)

type ProxyParam struct {
	Name string
}

type ProxyResult struct {
	Name string
	Text string
}

type IProxyDemo interface {
	ProxyDemo(text string, param ProxyParam) (ProxyResult, error)
	ProxyDemoSlice(text string, arr []string, param ProxyParam) (ProxyResult, error)
	ProxyDemoComplexSlice(text string, arg2 map[string]ProxyParam, arr []*ProxyParam) (ProxyResult, error)
}

type DemoProxy struct {
}

func (self DemoProxy) ProxyDemo(text string, param ProxyParam) (ProxyResult, error) {
	// fmt.Println("----------ProxyDemo")
	return ProxyResult{param.Name, text}, nil
}

func (self DemoProxy) ProxyDemoSlice(text string, arr []string, param ProxyParam) (ProxyResult, error) {
	// fmt.Println("----------ProxyDemo")
	return ProxyResult{param.Name, text}, nil
}

func (self DemoProxy) ProxyDemoComplexSlice(text string, arg2 map[string]ProxyParam, arr []*ProxyParam) (ProxyResult, error) {
	// fmt.Println("----------ProxyDemo")
	return ProxyResult{"test", text}, nil
}

var stat = NewMoaStat("hostname", "serviceUri",
	func(serviceUri, host string, moainfo MoaInfo) {}, func() turbo.NetworkStat { return turbo.NetworkStat{} })

func TestInvocationHandler(t *testing.T) {
	handler := NewInvocationHandler([]Service{
		Service{
			ServiceUri: "demo",
			Instance:   DemoProxy{},
			Interface:  (*IProxyDemo)(nil)}}, stat)

	m, ok := handler.instances["demo"].methods["proxydemo"]
	if !ok {
		t.Fail()
	}
	t.Logf("TestInvocationHandler|Method Fields|%s", m.ParamTypes)
	for _, f := range m.ParamTypes {
		t.Logf("TestInvocationHandler|ProxyDemo|%s", f.Kind().String())
		if f.Kind() != reflect.String && f.Kind() != reflect.Struct {
			t.Fail()
		}
	}
}

func TestInvocationInvoke(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: DemoProxy{}, Interface: (*IProxyDemo)(nil)}}, stat)
	req := &proto.MoaReqPacket{}
	req.ServiceUri = "demo"
	req.Channel = make(chan interface{}, 10)
	req.Params.Args = []interface{}{"fuck", DemoParam{"you"}}
	req.Params.Method = "proxydemo"
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(proto.MoaRequest2Raw(req))
	t.Logf("TestInvocationInvoke|Invoke|%s\n", resp)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvocationInvoke|Invoke|Result|%s\n", string(data))
	}

}

func TestInvokeProxyDemoSlice(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: DemoProxy{}, Interface: (*IProxyDemo)(nil)}}, stat)
	req := &proto.MoaReqPacket{}
	req.Channel = make(chan interface{}, 10)
	req.ServiceUri = "demo"
	req.Params.Args = []interface{}{"fuck", []string{"a", "b"}, ProxyParam{"you"}}
	req.Params.Method = "ProxyDemoSlice"
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(proto.MoaRequest2Raw(req))
	t.Logf("TestInvokeProxyDemoSlice|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeProxyDemoSlice|Invoke|Result|%s\n", string(data))
	}
}

func TestInvokeJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: DemoProxy{}, Interface: (*IProxyDemo)(nil)}}, stat)

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"ProxyDemoSlice\",\"args\":[\"fuck\",[\"a\", \"b\"],{\"Name\":\"you\"}]}}"
	var req proto.MoaRawReqPacket
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}
	t.Log(req)
	req.Channel = make(chan interface{}, 10)
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(&req)
	t.Logf("TestInvokeProxyDemoSlice|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeProxyDemoSlice|Invoke|Result|%s\n", string(data))
	}
}

func TestComplexSliceJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{
		Service{
			ServiceUri: "demo",
			Instance:   DemoProxy{},
			Interface:  (*IProxyDemo)(nil)}},
		stat)

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"ProxyDemoComplexSlice\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	var req proto.MoaRawReqPacket
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}

	t.Log(req)
	req.Channel = make(chan interface{}, 10)
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(&req)
	t.Logf("TestInvokeProxyDemoSlice|Invoke|%s\n", resp)
	if resp.ErrCode != 200 && resp.ErrCode != 0 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeProxyDemoSlice|Invoke|Result|%s\n", string(data))
	}
}
