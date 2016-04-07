package core

import (
	"fmt"
	"github.com/blackbeans/go-moa/proxy"
	"gopkg.in/redis.v3"
	"testing"
)

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type IHello interface {
	GetService(serviceUri, proto string) (DemoResult, error)
	// 注册
	RegisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error)
	// 注销
	UnregisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error)
}

type DemoParam struct {
	Name string
}

type Demo struct {
	hosts map[string][]string
	uri   string
}

func (self Demo) GetService(serviceUri, proto string) (DemoResult, error) {
	result := DemoResult{}
	val, _ := self.hosts[serviceUri+"_"+proto]
	result.Hosts = val
	result.Uri = self.uri
	fmt.Printf("GetService|SUCC|%s|%s|%s\n", serviceUri, proto, result)
	return result, nil
}

// 注册
func (self Demo) RegisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error) {
	self.hosts[serviceUri+"_"+proto] = []string{hostPort + "?timeout=1000&version=2"}
	fmt.Println("RegisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS", nil
}

// 注销
func (self Demo) UnregisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error) {
	delete(self.hosts, serviceUri+"_"+proto)
	fmt.Println("UnregisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS", nil
}

func init() {
	demo := Demo{make(map[string][]string, 2), "/service/lookup"}
	inter := (*IHello)(nil)
	NewApplcation("../conf/cluster_test.toml", func() []proxy.Service {
		return []proxy.Service{
			proxy.Service{
				ServiceUri: "/service/lookup",
				Instance:   demo,
				Interface:  inter},
			proxy.Service{
				ServiceUri: "/service/moa-admin",
				Instance:   demo,
				Interface:  inter},
		}
	})

}

func TestApplication(t *testing.T) {

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:13000",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"GetService\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	val, _ := client.Get(cmd).Result()
	t.Log(val)
	pong, err := client.Ping().Result()
	t.Logf("pong:%s,err:%s\n", pong, err)

}

func BenchmarkApplication(t *testing.B) {
	t.StopTimer()
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:13000",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()
	cmd := "{\"action\":\"demo\",\"params\":{\"m\":\"GetService\",\"args\":[\"fuck\",{\"key\":{\"Name\":\"you\"}},[{\"key\":{\"Name\":\"you\"}},{\"key\":{\"Name\":\"you\"}}]]}}"
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		client.Get(cmd)
	}

}
