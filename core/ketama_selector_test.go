package core

import (
	"context"
	"reflect"
	"testing"
)

func TestDemo(t *testing.T) {
	ctx := context.WithValue(context.TODO(), "key", "v")
	ctxType := reflect.TypeOf(ctx)
	t1 := reflect.TypeOf(new(context.Context)).Elem()
	eq := ctxType.Implements(t1)
	if eq {
		t.Log("-------------------")
	}
}

func TestKetamaSelector(t *testing.T) {

	nodes := []string{"localhost:2181", "localhost:2182", "localhost:2183"}
	strategy := NewKetamaStrategy(nodes)
	host := strategy.Select("100777")
	t.Log(host)
	if host != "localhost:2182" {
		t.Fail()
	}

	host = strategy.Select("100778")
	t.Log(host)
	if host != "localhost:2181" {
		t.Fail()
	}

	//change
	nodes = []string{"localhost:2186"}
	strategy.ReHash(nodes)
	host = strategy.Select("100777")
	t.Log(host)
	if host != "localhost:2186" {
		t.Fail()
	}
}

func BenchmarkKetamaSelector(b *testing.B) {
	nodes := []string{"localhost:2181", "localhost:2182", "localhost:2183"}
	strategy := NewKetamaStrategy(nodes)
	for i := 0; i < b.N; i++ {
		host := strategy.Select("100777")
		if host != "localhost:2182" {
			b.Fail()
		}
	}
}
