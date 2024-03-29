package core

//
//import (
//	"testing"
//	"time"
//)
//
//func TestOldZKRegisteService(t *testing.T) {
//
//	// t.Log("test")
//	regAddr := "localhost:2181"
//	serviceUri := "/service/bibi-profile"
//	protocol := "v1"
//	hostport := "localhost:18000"
//
//	registry := NewZkRegistry(regAddr, []string{serviceUri}, false)
//
//	registry.RegisteService(serviceUri, hostport, "v1", "*", ServiceMeta{
//		ServiceUri:   serviceUri,
//		GroupId:      "",
//		HostPort:     hostport,
//		ProtoVersion: protocol,
//		IsPre:        false,
//	})
//
//	time.Sleep(10 * time.Second)
//	data, err := registry.GetService(serviceUri, protocol, "*")
//	if err != nil {
//		t.Fail()
//		t.Logf("GetService FAIL! %s", err.Error())
//	} else if len(data) > 0 {
//		t.Logf("GetService %d-> %v SUCC", len(data), data)
//	} else {
//		t.Fail()
//	}
//
//	flag := registry.UnRegisteService(serviceUri, hostport, protocol, "*")
//	if !flag {
//		t.Fatalf("UnRegisteService %s Fail", serviceUri)
//	}
//
//	time.Sleep(5 * time.Second)
//	data, err = registry.GetService(serviceUri, protocol, "*")
//	if err != nil {
//		t.Fail()
//		t.Logf("GetService FAIL! %s", err.Error())
//	} else if len(data) > 0 {
//		t.Fail()
//		t.Logf("GetService %d-> %v fail", len(data), data)
//	} else {
//		t.Logf("GetService %d-> %v SUCC", len(data), data)
//	}
//
//}
//
//func TestGroupZKRegisteService(t *testing.T) {
//
//	// t.Log("test")
//	regAddr := "localhost:2181"
//	serviceUri := "/service/bibi-profile"
//	protocol := "v1"
//	hostport := "localhost:18000"
//	groupId := "s-mts-group"
//
//	groupUri := BuildServiceUri("/service/bibi-profile", "s-mts-group")
//	registry := NewZkRegistry(regAddr, []string{groupUri}, false)
//
//	registry.RegisteService(serviceUri, hostport, protocol, groupId, ServiceMeta{
//		ServiceUri:   serviceUri,
//		GroupId:      groupId,
//		HostPort:     hostport,
//		ProtoVersion: protocol,
//		IsPre:        false,
//	})
//	time.Sleep(10 * time.Second)
//	data, err := registry.GetService(serviceUri, protocol, groupId)
//	if err != nil || len(data) <= 0 {
//		t.Fail()
//		t.Logf("GetService FAIL! %s", err)
//	} else {
//		t.Logf("GetService %d-> %v SUCC", len(data), data)
//	}
//
//	//different groupId
//	data, err = registry.GetService(serviceUri, protocol, "s-mts-group-2")
//	if err != nil || len(data) <= 0 {
//		t.Logf("No Group GetService [%s] SUCC", "s-mts-group-2")
//	} else {
//		t.Fail()
//		t.Logf("amazing GetService [%s] SUCC~", "s-mts-group-2")
//		return
//	}
//
//	flag := registry.UnRegisteService(serviceUri, hostport, protocol, groupId)
//	if !flag {
//		t.Fatalf("UnRegisteService %s Fail", serviceUri)
//	}
//
//	time.Sleep(10 * time.Second)
//	data, err = registry.GetService(serviceUri, protocol, groupId)
//	if err != nil {
//		t.Fail()
//		t.Logf("GetService FAIL! %s", err.Error())
//	} else if len(data) > 0 {
//		t.Fail()
//		t.Logf("GetService %d-> %v fail", len(data), data)
//	} else {
//		t.Logf("GetService %d-> %v SUCC", len(data), data)
//	}
//
//}
