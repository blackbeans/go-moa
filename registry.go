package core

import (
	"bytes"
	"strings"

	log "github.com/blackbeans/log4go"
)

const (
	SCHEME_ZK   = "zk://"
	SCHEME_FILE = "file://" //直接连接
)

type ConfigCenter struct {
	registry IRegistry
	services []Service
	hostport string
}

//用于创建
func NewConfigCenter(registryAddr,
	hostport string, services []Service) *ConfigCenter {

	uris := make([]string, 0, 10)
	for _, s := range services {
		uris = append(uris, BuildServiceUri(s.ServiceUri, s.GroupId))
	}
	var reg IRegistry
	if strings.HasPrefix(registryAddr, SCHEME_ZK) {
		reg = NewZkRegistry(strings.TrimPrefix(registryAddr, SCHEME_ZK), uris, true)
	} else if strings.HasPrefix(registryAddr, SCHEME_FILE) {
		//本地文件配置
		reg = NewFileRegistry(strings.TrimPrefix(registryAddr, SCHEME_FILE), uris, true)
	}
	center := &ConfigCenter{registry: reg, services: services, hostport: hostport}
	//	 zookeeper发布一次吧
	center.RegisteAllServices()
	return center
}

func (self *ConfigCenter) RegisteAllServices() {
	//注册服务
	for _, s := range self.services {
		succ := self.RegisteService(s.ServiceUri, self.hostport, PROTOCOL, s.GroupId,
			ServiceMeta{
				ServiceUri:   s.ServiceUri,
				GroupId:      s.GroupId,
				IsPre:        s.IsPre,
				ProtoVersion: PROTOCOL,
				HostPort:     self.hostport,
			})
		if !succ {
			panic("ConfigCenter|RegisteAllServices|FAIL|" + s.ServiceUri)
		}
	}

}

func (self *ConfigCenter) RegisteService(serviceUri, hostport, protoType, groupid string, s ServiceMeta) bool {
	s.ServiceUri = serviceUri
	s.HostPort = hostport
	s.ProtoVersion = protoType
	s.GroupId = groupid
	return self.registry.RegisteService(serviceUri, hostport, protoType, groupid, s)
}

func (self *ConfigCenter) UnRegisteService(serviceUri, hostport, protoType, groupid string) bool {
	return self.registry.UnRegisteService(serviceUri, hostport, protoType, groupid)
}

func (self *ConfigCenter) GetService(serviceUri, protoType string, groupid string) ([]ServiceMeta, error) {
	return self.registry.GetService(serviceUri, protoType, groupid)
}

func (self *ConfigCenter) Destroy() {
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

const (
	// /moa/service/v1/service/relation-service#{groupId}/localhost:13000?timeout=1000&protocol=v1
	ZK_MOA_ROOT_PATH  = "/moa/service"
	ZK_ROOT           = "/"
	ZK_PATH_DELIMITER = "/"

	PROTOCOL           = "v1"
	REGISTRY_ZOOKEEPER = "zookeeper"
	ALL_GROUP          = "*"
)

// 拼接字符串
func concat(args ...string) string {
	var buffer bytes.Buffer
	for _, arg := range args {
		buffer.WriteString(arg)
	}
	return buffer.String()
}

func BuildServiceUri(serviceUri, groupId string) string {
	if len(groupId) > 0 && "*" != groupId {
		return concat(serviceUri, "#", groupId)
	} else {
		return serviceUri
	}
}

func UnwrapServiceUri(serviceUri string) (string, string) {
	if strings.IndexAny(serviceUri, "#") >= 0 {
		split := strings.SplitN(serviceUri, "#", 2)
		return split[0], split[1]
	} else {
		return serviceUri, "*"
	}
}
