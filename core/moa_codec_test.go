package core

import (
	_ "bytes"
	"encoding/json"
	"reflect"
	"testing"
)

type ParamsTmp struct {
	Method string `json:"m"`
	// Args   []interface{} `json:"args"`
	Args []*json.RawMessage `json:"args"`
}

type User struct {
	Name string `json:"name"`
	// Args   []interface{} `json:"args"`
}

type Request struct {
	ServiceUri string `json:"action"`
	Params     struct {
		Method string `json:"m"`
		// Args   []interface{} `json:"args"`
		Args []*json.RawMessage `json:"args"`
	} `json:"params"`
}

func BenchmarkUnmarshal(t *testing.B) {
	t.StopTimer()
	cmd := []byte("{\"action\":\"/service/user-service\",\"params\":{\"m\":\"setName\",\"args\":[\"a\",1,2,{\"name\":\"bbafa\"}]}}")
	t.StartTimer()
	var reqtmp Request
	for i := 0; i < t.N; i++ {
		var req Request
		json.Unmarshal(cmd, &req)
		// args := []interface{}{"", 0, 0, ""}
		// json.Unmarshal(req.Params.Args, &args)
		reqtmp = req
	}
	t.StopTimer()

	inst := reflect.New(reflect.ValueOf((*User)(nil)).Type())
	json.Unmarshal(*reqtmp.Params.Args[3], inst.Interface())
	t.Log(inst.Elem().Interface())

}
