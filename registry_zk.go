package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/blackbeans/go-zookeeper/zk"
	log "github.com/sirupsen/logrus"
)

type IRegistry interface {
	RegisteService(serviceUri, hostport, protoType, groupId string, s ServiceMeta) bool
	UnRegisteService(serviceUri, hostport, protoType, groupId string) bool
	GetService(serviceUri, protoType, groupId string) ([]ServiceMeta, error)
	Destroy()
}

type ZkRegistry struct {
	service      []string
	zkManager    *ZKManager
	uri2Services map[string][]ServiceMeta
	lock         sync.RWMutex
	serverModel  bool
}

func NewZkRegistry(regAddr string, service []string, serverModel bool) *ZkRegistry {

	zkManager := NewZKManager(regAddr)
	uri2Services := make(map[string][]ServiceMeta, 2)

	zoo := &ZkRegistry{}
	zoo.service = service
	zoo.zkManager = zkManager
	zoo.uri2Services = uri2Services
	zoo.serverModel = serverModel

	if !serverModel {
		for _, uri := range service {

			// 初始化，由于客户端订阅延迟，需要主动监听节点事件，然后主动从zk上拉取一次，放入缓存
			servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL, uri)

			flag := zkManager.RegisteWatcher(servicePath, zoo)

			log.Infof("ZkRegistry|NewZkRegistry|RegisteWather|%v|%s", flag, servicePath)

			hosts, _, _, err := zkManager.session.ChildrenW(servicePath)
			if err != nil {
				log.Errorf("ZkRegistry|NewZkRegistry|init uri2hosts|FAIL|%s", servicePath)
			} else {

				if services, err := zoo.PullChildrenData(servicePath, uri, hosts...); nil == err {
					uri2Services[uri] = services
				}
			}
		}
	} else {
		// server
		zkManager.RegisteWatcher(ZK_MOA_ROOT_PATH, zoo)
	}

	return zoo
}

//获取孩子节点的数据
func (self *ZkRegistry) PullChildrenData(pathPrefix string, uri string, hosts ...string) ([]ServiceMeta, error) {
	sort.Strings(hosts)
	services := make([]ServiceMeta, 0, len(hosts))
	for _, host := range hosts {
		var meta ServiceMeta
		rawNode, _, _, err := self.zkManager.session.GetW(fmt.Sprintf("%s%s%s", pathPrefix, ZK_PATH_DELIMITER, host))
		if nil == err && nil != rawNode && len(rawNode) > 0 {
			err = json.Unmarshal(rawNode, &meta)
		}
		//这里只是兼容旧的节点服务节点
		if nil != err || nil == rawNode || len(rawNode) <= 0 {
			serviceUri, groupid := UnwrapServiceUri(uri)
			meta = ServiceMeta{
				ServiceUri:   serviceUri,
				GroupId:      groupid,
				HostPort:     host,
				ProtoVersion: PROTOCOL,
				IsPre:        false,
			}
		}
		services = append(services, meta)
	}

	return services, nil
}

func (self *ZkRegistry) RegisteService(serviceUri, hostport, protoType, groupId string, s ServiceMeta) bool {
	// /moa/service/v1/service/relation-service#{groupId}/localhost:13000?timeout=1000&protocol=v1
	// hostport = "localhost:13000" //test
	servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, protoType)
	//has groupId
	servicePath = concat(servicePath, BuildServiceUri(serviceUri, groupId))

	svAddrPath := concat(servicePath, ZK_PATH_DELIMITER, hostport)

	conn := self.zkManager.session

	// 创建持久服务节点 /moa/service/v1/service/relation-service#{groupId}
	exist, _, err := conn.Exists(servicePath)
	if err != nil {
		conn.Close()
		panic("无法创建" + servicePath + err.Error())
	}
	if !exist {
		err = self.zkManager.CreateNode(conn, servicePath)
		if err != nil {
			panic("NewZkRegistry|RegisteService|FAIL|" + servicePath + "|" + err.Error())
		}
	}

	// 创建临时服务地址节点 /moa/service/v1/service/relation-service#{groupId}/localhost:13000?timeout=1000&protocol=v1
	// 先删除，后创建吧。不然zk不通知，就坐等坑爹吧。蛋碎了一地。/(ㄒoㄒ)/~~

	conn.Delete(svAddrPath, 0)
	rawService, _ := json.Marshal(s)
	_, err = conn.Create(svAddrPath, rawService, zk.CreateEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		panic("NewZkRegistry|RegisteService|FAIL|" + svAddrPath + "|" + err.Error())
	}
	log.Infof("ZkRegistry|RegisteService|SUCC|%s|%s|%s|%s", hostport, serviceUri, protoType, groupId)
	return true
}

func (self *ZkRegistry) UnRegisteService(serviceUri, hostport, protoType, groupId string) bool {

	servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, protoType)
	//has groupId
	servicePath = concat(servicePath, BuildServiceUri(serviceUri, groupId), ZK_PATH_DELIMITER, hostport)
	// fmt.Printf("-------%s\n", servicePath)
	conn := self.zkManager.session
	if flag, _, err := conn.Exists(servicePath); err != nil {
		log.Errorf("ZkRegistry|UnRegisteService|ERROR|%s|%s|%s|%s|%s",
			err, serviceUri, hostport, protoType, groupId)
		return false
	} else {
		if flag {
			err := conn.Delete(servicePath, 0)
			if err != nil {
				log.Errorf("ZkRegistry|UnRegisteService|DEL|ERROR|%s|%s", err, servicePath)
				return false
			}
		}
	}
	log.Infof("ZkRegistry|UnRegisteService|SUCC|%s", servicePath)
	return true
}

func (self *ZkRegistry) GetService(serviceUri, protoType, groupId string) ([]ServiceMeta, error) {
	// log.Warnf( "ZkRegistry|GetService|SUCC|%s|%s|%s", serviceUri, protoType, self.addrManager.uri2Services)
	self.lock.RLock()
	defer self.lock.RUnlock()
	key := BuildServiceUri(serviceUri, groupId)
	hosts, ok := self.uri2Services[key]
	if !ok {
		if len(hosts) < 1 {
			return nil, errors.New(fmt.Sprintf("No Hosts! /moa/service/%s%s", protoType, serviceUri))
		}
	}
	return hosts, nil
}

//会话超时时，需要重新订阅/推送watcher
func (self *ZkRegistry) OnSessionExpired() {
	if self.serverModel {
		// 服务端 需要重新推送
		conn := self.zkManager.session
		for uri, serviceMeta := range self.uri2Services {
			servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL, uri)
			for _, s := range serviceMeta {
				svAddrPath := concat(servicePath, ZK_PATH_DELIMITER, s.HostPort)
				conn.Delete(svAddrPath, 0)
				_, err := conn.Create(svAddrPath, nil, zk.CreateEphemeral, zk.WorldACL(zk.PermAll))
				if err != nil {
					panic("ReSubZkServer|FAIL|" + svAddrPath + "|" + err.Error())
				}
			}
		}
		log.Infof("ZkRegistry|OnSessionExpired|%v", self.serverModel)
	} else {
		// 客户端需要重新订阅
		conn := self.zkManager.session
		for _, uri := range self.service {
			servicePath := concat(ZK_MOA_ROOT_PATH, ZK_PATH_DELIMITER, PROTOCOL, uri)
			conn.ChildrenW(servicePath)
		}
		log.Infof("ZkRegistry|OnSessionExpired|%v", self.serverModel)
	}
}

// 用户客户端监听服务节点地址发生变化时触发
func (self *ZkRegistry) NodeChange(path string, eventType ZkEvent, addrs []string) {
	reg, _ := regexp.Compile(`/moa/service/v1([^\s]*)`)
	uri := reg.FindAllStringSubmatch(path, -1)[0][1]
	needChange := true
	//对比变化
	func() {
		self.lock.RLock()
		defer self.lock.RUnlock()

		sort.Strings(addrs)
		oldAddrs, ok := self.uri2Services[uri]
		if ok {
			if len(oldAddrs) > 0 &&
				len(oldAddrs) == len(addrs) {
				for j, v := range addrs {
					//对比下是否相同
					if oldAddrs[j].HostPort == v && j == len(addrs)-1 {
						needChange = false
						break
					}
				}
			}
		}
	}()
	//变化则更新
	if needChange {
		serviceMeta, err := self.PullChildrenData(path, uri, addrs...)
		if nil == err {
			self.lock.Lock()
			self.uri2Services[uri] = serviceMeta
			self.lock.Unlock()
		}
	}
	log.Warnf("ZkRegistry|NodeChange|%s|%s", uri, addrs)

}

func (self *ZkRegistry) Destroy() {
	self.zkManager.Close()
}
