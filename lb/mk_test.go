package lb

import (
	"fmt"
	"git.wemomo.com/bibi/go-moa/core"
	"git.wemomo.com/bibi/go-moa/proxy"
	"reflect"
	"strings"
	"testing"
)

//{"hosts":["10.83.76.80:31001?timeout=1000&version=2","10.83.76.78:31001?timeout=1000&version=2","10.83.76.79:31001?timeout=1000&version=2"],"uri":"/service/lookup"}

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type IHello interface {
	GetService(serviceUri, proto string) DemoResult
	// 注册
	RegisterService(serviceUri, hostPort, proto string, config map[string]string) string
	// 注销
	UnregisterService(serviceUri, hostPort, proto string, config map[string]string) string
}

type DemoParam struct {
	Name string
}

type Demo struct {
	hosts map[string][]string
	uri   string
}

func (self Demo) GetService(serviceUri, proto string) DemoResult {
	result := DemoResult{}
	val, _ := self.hosts[serviceUri+"_"+proto]
	result.Hosts = val
	result.Uri = self.uri
	fmt.Printf("GetService|SUCC|%s|%s|%s\n", serviceUri, proto, result)
	return result
}

// 注册
func (self Demo) RegisterService(serviceUri, hostPort, proto string, config map[string]string) string {
	self.hosts[serviceUri+"_"+proto] = []string{hostPort + "?timeout=1000&version=2"}
	fmt.Println("RegisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS"
}

// 注销
func (self Demo) UnregisterService(serviceUri, hostPort, proto string, config map[string]string) string {
	delete(self.hosts, serviceUri+"_"+proto)
	fmt.Println("UnregisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS"
}

var app *core.Application

func init() {
	demo := Demo{make(map[string][]string, 2), "/service/lookup"}
	inter := reflect.TypeOf((*IHello)(nil)).Elem()
	app = core.NewApplcation("../cluster_test.toml", func() []proxy.Service {
		return []proxy.Service{
			proxy.Service{
				ServiceUri: "/service/lookup",
				Instance:   demo,
				Interface:  inter},
			proxy.Service{
				ServiceUri: "/service/moa-admin",
				Instance:   demo,
				Interface:  inter},
		}
	})

}

func TestRegisteService(t *testing.T) {
	center := NewConfigCenter("momokeeper", "localhost:13000,localhost:13000", "localhost:12000", nil)

	succ := center.RegisteService("/service/bibi-profile", "localhost:12000", "redis")
	if !succ {
		t.Fail()
	}

	hosts, err := center.GetService("/service/bibi-profile", "redis")
	if nil != err {
		t.Error(err)
		t.Fail()
		return
	}

	if len(hosts) != 1 {
		t.Log(hosts)
		t.Fail()
		return
	}
	if !strings.HasPrefix(hosts[0], "localhost:12000") {
		t.Log(hosts[0])
		t.Fail()
		return
	}

	succ = center.UnRegisteService("/service/bibi-profile", "localhost:12000", "redis")
	if !succ {
		t.Log(succ)
		t.Fail()
		return
	}

	hosts, err = center.GetService("/service/bibi-profile", "redis")
	if nil != err {
		t.Error(err)
		t.Fail()
		return
	}

	if len(hosts) != 0 {
		t.Log(hosts)
		t.Fail()
		return
	}

}
