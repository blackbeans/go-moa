package core

import (
	"context"
	"errors"
	"log"
	"net"
	"testing"
	"time"

	"github.com/blackbeans/turbo"
)

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type IHello interface {
	GetService(serviceUri, proto, groupId string) (DemoResult, error)

	HelloError(text string) (DemoResult, error)
	// 注册
	RegisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error)
	// 注销
	UnregisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error)
}

type DemoParam struct {
	Name string
}

type Demo struct {
	hosts map[string][]string
	uri   string
}

func (self Demo) GetService(serviceUri, proto, groupId string) (DemoResult, error) {
	result := DemoResult{}
	val, _ := self.hosts[serviceUri+"_"+proto+"_"+groupId]
	result.Hosts = val
	result.Uri = self.uri
	//	fmt.Printf("GetService|SUCC|%s|%s|%s\n", serviceUri, proto, result)
	return result, nil
}

// 注册
func (self Demo) RegisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error) {
	self.hosts[serviceUri+"_"+proto+"_"+groupId] = []string{hostPort + "?timeout=1000&version=2"}
	//	fmt.Println("RegisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS", nil
}

// 注销
func (self Demo) UnregisterService(serviceUri, hostPort, proto, groupId string, config map[string]string) (string, error) {
	delete(self.hosts, serviceUri+"_"+proto+"_"+groupId)
	//fmt.Println("UnregisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS", nil
}

func (self Demo) HelloError(text string) (DemoResult, error) {
	return DemoResult{}, errors.New(text)
}

var tclient *turbo.TClient

func init() {
	demo := Demo{make(map[string][]string, 2), "/service/lookup"}
	inter := (*IHello)(nil)
	NewApplication("./conf/moa.toml", func() []Service {
		return []Service{
			Service{
				ServiceUri: "/service/lookup",
				Instance:   demo,
				Interface:  inter},
			Service{
				ServiceUri: "/service/moa-admin",
				Instance:   demo,
				Interface:  inter},
		}
	})

	//创建物理连接
	conn, _ := func(hostport string) (*net.TCPConn, error) {
		//连接
		remoteAddr, err_r := net.ResolveTCPAddr("tcp4", hostport)
		if nil != err_r {
			log.Printf("KiteClientManager|RECONNECT|RESOLVE ADDR |FAIL|remote:%s\n", err_r)
			return nil, err_r
		}
		conn, err := net.DialTCP("tcp4", nil, remoteAddr)
		if nil != err {
			log.Printf("KiteClientManager|RECONNECT|%s|FAIL|%s\n", hostport, err)
			return nil, err
		}

		return conn, nil
	}("localhost:13000")

	config := turbo.NewTConfig(
		"turbo-client:localhost:28888",
		10, 16*1024,
		16*1024, 20000, 20000,
		10*time.Second,
		50*10000)

	tclient = turbo.NewTClient(context.Background(), conn,
		func() turbo.ICodec {
			return BinaryCodec{
				MaxFrameLength: turbo.MAX_PACKET_BYTES}
		}, func(ctx *turbo.TContext) error {
			ctx.Client.Attach(ctx.Message.Header.Opaque, ctx.Message.Data)
			return nil
		}, config)
	tclient.Start()

}

func TestApplication(t *testing.T) {
	reqPacket := MoaReqPacket{}
	reqPacket.ServiceUri = "/service/lookup"
	reqPacket.Params.Method = "GetService"
	reqPacket.Params.Args = []interface{}{"fuck", "redis", "groupId"}
	reqPacket.Properties = map[string]string{
		"LAGN": "zh-CN",
	}

	p := turbo.NewPacket(REQ, nil)
	p.PayLoad = reqPacket

	val, err := tclient.WriteAndGet(*p, 60*time.Second)
	if nil != err {
		t.Logf("WriteAndGet|FAIL|%v\n", err)
		t.FailNow()

	}
	t.Logf("%v\n", val)

	pipo := PiPo{Timestamp: time.Now().Unix()}
	p = turbo.NewPacket(PING, nil)
	p.PayLoad = pipo
	val, _ = tclient.WriteAndGet(*p, 5*time.Second)
	if nil != err {
		t.Logf("WriteAndGet|PING|FAIL|%v\n", err)
		t.FailNow()
	}

	t.Logf("Recieve|PONG|%s", val)

	//panic

	reqPacket.ServiceUri = "/service/lookup"
	reqPacket.Params.Method = "HelloError"
	reqPacket.Params.Args = []interface{}{"error test"}

	for i := 0; i < 10; i++ {
		p = turbo.NewPacket(REQ, nil)
		p.PayLoad = reqPacket

		val, err = tclient.WriteAndGet(*p, 60*time.Second)
		if nil != err {
			t.Logf("WriteAndGet|FAIL|%v\n", err)
			t.FailNow()

		}

		t.Logf("%v\n", string(val.([]byte)))
	}

	time.Sleep(5 * time.Second)
}

func BenchmarkApplication(t *testing.B) {
	t.StopTimer()
	reqPacket := MoaReqPacket{}
	reqPacket.ServiceUri = "/service/lookup"
	reqPacket.Params.Method = "GetService"
	reqPacket.Params.Args = []interface{}{"fuck", "redis", "groupId"}

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		p := turbo.NewPacket(REQ, nil)
		p.PayLoad = reqPacket
		_, err := tclient.WriteAndGet(*p, 5*time.Second)
		if nil != err {
			t.Logf("WriteAndGet|FAIL|%v\n", err)
			t.FailNow()
		}
	}
}
