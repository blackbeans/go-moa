package core

import (
	"errors"
	"log"
	"net"
	"testing"
	"time"

	"github.com/blackbeans/go-moa/proto"
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
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

var remoteClient *client.RemotingClient

func init() {
	demo := Demo{make(map[string][]string, 2), "/service/lookup"}
	inter := (*IHello)(nil)
	NewApplcation("../conf/cluster_test.toml", func() []Service {
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

	rcc := turbo.NewRemotingConfig(
		"turbo-client:localhost:28888",
		1000, 16*1024,
		16*1024, 20000, 20000,
		10*time.Second, 160000)

	remoteClient = client.NewRemotingClient(conn,
		func() codec.ICodec {
			return proto.BinaryCodec{
				MaxFrameLength: packet.MAX_PACKET_BYTES}
		}, clientPacketDispatcher, rcc)
	remoteClient.Start()

}

func clientPacketDispatcher(rclient *client.RemotingClient, resp *packet.Packet) {
	rclient.Attach(resp.Header.Opaque, resp.Data)
}

func TestApplication(t *testing.T) {

	reqPacket := proto.MoaReqPacket{}
	reqPacket.ServiceUri = "/service/lookup"
	reqPacket.Params.Method = "GetService"
	reqPacket.Params.Args = []interface{}{"fuck", "redis", "groupId"}

	p := packet.NewPacket(proto.REQ, nil)
	p.PayLoad = reqPacket

	val, err := remoteClient.WriteAndGet(*p, 5*time.Second)
	if nil != err {
		t.Logf("WriteAndGet|FAIL|%v\n", err)
		t.FailNow()

	}
	t.Logf("%v\n", val)

	pipo := proto.PiPo{Timestamp: time.Now().Unix()}
	p = packet.NewPacket(proto.PING, nil)
	p.PayLoad = pipo
	val, _ = remoteClient.WriteAndGet(*p, 5*time.Second)
	if nil != err {
		t.Logf("WriteAndGet|PING|FAIL|%v\n", err)
		t.FailNow()
	}

	t.Logf("Recieve|PONG|%s", val)
}

func innerTestRPC(t testing.TB) {

}

func BenchmarkApplication(t *testing.B) {
	t.StopTimer()
	reqPacket := proto.MoaReqPacket{}
	reqPacket.ServiceUri = "/service/lookup"
	reqPacket.Params.Method = "GetService"
	reqPacket.Params.Args = []interface{}{"fuck", "redis", "groupId"}

	t.StartTimer()
	for i := 0; i < t.N; i++ {
		p := packet.NewPacket(proto.REQ, nil)
		p.PayLoad = reqPacket
		_, err := remoteClient.WriteAndGet(*p, 5*time.Second)
		if nil != err {
			t.Logf("WriteAndGet|FAIL|%v\n", err)
			t.FailNow()
		}
	}
}
