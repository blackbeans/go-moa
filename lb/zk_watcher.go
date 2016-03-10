package lb

import (
	// "fmt"
	log "github.com/blackbeans/log4go"
	"regexp"
)

// ############## moa client watcher ################

type IAddressListener func(uri string, hosts []string)

type IReInitor func()

type MoaClientWatcher struct {
	listener  IAddressListener
	iReInitor IReInitor
}

func NewMoaClientWatcher(listener IAddressListener, iReInitor IReInitor) *MoaClientWatcher {
	return &MoaClientWatcher{listener: listener, iReInitor: iReInitor}
}

func (self MoaClientWatcher) OnSessionExpired() {
	log.InfoLog("config_center", "MoaClientWatcher|OnSessionExpired")
	// 需要重新拉取并监听服务地址信息，
	self.iReInitor()

}
func (self MoaClientWatcher) NodeChange(path string, eventType ZkEvent, addrs []string) {
	// log.InfoLog("moa_service", "NodeChange|%s|%s|%s", path, eventType, addrs)
	reg, _ := regexp.Compile(`/moa/service/redis([^\s]*)`)
	uri := reg.FindAllStringSubmatch(path, -1)[0][1]
	self.listener(uri, addrs)
}

// ############## moa server watcher ################
type MoaServerWatcher struct {
}

func NewMoaServerWatcher() *MoaServerWatcher {
	return &MoaServerWatcher{}
}

func (self MoaServerWatcher) OnSessionExpired() {
	log.InfoLog("config_center", "MoaServerWatcher|OnSessionExpired")
}

func (self MoaServerWatcher) NodeChange(path string, eventType ZkEvent, children []string) {
	log.InfoLog("moa_service", "NodeChange|%s|%s|%s", path, eventType, children)
}
