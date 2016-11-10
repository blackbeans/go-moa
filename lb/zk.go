package lb

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/blackbeans/go-moa/proxy"
	"github.com/blackbeans/go-zookeeper/zk"
	log "github.com/blackbeans/log4go"
	"regexp"
	"sort"
	"sync"
)

const (
	// /moa/service/redis/service/relation-service#{groupId}/localhost:13000?timeout=1000&protocol=redis
	ZK_MOA_ROOT_PATH  = "/moa/service"
	ZK_ROOT           = "/"
	ZK_PATH_DELIMITER = "/"
)

type zookeeper struct {
	service     []proxy.Service
	zkManager   *ZKManager
	uri2Hosts   map[string][]string
	lock        sync.RWMutex
	serverModel bool
}

func NewZookeeper(regAddr string, service []proxy.Service, serverModel bool) *zookeeper {

	zkManager := NewZKManager(regAddr)
	uri2Hosts := make(map[string][]string, 2)

	zoo := &zookeeper{}
	zoo.service = service
	zoo.zkManager = zkManager
	zoo.uri2Hosts = uri2Hosts
	zoo.serverModel = serverModel

	if !serverModel {
		for _, uri := range service {

			// 初始化，由于客户端订阅延迟，需要主动监听节点事件，然后主动从zk上拉取一次，放入缓存
			servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL)
			serviceUri := buildServiceUri(uri.ServiceUri, uri.GroupId)
			//has groupId
			servicePath = concat(servicePath, serviceUri)

			flag := zkManager.RegisteWatcher(servicePath, zoo)

			log.InfoLog("config_center", "zookeeper|NewZookeeper|RegisteWather|%v|%s", flag, servicePath)

			hosts, _, _, err := zkManager.session.ChildrenW(servicePath)
			if err != nil {
				log.ErrorLog("config_center", "zookeeper|NewZookeeper|init uri2hosts|FAIL|%s", servicePath)
			} else {
				sort.Strings(hosts)
				uri2Hosts[serviceUri] = hosts
			}
		}
	} else {
		// server
		zkManager.RegisteWatcher(ZK_MOA_ROOT_PATH, zoo)
	}

	return zoo
}

func buildServiceUri(serviceUri, groupId string) string {
	if len(groupId) > 0 && "*" != groupId {
		return concat(serviceUri, "#", groupId)
	} else {
		return serviceUri
	}
}

func (self zookeeper) RegisteService(serviceUri, hostport, protoType, groupId string) bool {
	// /moa/service/redis/service/relation-service#{groupId}/localhost:13000?timeout=1000&protocol=redis
	// hostport = "localhost:13000" //test
	servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, protoType)
	//has groupId
	servicePath = concat(servicePath, buildServiceUri(serviceUri, groupId))

	svAddrPath := concat(servicePath, ZK_PATH_DELIMITER, hostport)

	conn := self.zkManager.session

	// 创建持久服务节点 /moa/service/redis/service/relation-service#{groupId}
	exist, _, err := conn.Exists(servicePath)
	if err != nil {
		conn.Close()
		panic("无法创建" + servicePath + err.Error())
	}
	if !exist {
		err = self.zkManager.CreateNode(conn, servicePath)
		if err != nil {
			panic("NewZookeeper|RegisteService|FAIL|" + servicePath + "|" + err.Error())
		}
	}

	// 创建临时服务地址节点 /moa/service/redis/service/relation-service#{groupId}/localhost:13000?timeout=1000&protocol=redis
	// 先删除，后创建吧。不然zk不通知，就坐等坑爹吧。蛋碎了一地。/(ㄒoㄒ)/~~

	conn.Delete(svAddrPath, 0)
	_, err = conn.Create(svAddrPath, nil, zk.CreateEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		panic("NewZookeeper|RegisteService|FAIL|" + svAddrPath + "|" + err.Error())
	}
	log.InfoLog("config_center", "zookeeper|RegisteService|SUCC|%s|%s|%s|%s", hostport, serviceUri, protoType, groupId)
	return true
}

func (self zookeeper) UnRegisteService(serviceUri, hostport, protoType, groupId string) bool {

	servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, protoType)
	//has groupId
	servicePath = concat(servicePath, buildServiceUri(serviceUri, groupId), ZK_PATH_DELIMITER, hostport)
	// fmt.Printf("-------%s\n", servicePath)
	conn := self.zkManager.session
	if flag, _, err := conn.Exists(servicePath); err != nil {
		log.ErrorLog("config_center", "zookeeper|UnRegisteService|ERROR|%s|%s|%s|%s|%s",
			err, serviceUri, hostport, protoType, groupId)
		return false
	} else {
		if flag {
			err := conn.Delete(servicePath, 0)
			if err != nil {
				log.ErrorLog("config_center", "zookeeper|UnRegisteService|DEL|ERROR|%s|%s", err, servicePath)
				return false
			}
		}
	}
	log.InfoLog("config_center", "zookeeper|UnRegisteService|SUCC|%s", servicePath)
	return true
}

func (self zookeeper) GetService(serviceUri, protoType, groupId string) ([]string, error) {
	// log.WarnLog("config_center", "zookeeper|GetService|SUCC|%s|%s|%s", serviceUri, protoType, self.addrManager.uri2Hosts)
	self.lock.RLock()
	defer self.lock.RUnlock()
	key := buildServiceUri(serviceUri, groupId)
	hosts, ok := self.uri2Hosts[key]
	if !ok {
		if len(hosts) < 1 {
			return nil, errors.New(fmt.Sprintf("No Hosts! /moa/service/%s%s", protoType, serviceUri))
		}
	}
	return hosts, nil
}

//会话超时时，需要重新订阅/推送watcher
func (self zookeeper) OnSessionExpired() {
	if self.serverModel {
		// 服务端 需要重新推送
		conn := self.zkManager.session
		for uri, hosts := range self.uri2Hosts {
			servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL, uri)
			for _, host := range hosts {
				svAddrPath := concat(servicePath, ZK_PATH_DELIMITER, host)
				conn.Delete(svAddrPath, 0)
				_, err := conn.Create(svAddrPath, nil, zk.CreateEphemeral, zk.WorldACL(zk.PermAll))
				if err != nil {
					panic("ReSubZkServer|FAIL|" + svAddrPath + "|" + err.Error())
				}
			}
		}
		log.InfoLog("config_center", "zookeeper|OnSessionExpired|%v", self.serverModel)
	} else {
		// 客户端需要重新订阅
		conn := self.zkManager.session
		for _, uri := range self.service {
			serviceUri := buildServiceUri(uri.ServiceUri, uri.GroupId)
			servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL, serviceUri)
			conn.ChildrenW(servicePath)
		}
		log.InfoLog("config_center", "zookeeper|OnSessionExpired|%v", self.serverModel)
	}
}

// 用户客户端监听服务节点地址发生变化时触发
func (self zookeeper) NodeChange(path string, eventType ZkEvent, addrs []string) {
	reg, _ := regexp.Compile(`/moa/service/redis([^\s]*)`)
	uri := reg.FindAllStringSubmatch(path, -1)[0][1]
	// fmt.Printf("--------%s\t%s\n", path, uri)
	needChange := true
	//对比变化
	func() {
		self.lock.RLock()
		defer self.lock.RUnlock()

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
	}()
	//变化则更新
	if needChange {
		self.lock.Lock()
		self.uri2Hosts[uri] = addrs
		self.lock.Unlock()
	}
	log.WarnLog("config_center", "zookeeper|NodeChange|%s|%s", uri, addrs)

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
