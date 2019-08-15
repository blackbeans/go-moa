package core

import (
	"strings"

	log "github.com/blackbeans/log4go"
)

const (
	SCHEMA_ZK = "zk://"
)

type ConfigCenter struct {
	registry IRegistry
	services []Service
	hostport string
}

//用于创建
func NewConfigCenter(registryAddr,
	hostport string, services []Service) *ConfigCenter {
	var reg IRegistry
	if strings.HasPrefix(registryAddr, SCHEMA_ZK) {
		uris := make([]string, 0, 10)
		for _, s := range services {
			uris = append(uris, BuildServiceUri(s.ServiceUri, s.GroupId))
		}
		reg = NewZkRegistry(strings.TrimPrefix(registryAddr, SCHEMA_ZK), uris, true)
	}
	center := &ConfigCenter{registry: reg, services: services, hostport: hostport}
	//	 zookeeper发布一次吧
	center.RegisteAllServices()
	return center
}

func (self ConfigCenter) RegisteAllServices() {
	//注册服务
	for _, s := range self.services {
		succ := self.RegisteService(s.ServiceUri, self.hostport, PROTOCOL, s.GroupId)
		if !succ {
			panic("ConfigCenter|RegisteAllServices|FAIL|" + s.ServiceUri)
		}
	}

}

func (self ConfigCenter) RegisteService(serviceUri, hostport, protoType, groupid string) bool {
	return self.registry.RegisteService(serviceUri, hostport, protoType, groupid)
}

func (self ConfigCenter) UnRegisteService(serviceUri, hostport, protoType, groupid string) bool {
	return self.registry.UnRegisteService(serviceUri, hostport, protoType, groupid)
}

func (self ConfigCenter) GetService(serviceUri, protoType string, groupid string) ([]string, error) {
	return self.registry.GetService(serviceUri, protoType, groupid)
}

func (self ConfigCenter) Destroy() {
	//注册服务
	for _, s := range self.services {
		succ := self.UnRegisteService(s.ServiceUri, self.hostport, PROTOCOL, s.GroupId)
		if succ {
			log.InfoLog("config_center", "ConfigCenter|Destroy|UnRegisteService|SUCC|%s", s.ServiceUri)
		} else {
			log.InfoLog("config_center", "ConfigCenter|Destroy|UnRegisteService|FAIL|%s", s.ServiceUri)
		}
	}
	self.registry.Destroy()
}
