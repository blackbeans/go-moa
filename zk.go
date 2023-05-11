package core

import (
	"github.com/blackbeans/go-zookeeper/zk"
	_ "net"
	"strings"
	"time"
)

type ZKManager struct {
	zkhosts   string
	wathcers  map[string]IWatcher //基本的路径--->watcher zk可以复用了
	session   *zk.Conn
	eventChan <-chan zk.Event
	isClose   bool
}

type ZkEvent zk.EventType

const (
	Created ZkEvent = 1 // From Exists, Get NodeCreated (1),
	Deleted ZkEvent = 2 // From Exists, Get	NodeDeleted (2),
	Changed ZkEvent = 3 // From Exists, Get NodeDataChanged (3),
	Child   ZkEvent = 4 // From Children NodeChildrenChanged (4)
)

//每个watcher
type IWatcher interface {
	OnSessionExpired()
	// DataChange(path string, binds []*Binding)
	NodeChange(path string, eventType ZkEvent, children []string)
}

func NewZKManager(zkhosts string) *ZKManager {
	zkmanager := &ZKManager{zkhosts: zkhosts, wathcers: make(map[string]IWatcher, 10)}
	zkmanager.Start()

	return zkmanager
}

func (self *ZKManager) Start() {
	if len(self.zkhosts) <= 0 {
		log.Warnf("使用默认zkhosts！|localhost:2181")
		self.zkhosts = "localhost:2181"
	} else {
		log.Infof("使用zkhosts:[%s]", self.zkhosts)
	}

	ss, eventChan, err := zk.Connect(strings.Split(self.zkhosts, ","), 5*time.Second)
	if nil != err {
		panic("连接zk失败..." + err.Error())
		return
	}
	self.CreateNode(ss, ZK_MOA_ROOT_PATH+ZK_PATH_DELIMITER+PROTOCOL)
	self.session = ss
	self.isClose = false
	self.eventChan = eventChan
	go self.listenEvent()
}

func (self *ZKManager) CreateNode(conn *zk.Conn, servicePath string) error {
	absolutePath := ZK_ROOT
	for _, path := range strings.Split(servicePath, ZK_PATH_DELIMITER) {
		if len(path) < 1 || path == ZK_ROOT {
			continue
		} else {
			if !strings.HasSuffix(absolutePath, ZK_PATH_DELIMITER) {
				absolutePath = concat(absolutePath, ZK_PATH_DELIMITER)
			}
			absolutePath = concat(absolutePath, path)
			if flag, _, err := conn.Exists(absolutePath); err != nil {
				log.Errorf("NewZKManager|CreateNode|FAIL|%s", servicePath)
				return err
			} else {
				if !flag {
					resp, err := conn.Create(absolutePath, []byte{}, zk.CreatePersistent, zk.WorldACL(zk.PermAll))
					if err != nil {
						conn.Close()
						panic("NewZKManager|CreateNode|FAIL|" + servicePath)
					} else {
						log.Infof("NewZKManager|CREATE ROOT PATH|SUCC|%s", resp)
					}
				}
			}
		}
	}
	return nil
}

//如果返回false则已经存在
func (self *ZKManager) RegisteWatcher(rootpath string, w IWatcher) bool {
	_, ok := self.wathcers[rootpath]
	if ok {
		return false
	} else {
		self.wathcers[rootpath] = w
		return true
	}
}

//监听数据变更
func (self *ZKManager) listenEvent() {
	for !self.isClose {

		//根据zk的文档 Watcher机制是无法保证可靠的，其次需要在每次处理完Watcher后要重新注册Watcher
		change := <-self.eventChan
		path := change.Path
		// log.Warnf( "NewZKManager|listenEvent|path|%s|%s|%s", path, change.State, change.Type)
		//开始检查符合的watcher
		watcher := func() IWatcher {
			for k, w := range self.wathcers {
				//以给定的
				if strings.Index(path, k) >= 0 {
					return w
				}
			}
			return nil
		}()

		//如果没有wacher那么久忽略
		if nil == watcher {
			log.Warnf("ZKManager|listenEvent|NO  WATCHER|path:%s|event:%v", path, change.State)
			continue
		}

		switch change.Type {
		case zk.EventSession:
			if change.State == zk.StateExpired || change.State == zk.StateDisconnected {
				log.Warnf("ZKManager|OnSessionExpired!|Reconnect Zk ....")
				//session失效必须通知所有的watcher
				func() {
					for _, w := range self.wathcers {
						//zk链接开则需要重新链接重新推送
						w.OnSessionExpired()
					}
				}()

			}
		case zk.EventNodeDeleted:
			self.session.ExistsW(path)
			watcher.NodeChange(path, ZkEvent(change.Type), []string{})
			// log.Info("ZKManager|listenEvent|%s|%s\n", path, change)

		case zk.EventNodeCreated, zk.EventNodeChildrenChanged:
			childnodes, _, _, err := self.session.ChildrenW(path)
			if nil != err {
				log.Errorf("ZKManager|listenEvent|CD|%s|%s|%v", err, path, change.Type)
			} else {
				watcher.NodeChange(path, ZkEvent(change.Type), childnodes)
				// log.Info("ZKManager|listenEvent|%s|%s|%s\n", path, change, childnodes)
			}

		}
	}
}

func (self *ZKManager) Close() {
	self.isClose = true
	self.session.Close()
}
