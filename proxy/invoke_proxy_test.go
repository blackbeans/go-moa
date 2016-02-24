package proxy

import (
	"encoding/json"
	"git.wemomo.com/bibi/go-moa/protocol"
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
	Hello(text string, param DemoParam) DemoResult
	HelloSlice(text string, arr []string, param DemoParam) DemoResult
	HelloComplexSlice(text string, arg2 map[string]DemoParam, arr []*DemoParam) DemoResult
}

type DemoParam struct {
	Name string
}

type Demo struct {
}

func (self Demo) Hello(text string, param DemoParam) DemoResult {
	// fmt.Println("----------Hello")
	return DemoResult{param.Name, text}
}

func (self Demo) HelloSlice(text string, arr []string, param DemoParam) DemoResult {
	// fmt.Println("----------Hello")
	return DemoResult{param.Name, text}
}

func (self Demo) HelloComplexSlice(text string, arg2 map[string]DemoParam, arr []*DemoParam) DemoResult {
	// fmt.Println("----------Hello")
	return DemoResult{"test", text}
}

func TestInvocationHandler(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: reflect.TypeOf((*IHello)(nil)).Elem()}})
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
		Instance: Demo{}, Interface: reflect.TypeOf((*IHello)(nil)).Elem()}})
	req := protocol.MoaReqPacket{}
	req.ServiceUri = "demo"
	req.Params = []interface{}{"fuck", DemoParam{"you"}}
	req.Method = "Hello"
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(req)
	t.Logf("TestInvocationInvoke|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvocationInvoke|Invoke|Result|%s\n", string(data))
	}

}

func TestInvokeHelloSlice(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: reflect.TypeOf((*IHello)(nil)).Elem()}})
	req := protocol.MoaReqPacket{}
	req.ServiceUri = "demo"
	req.Params = []interface{}{"fuck", []string{"a", "b"}, DemoParam{"you"}}
	req.Method = "HelloSlice"
	req.Timeout = 5 * time.Second
	resp := handler.Invoke(req)
	t.Logf("TestInvokeHelloSlice|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeHelloSlice|Invoke|Result|%s\n", string(data))
	}
}

func TestInvokeJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: reflect.TypeOf((*IHello)(nil)).Elem()}})

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"HelloSlice\",\"args\":[\"fuck\",[\"a\", \"b\"],{\"Name\":\"you\"}]}}"
	var req protocol.CommandRequest
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}
	t.Log(req)
	moaReq := protocol.Command2MoaRequest(req)
	moaReq.Timeout = 5 * time.Second
	resp := handler.Invoke(moaReq)
	t.Logf("TestInvokeHelloSlice|Invoke|%s\n", resp.Result)
	if resp.ErrCode != 200 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeHelloSlice|Invoke|Result|%s\n", string(data))
	}
}

func TestComplexSliceJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: Demo{}, Interface: reflect.TypeOf((*IHello)(nil)).Elem()}})

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"HelloComplexSlice\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	var req protocol.CommandRequest
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}
	t.Log(req)
	moaReq := protocol.Command2MoaRequest(req)
	moaReq.Timeout = 5 * time.Second
	resp := handler.Invoke(moaReq)
	t.Logf("TestInvokeHelloSlice|Invoke|%s\n", resp)
	if resp.ErrCode != 200 {
		t.Fail()
	} else {
		data, _ := json.Marshal(resp.Result)
		t.Logf("TestInvokeHelloSlice|Invoke|Result|%s\n", string(data))
	}
}
