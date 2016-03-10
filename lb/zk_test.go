package lb

import (
	// "strings"
	// "sort"
	// "github.com/blackbeans/go-zookeeper/zk"
	// log "github.com/blackbeans/log4go"
	// "regexp"
	"testing"
	"time"
)

// func TestRegexp(t *testing.T) {
// 	path := "/moa/service/redis/service/relation-service"
// 	reg, err := regexp.Compile(`/moa/service/redis([^\s]*)`)
// 	if err != nil {
// 		t.Fatal(err.Error())
// 	}
// 	uri := reg.FindAllStringSubmatch(path, -1)
// 	t.Log("uri:", uri)
// }

// func TestZkManager(t *testing.T) {
// 	zkhosts := "localhost:2181,localhost:2181"
// 	zkManager := NewZKManager(zkhosts)
// 	path := "/moa/service5"

// 	w := &MoaClientWatcher{}

// 	f := zkManager.RegisteWather(path, w)
// 	if f {
// 		t.Log("register watcher succ!", f)
// 	}
// 	<-time.After(time.Second * 3)
// 	// zkManager.CreateNewNode(path)
// 	<-time.After(time.Second * 3)

// }

// func TestZk(t *testing.T) {
// 	// server := "localhost:2181"
// 	// recvTimeout := 10000
// 	// conn, event, err := zk.Connect(server, recvTimeout)
// 	// if err != nil {
// 	// 	t.Log("connect succ!")
// 	// 	zkManager := &ZkManager{conn: conn, event: event}
// 	// 	go zkManager.lisenEvent()
// 	// }

// 	regAddr := "localhost:2181"
// 	uris := []string{"/demo"}
// 	zoo := NewZookeeper(regAddr, uris)

// 	zoo.RegisteService(serviceUri, hostport, protoType)

// }

// func (self ZkManager) lisenEvent() error {
// 	// for {
// 	// 	ev := <-self.event
// 	// 	path := ev.Path
// 	// 	switch(ev.Type) {
// 	// 		case zk.EventNodeChildrenChanged {

// 	// 		}
// 	// 	}
// 	// }
// }

func TestZKRegisteService(t *testing.T) {

	// t.Log("test")
	regAddr := "localhost:2181"
	serviceUri := "/demo"
	protocol := "redis"
	hostport := "localhost:18000"

	registry := NewZookeeper(regAddr, []string{serviceUri})

	flag := registry.RegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("RegisteService %s FAIL!", serviceUri)
	}
	go func() {
		for {
			<-time.After(time.Second * 1)
			data, err := registry.GetService(serviceUri, protocol)
			if err != nil {
				t.Logf("GetService FAIL! %s", err.Error())
			} else {
				t.Logf("GetService %d-> %s SUCC", len(data), data)
			}
		}
	}()

	// 模拟多次注册同一个hostport时是否会变更缓存
	// <-time.After(time.Second * 3)
	// flag = registry.RegisteService(serviceUri, "localhost:18001", protocol)
	// if !flag {
	// 	t.Fatalf("RegisteService %s FAIL!", serviceUri)
	// }

	<-time.After(time.Second * 2)
	flag = registry.UnRegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("UnRegisteService %s Fail", serviceUri)
	}

	// <-time.After(time.Second * 2)
	// flag = registry.RegisteService(serviceUri, hostport, protocol)
	// if !flag {
	// 	t.Fatalf("RegisteService %s FAIL!", serviceUri)
	// }
	<-time.After(time.Second * 2)

}
