package lb

import (
	"strings"
	"testing"
)

func TestZKRegisteService(t *testing.T) {
	center := NewConfigCenter("zookeeper", "localhost:13000,localhost:13000", "localhost:12000", nil)

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
