package lb

import (
	"testing"
	"time"
)

func TestZKRegisteService(t *testing.T) {

	// t.Log("test")
	regAddr := "localhost:2181"
	serviceUri := "/service/bibi-service"
	protocol := "redis"
	hostport := "localhost:18000"

	registry := NewZookeeper(regAddr, []string{serviceUri})

	flag := registry.RegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("RegisteService %s FAIL!", serviceUri)
	}

	time.Sleep(5 * time.Second)
	data, err := registry.GetService(serviceUri, protocol)
	if err != nil {
		t.Fail()
		t.Logf("GetService FAIL! %s", err.Error())
	} else if len(data) > 0 {
		t.Logf("GetService %d-> %s SUCC", len(data), data)
	} else {
		t.Fail()
	}

	flag = registry.UnRegisteService(serviceUri, hostport, protocol)
	if !flag {
		t.Fatalf("UnRegisteService %s Fail", serviceUri)
	}

	time.Sleep(5 * time.Second)
	data, err = registry.GetService(serviceUri, protocol)
	if err != nil {
		t.Fail()
		t.Logf("GetService FAIL! %s", err.Error())
	} else if len(data) > 0 {
		t.Fail()
		t.Logf("GetService %d-> %s SUCC", len(data), data)
	} else {
		t.Logf("GetService %d-> %s SUCC", len(data), data)
	}

}
