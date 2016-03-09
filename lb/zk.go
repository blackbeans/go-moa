package lb

import (
	"bytes"
	"errors"
	"github.com/blackbeans/go-zookeeper/zk"
	log "github.com/blackbeans/log4go"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ZK_MOA_ROOT_PATH  = "/go-moa/services"
	ZK_ROOT           = "/"
	ZK_PATH_DELIMITER = "/"
)

type zookeeper struct {
	regAddr    string
	regCon     *zk.Conn
	serviceUri string
	lock       sync.RWMutex
	uri2Hosts  map[string][]string
}

func NewZookeeper(regAddr string) *zookeeper {

	regCon, _, err := zk.Connect([]string{regAddr}, 15*time.Second)
	if err != nil {
		panic(err)
	}

	uri2Hosts := make(map[string][]string, 2)
	return &zookeeper{
		regAddr:   regAddr,
		regCon:    regCon,
		uri2Hosts: uri2Hosts}
}

func (self zookeeper) RegisteService(serviceUri, hostport, protoType string) bool {
	// /go-moa/services/demo_redis/localhost:13000?timeout=1000&protocol=redis
	servicePath := concat(ZK_MOA_ROOT_PATH, concat(serviceUri, "_", protoType), ZK_PATH_DELIMITER, hostport)
	err := self.createServiceNode(servicePath)
	if err != nil {
		return false
	}
	log.InfoLog("config_center", "zookeeper|RegisteService|SUCC|%s|%s|%s", hostport, serviceUri, protoType)

	// zookeeper 坐等zk服务端通知即可
	path := concat(ZK_MOA_ROOT_PATH, concat(serviceUri, "_", PROTOCOL))
	func() {
		defer func() {
			if err := recover(); nil != err {

			}
		}()
		//需要监听拉取服务地址
		self.listenZkNodeChanged(path, serviceUri)
	}()
	return true

}

func (self zookeeper) createServiceNode(servicePath string) error {
	absolutePath := ZK_ROOT
	conn := self.regCon
	for _, path := range strings.Split(servicePath, ZK_PATH_DELIMITER) {
		if len(path) < 1 || path == ZK_ROOT {
			continue
		} else {
			if !strings.HasSuffix(absolutePath, ZK_PATH_DELIMITER) {
				absolutePath = concat(absolutePath, ZK_PATH_DELIMITER)
			}
			absolutePath = concat(absolutePath, path)
			if flag, _, err := conn.Exists(absolutePath); err != nil {
				log.ErrorLog("config_center", "zookeeper|RegisteService|SUCC|%s", servicePath)
				return err
			} else {
				if !flag {
					_, err := conn.Create(absolutePath, []byte{}, 0, zk.WorldACL(zk.PermAll))
					if err != nil {
						log.ErrorLog("config_center", "zookeeper|RegisteService|SUCC|%s", servicePath)
						return err
					}
				}
			}
		}
	}
	return nil
}

func (self zookeeper) UnRegisteService(serviceUri, hostport, protoType string) bool {

	servicePath := concat(ZK_MOA_ROOT_PATH, concat(serviceUri, "_", protoType), ZK_PATH_DELIMITER, hostport)
	conn := self.regCon
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
	data, _ := self.uri2Hosts[serviceUri]
	if len(data) < 1 {
		return nil, errors.New("No Hosts! " + serviceUri + "?protocol=" + protoType)
	}
	return data, nil
}

func (self zookeeper) listenZkNodeChanged(path, uri string) error {
	snapshots, errors := self.fetchNodeChildren(path)
	go func() {
		for {
			select {
			case addrs := <-snapshots:
				//对比变化
				func() {
					defer func() {
						if r := recover(); nil != r {
							//do nothing
						}
					}()
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

					log.DebugLog("config_center", "zookeeper|listenZkNodeChanged|needChange|%s", needChange)
					//变化则更新
					if needChange {
						self.uri2Hosts[uri] = addrs
					}
				}()
			case err := <-errors:
				panic(err)
			}
		}
	}()
	return nil
}

// 监听服务节点孩子节点变化
func (self zookeeper) fetchNodeChildren(path string) (chan []string, chan error) {
	snapshots := make(chan []string)
	errors := make(chan error)
	go func() {
		for {
			snapshot, _, events, err := self.regCon.ChildrenW(path)
			if err != nil {
				errors <- err
				return
			}
			snapshots <- snapshot
			evt := <-events
			if evt.Err != nil {
				errors <- evt.Err
				return
			}
		}
	}()
	return snapshots, errors
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

}
