package lb

import (
	"bytes"
	"errors"
	"github.com/blackbeans/go-zookeeper/zk"
	log "github.com/blackbeans/log4go"
	"sort"
	// "strings"
	"sync"
	// "time"
)

const (
	// /moa/service/redis/service/relation-service/localhost:13000?timeout=1000&protocol=redis
	ZK_MOA_ROOT_PATH  = "/moa/service"
	ZK_ROOT           = "/"
	ZK_PATH_DELIMITER = "/"
)

type AddressManager struct {
	uri2Hosts map[string][]string
	lock      sync.RWMutex
}

type zookeeper struct {
	serviceUri  []string
	zkManager   *ZKManager
	addrManager *AddressManager
}

func NewZookeeper(regAddr string, uris []string) *zookeeper {

	zkManager := NewZKManager(regAddr)
	uri2Hosts := make(map[string][]string, 2)
	addressManager := &AddressManager{uri2Hosts: uri2Hosts}

	zoo := &zookeeper{serviceUri: uris}

	var watcher IWatcher
	if len(uris) > 0 {
		// client
		watcher = NewMoaClientWatcher(addressManager.OnAddressChange, zoo.ReSubZkServer)
		for _, uri := range uris {
			flag := zkManager.RegisteWather(uri, watcher)
			if !flag {
				log.ErrorLog("config_center", "zookeeper|NewZookeeper|RegisteWather|FAIL|%s", uri)
			}
		}
	} else {
		// server
		watcher = MoaServerWatcher{}
		zkManager.RegisteWather(ZK_MOA_ROOT_PATH, watcher)
	}

	zoo.zkManager = zkManager
	zoo.addrManager = addressManager
	return zoo
}

func (self zookeeper) RegisteService(serviceUri, hostport, protoType string) bool {
	// /moa/service/redis/service/relation-service/localhost:13000?timeout=1000&protocol=redis
	servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, protoType, serviceUri)
	svAddrPath := concat(servicePath, ZK_PATH_DELIMITER, hostport)

	conn := self.zkManager.session
	conn.ChildrenW(servicePath)

	// 创建持久服务节点 /moa/service/redis/service/relation-service
	exist, _, err := conn.Exists(servicePath)
	if err != nil {
		conn.Close()
		panic("无法创建" + servicePath + err.Error())
	}
	if !exist {
		_, err := conn.Create(servicePath, nil, zk.CreatePersistent, zk.WorldACL(zk.PermAll))
		if err != nil {
			panic("NewZookeeper|RegisteService|FAIL|" + servicePath + "|" + err.Error())
		}
	}

	// 创建临时服务地址节点 /moa/service/redis/service/relation-service/localhost:13000?timeout=1000&protocol=redis
	// 先删除，后创建吧。不然zk不通知，就坐等坑爹吧。蛋碎了一地。/(ㄒoㄒ)/~~

	conn.Delete(svAddrPath, 0)

	_, err = conn.Create(svAddrPath, nil, zk.CreateEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		panic("NewZookeeper|RegisteService|FAIL|" + svAddrPath + "|" + err.Error())
	}
	log.InfoLog("config_center", "zookeeper|RegisteService|SUCC|%s|%s|%s", hostport, serviceUri, protoType)

	return true
}

func (self zookeeper) UnRegisteService(serviceUri, hostport, protoType string) bool {

	servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, protoType, serviceUri, ZK_PATH_DELIMITER, hostport)
	conn := self.zkManager.session
	if flag, _, err := conn.Exists(servicePath); err != nil {
		log.ErrorLog("config_center", "zookeeper|UnRegisteService|ERROR|%s|%s|%s", serviceUri, hostport, protoType)
		return false
	} else {
		if flag {
			err := conn.Delete(servicePath, 0)
			if err != nil {
				log.ErrorLog("config_center", "zookeeper|UnRegisteService|ERROR|%s|%s|%s", serviceUri, hostport, protoType)
				return false
			}
		}
	}
	log.InfoLog("config_center", "zookeeper|UnRegisteService|SUCC|%s|%s|%s", hostport, serviceUri, protoType)
	return true
}

func (self zookeeper) GetService(serviceUri, protoType string) ([]string, error) {
	self.addrManager.lock.RLock()
	defer self.addrManager.lock.RUnlock()
	hosts, _ := self.addrManager.uri2Hosts[serviceUri]
	if len(hosts) < 1 {
		return nil, errors.New("No Hosts! " + serviceUri + "?protocol=" + protoType)
	}
	return hosts, nil
}

//需要重新订阅watcher
func (self zookeeper) ReSubZkServer() {
	conn := self.zkManager.session
	for _, uri := range self.serviceUri {
		servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL, uri)
		conn.ChildrenW(servicePath)
	}
	log.InfoLog("config_center", "zookeeper|ReSubZkServer|OK")
}

func (self AddressManager) OnAddressChange(uri string, addrs []string) {
	//对比变化
	self.lock.Lock()
	defer self.lock.Unlock()
	needChange := true
	sort.Strings(addrs)
	oldAddrs, ok := self.uri2Hosts[uri]
	if ok {
		if len(oldAddrs) > 0 &&
			len(oldAddrs) == len(addrs) {
			for j, v := range addrs {
				//如果是最后一个并且相等那么就应该不需要更新
				if oldAddrs[j] == v && j == len(addrs)-1 {
					needChange = false
					break
				}
			}
		}
	}
	log.DebugLog("config_center", "zookeeper|OnAddressChange|uri|needChange|%s|%s|%s", uri, needChange, self.uri2Hosts)
	//变化则更新
	if needChange {
		self.uri2Hosts[uri] = addrs
	}

}

// 拼接字符串
func concat(args ...string) string {
	var buffer bytes.Buffer
	for _, arg := range args {
		buffer.WriteString(arg)
	}
	return buffer.String()
}

func (self zookeeper) Destroy() {
	self.zkManager.Close()
}
