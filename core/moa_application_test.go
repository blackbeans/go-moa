package core

import (
	"gopkg.in/redis.v3"
	"testing"
	"time"
)

func init() {
	NewApplcation("../cluster.toml", func() []Service {
		return make([]Service, 0, 2)
	})
}

func TestApplication(t *testing.T) {

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:13000",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	val, _ := client.Get("hello").Result()
	t.Log(val)
	if val != "hello" {
		t.Fail()
	}
}

func BenchmarkApplication(t *testing.B) {

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:13000",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	for i := 0; i < t.N; i++ {
		val, _ := client.Get("hello").Result()
		t.Log(val)
		if val != "hello" {
			t.Fail()
		}
	}

}
