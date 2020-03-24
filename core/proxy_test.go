package core

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/blackbeans/turbo"

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
	ProxyDemo(ctx context.Context,text string, param ProxyParam) (ProxyResult, error)
	ProxyDemoSlice(text string, arr []string, param ProxyParam) (ProxyResult, error)
	ProxyDemoComplexSlice(text string, arg2 map[string]ProxyParam, arr []*ProxyParam) (ProxyResult, error)
}

type DemoProxy struct {
}

func (self DemoProxy) ProxyDemo(ctx context.Context,text string, param ProxyParam) (ProxyResult, error) {
	fmt.Println("----------ProxyDemo")
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

func MoaRequest2Raw(req *MoaReqPacket) *MoaRawReqPacket {
	raw := &MoaRawReqPacket{}
	raw.ServiceUri = req.ServiceUri

	raw.Params.Method = req.Params.Method
	rawArgs := make([]json.RawMessage, 0, len(req.Params.Args))
	for _, a := range req.Params.Args {
		rw, _ := json.Marshal(a)
		rawArgs = append(rawArgs, json.RawMessage(rw))
	}
	raw.Params.Args = rawArgs
	raw.Timeout = req.Timeout
	return raw
}

var stat = NewMoaStat("hostname",
	"serviceUri",
	turbo.NewLimitPool(context.Background(), turbo.NewTimerWheel(100, 10), 100),
	func(serviceUri, host string, moainfo MoaInfo) {},
	func() turbo.NetworkStat { return turbo.NetworkStat{} })

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
		if f.Kind() != reflect.String && f.Kind() != reflect.Struct && f.Kind() != reflect.Interface{
			t.Fail()
		}
	}
}

func TestInvocationInvoke(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: DemoProxy{}, Interface: (*IProxyDemo)(nil)}}, stat)
	req := &MoaReqPacket{}
	req.ServiceUri = "demo"
	req.Params.Args = []interface{}{"fuck", DemoParam{"you"}}
	req.Params.Method = "proxydemo"
	req.Timeout = 5 * time.Second
	handler.Invoke(context.TODO(),*MoaRequest2Raw(req), func(resp MoaRespPacket) error {
		t.Logf("TestInvocationInvoke|Invoke|%v\n", resp)
		if resp.ErrCode != 200 && resp.ErrCode != 0 {
			t.Fail()
		} else {
			data, _ := json.Marshal(resp.Result)
			t.Logf("TestInvocationInvoke|Invoke|Result|%s\n", string(data))
		}
		return nil
	})

}

func TestInvokeProxyDemoSlice(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: DemoProxy{}, Interface: (*IProxyDemo)(nil)}}, stat)
	req := &MoaReqPacket{}
	req.ServiceUri = "demo"
	req.Params.Args = []interface{}{"fuck", []string{"a", "b"}, ProxyParam{"you"}}
	req.Params.Method = "ProxyDemoSlice"
	req.Timeout = 5 * time.Second
	handler.Invoke(context.TODO(),*MoaRequest2Raw(req), func(resp MoaRespPacket) error {
		t.Logf("TestInvokeProxyDemoSlice|Invoke|%s\n", resp.Result)
		if resp.ErrCode != 200 && resp.ErrCode != 0 {
			t.Fail()
		} else {
			data, _ := json.Marshal(resp.Result)
			t.Logf("TestInvokeProxyDemoSlice|Invoke|Result|%s\n", string(data))
		}
		return nil
	})

}

func TestInvokeJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{Service{ServiceUri: "demo",
		Instance: DemoProxy{}, Interface: (*IProxyDemo)(nil)}}, stat)

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"ProxyDemoSlice\",\"args\":[\"fuck\",[\"a\", \"b\"],{\"Name\":\"you\"}]}}"
	var req MoaRawReqPacket
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}
	t.Log(req)
	req.Timeout = 5 * time.Second
	handler.Invoke(context.TODO(),req, func(resp MoaRespPacket) error {
		t.Logf("TestInvokeProxyDemoSlice|Invoke|%s\n", resp.Result)
		if resp.ErrCode != 200 && resp.ErrCode != 0 {
			t.Fail()
		} else {
			data, _ := json.Marshal(resp.Result)
			t.Logf("TestInvokeProxyDemoSlice|Invoke|Result|%s\n", string(data))
		}
		return nil
	})
}

func TestComplexSliceJsonParams(t *testing.T) {
	handler := NewInvocationHandler([]Service{
		Service{
			ServiceUri: "demo",
			Instance:   DemoProxy{},
			Interface:  (*IProxyDemo)(nil)}},
		stat)

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"ProxyDemoComplexSlice\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	var req MoaRawReqPacket
	err := json.Unmarshal([]byte(cmd), &req)
	if nil != err {
		t.Error(err)
	}

	req.Timeout = 5 * time.Second
	handler.Invoke(context.TODO(),req, func(resp MoaRespPacket) error {
		t.Logf("TestInvokeProxyDemoSlice|Invoke|%v\n", resp)
		if resp.ErrCode != 200 && resp.ErrCode != 0 {
			t.Fail()
		} else {
			data, _ := json.Marshal(resp.Result)
			t.Logf("TestInvokeProxyDemoSlice|Invoke|Result|%s\n", string(data))
		}
		return nil
	})
}
