package lb

import (
	"fmt"
	"github.com/blackbeans/go-moa/core"
	"github.com/blackbeans/go-moa/proxy"
	"strings"
	"testing"
)

//{"hosts":["10.83.76.80:31001?timeout=1000&version=2","10.83.76.78:31001?timeout=1000&version=2","10.83.76.79:31001?timeout=1000&version=2"],"uri":"/service/lookup"}

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type IHello interface {
	GetService(serviceUri, proto, groupId string) (DemoResult, error)
	// 注册
	RegisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error)
	// 注销
	UnregisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error)

	IsIsolated(serviceUri, hostPort, proto, groupId string, config map[string]string) (bool, error)
}

type DemoParam struct {
	Name string
}

type Lookup struct {
	hosts map[string][]string
	uri   string
}

func (self Lookup) GetService(serviceUri, proto, groupId string) (DemoResult, error) {
	fmt.Printf("GetService|SUCC|%s|%s|%s|%v\n", serviceUri, groupId, proto, self.hosts)
	result := DemoResult{}
	val, _ := self.hosts[serviceUri+"_"+proto+"_"+groupId]
	result.Hosts = val
	result.Uri = self.uri

	return result, nil
}

func (self Lookup) IsIsolated(serviceUri, hostPort, proto, groupId string, config map[string]string) (bool, error) {
	// delete(self.hosts, serviceUri+"_"+proto+"_"+groupId)
	fmt.Printf("IsIsolated|SUCC|%s|%s|%s\n", serviceUri, proto, groupId)
	return true, nil
}

// 注册
func (self Lookup) RegisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error) {
	fmt.Printf("RegisterService|SUCC|%s|%s|%s\n", serviceUri, proto, groupId)
	self.hosts[serviceUri+"_"+proto+"_"+groupId] = []string{hostPort + "?timeout=1000&version=2&groupId=" + groupId}

	return "SUCCESS", nil
}

// 注销
func (self Lookup) UnregisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error) {
	delete(self.hosts, serviceUri+"_"+proto+"_"+groupId)
	fmt.Printf("UnregisterService|SUCC|%s|%s|%s\n", serviceUri, proto, groupId)
	return "SUCCESS", nil
}

var app *core.Application

func init() {
	demo := Lookup{make(map[string][]string, 2), "/service/lookup"}
	app = core.NewApplcation("../conf/cluster_test.toml", func() []proxy.Service {
		return []proxy.Service{
			proxy.Service{
				ServiceUri: "/service/lookup",
				Instance:   demo,
				Interface:  (*IHello)(nil)},
			proxy.Service{
				ServiceUri: "/service/moa-admin",
				Instance:   demo,
				Interface:  (*IHello)(nil)},
		}
	})

}

func TestOldRegisteService(t *testing.T) {
	center := NewConfigCenter("momokeeper", "localhost:13000,localhost:13000", "localhost:12000", "*", nil)
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

func TestRegisteService(t *testing.T) {
	center := NewConfigCenter("momokeeper", "localhost:13000,localhost:13000", "localhost:12000", "s-mts-group", nil)

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
	center.Destroy()
}
