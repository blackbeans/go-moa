package core

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"sort"
	"strings"
)

//
type LocalService struct {
	ServiceUri   string   `yaml:"service_uri"` //serviceUr对应的服务名称
	GroupId      string   `yaml:"gid"`         //该服务的分组
	ProtoVersion string   `yaml:"proto_ver"`   //协议版本
	IsPre        bool     `yaml:"isPre"`       //是否是预发环境
	HostPorts    []string `yaml:"hostports"`   //节点
}

type FileRegistry struct {
	service      []string
	uri2Services map[string][]ServiceMeta
	serverModel  bool
}

func NewFileRegistry(yamlPath string, service []string, serverModel bool) *FileRegistry {

	uri2Services := make(map[string][]ServiceMeta, 2)

	zoo := &FileRegistry{}
	zoo.service = service
	zoo.uri2Services = uri2Services
	zoo.serverModel = serverModel

	if !serverModel {
		// 加载本地的配置
		rawYaml, err := ioutil.ReadFile(yamlPath)
		if nil != err {
			panic(err)
		}

		var localServices struct {
			Clusters []LocalService `yaml:"clusters"`
		}
		err = yaml.Unmarshal(rawYaml, &localServices)
		if nil != err {
			panic(err)
		}

		for _, s := range localServices.Clusters {
			for _, hp := range s.HostPorts {
				uri := BuildServiceUri(s.ServiceUri, s.GroupId)
				ss, ok := uri2Services[uri]
				if !ok {
					ss = []ServiceMeta{}
				}

				if len(s.ProtoVersion) <= 0 {
					s.ProtoVersion = PROTOCOL
				}

				uri2Services[uri] = append(ss, ServiceMeta{
					ServiceUri:   s.ServiceUri,
					GroupId:      s.GroupId,
					HostPort:     hp,
					ProtoVersion: s.ProtoVersion,
					IsPre:        s.IsPre,
				})
			}
		}
	} else {
		// server

	}

	return zoo
}

//获取孩子节点的数据
func (self *FileRegistry) PullChildrenData(pathPrefix string, uri string, hosts ...string) ([]ServiceMeta, error) {
	sort.Strings(hosts)
	services := make([]ServiceMeta, 0, len(hosts))

	ss, ok := self.uri2Services[uri]
	if ok {
		for _, s := range ss {
			for _, host := range hosts {
				if strings.HasPrefix(s.HostPort, host) {
					services = append(services, s)
				}
			}
		}
	}

	return services, nil
}

func (self *FileRegistry) RegisteService(serviceUri, hostport, protoType, groupId string, s ServiceMeta) bool {
	return true
}

func (self *FileRegistry) UnRegisteService(serviceUri, hostport, protoType, groupId string) bool {
	return true
}

func (self *FileRegistry) GetService(serviceUri, protoType, groupId string) ([]ServiceMeta, error) {

	key := BuildServiceUri(serviceUri, groupId)
	hosts, ok := self.uri2Services[key]
	if !ok {
		if len(hosts) < 1 {
			return nil, errors.New(fmt.Sprintf("No Hosts! /moa/service/%s%s", protoType, serviceUri))
		}
	}
	validMetas := make([]ServiceMeta, 0, 2)
	for _, h := range hosts {
		if h.ProtoVersion == protoType {
			validMetas = append(validMetas, h)
		}
	}
	if len(validMetas) < 1 {
		return nil, errors.New(fmt.Sprintf("No Hosts! /moa/service/%s%s", protoType, serviceUri))
	}

	return validMetas, nil
}

//会话超时时，需要重新订阅/推送watcher
func (self *FileRegistry) OnSessionExpired() {

}

// 用户客户端监听服务节点地址发生变化时触发
func (self *FileRegistry) NodeChange(path string, eventType ZkEvent, addrs []string) {

}

func (self *FileRegistry) Destroy() {

}
