package core

import (
	"gopkg.in/redis.v3"
	"reflect"
	"testing"
)

func init() {
	NewApplcation("../cluster.toml", func() []Service {

		return []Service{Service{ServiceUri: "demo",
			Instance: Demo{}, Interface: reflect.TypeOf((*IHello)(nil)).Elem()}}
	})

}

func TestApplication(t *testing.T) {

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:13000",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"HelloComplexSlice\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	val, _ := client.Get(cmd).Result()
	t.Log(val)

}

func BenchmarkApplication(t *testing.B) {

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:13000",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	for i := 0; i < t.N; i++ {
		cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"HelloComplexSlice\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
		client.Get(cmd).Result()
	}

}
