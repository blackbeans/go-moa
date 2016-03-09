package lb

import (
	// "strings"
	// "sort"
	"testing"
	"time"
)

func TestZKRegisteService(t *testing.T) {
	regAddr := "localhost:2181"
	registry := NewZookeeper(regAddr)

	serviceUri := "/demo"
	protocol := "redis"
	hostport := "localhost:18000"
	flag := registry.RegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("RegisteService %s FAIL!", serviceUri)
	}
	go func() {
		for {
			<-time.After(time.Second * 1)
			data, err := registry.GetService(serviceUri, protocol)
			if err != nil {
				t.Fatalf("GetService FAIL! %s", err.Error())
			} else {
				t.Logf("GetService %d-> %s SUCC", len(data), data)
			}
		}
	}()

	<-time.After(time.Second * 2)
	flag = registry.UnRegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("UnRegisteService %s Fail", serviceUri)
	}

	// 模拟多次注册同一个hostport时是否会变更缓存
	<-time.After(time.Second * 2)
	flag = registry.RegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("RegisteService %s FAIL!", serviceUri)
	}

	<-time.After(time.Second * 2)
	flag = registry.RegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("RegisteService %s FAIL!", serviceUri)
	}
	<-time.After(time.Second * 2)

}
