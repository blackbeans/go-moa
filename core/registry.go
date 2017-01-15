package core

import (
	"github.com/blackbeans/go-moa/lb"
	log "github.com/blackbeans/log4go"
	"strings"
)

const (
	SCHEMA_ZK = "zk://"
)

type ConfigCenter struct {
	registry lb.IRegistry
	services []Service
	hostport string
	groupId  string
}

//用于创建
func NewConfigCenter(registryAddr,
	hostport, groupId string, services []Service) *ConfigCenter {
	var reg lb.IRegistry
	if strings.HasPrefix(registryAddr, SCHEMA_ZK) {
		uris := make([]string, 0, 10)
		for _, s := range services {
			uris = append(uris, lb.BuildServiceUri(s.ServiceUri, s.GroupId))
		}
		reg = lb.NewZkRegistry(strings.TrimPrefix(registryAddr, SCHEMA_ZK), uris, true)
	}
	center := &ConfigCenter{registry: reg, services: services, hostport: hostport, groupId: groupId}
	//	 zookeeper发布一次吧
	center.RegisteAllServices()
	return center
}

func (self ConfigCenter) RegisteAllServices() {
	//注册服务
	for _, s := range self.services {
		succ := self.RegisteService(s.ServiceUri, self.hostport, lb.PROTOCOL)
		if !succ {
			panic("ConfigCenter|RegisteAllServices|FAIL|" + s.ServiceUri)
		}
	}

}

func (self ConfigCenter) RegisteService(serviceUri, hostport, protoType string) bool {
	return self.registry.RegisteService(serviceUri, hostport, protoType, self.groupId)
}

func (self ConfigCenter) UnRegisteService(serviceUri, hostport, protoType string) bool {
	return self.registry.UnRegisteService(serviceUri, hostport, protoType, self.groupId)
}

func (self ConfigCenter) GetService(serviceUri, protoType string) ([]string, error) {
	return self.registry.GetService(serviceUri, protoType, self.groupId)
}

func (self ConfigCenter) Destroy() {
	//注册服务
	for _, s := range self.services {
		succ := self.UnRegisteService(s.ServiceUri, self.hostport, lb.PROTOCOL)
		if succ {
			log.InfoLog("config_center", "ConfigCenter|Destroy|UnRegisteService|SUCC|%s", s.ServiceUri)
		} else {
			log.InfoLog("config_center", "ConfigCenter|Destroy|UnRegisteService|FAIL|%s", s.ServiceUri)
		}
	}
	self.registry.Destroy()
}
