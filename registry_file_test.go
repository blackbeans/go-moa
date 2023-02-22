package core

import "testing"

func TestFileRegistry(t *testing.T) {
	registry := NewFileRegistry("./conf/cluster.yaml", []string{"/service/lookup"}, false)
	metas, err := registry.GetService("/service/lookup", PROTOCOL, "")
	if nil != err {
		t.FailNow()
	}
	t.Logf("/service/lookup =>%v", metas)
	if len(metas) < 2 {
		t.FailNow()
	}

}
