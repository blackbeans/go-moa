package lb

import (
	"bytes"
	"errors"
	"github.com/blackbeans/go-zookeeper/zk"
	log "github.com/blackbeans/log4go"
	"strings"
	"time"
)

const (
	ZK_LOOKUP       = "/service/lookup"
	ZK_SERVICE_URI  = "/service/moa-admin"
	ZK_REG_METHOD   = "registerService"
	ZK_UNREG_METHOD = "unregisterService"
	ZK_GET_METHOD   = "getService"
)

const (
	ZK_MOA_ROOT_PATH  = "/go-moa/services"
	ZK_ROOT           = "/"
	ZK_PATH_DELIMITER = "/"
)

type zookeeper struct {
	regAddr    string
	lookupAddr string
	regCon     *zk.Conn
	lookupCon  *zk.Conn
	serviceUri string
}

func NewZookeeper(regAddr, lookupAddr string) *zookeeper {

	lookupCon, _, err := zk.Connect([]string{lookupAddr}, 15*time.Second)
	if err != nil {
		panic(err)
	}
	regCon, _, err := zk.Connect([]string{regAddr}, 15*time.Second)
	if err != nil {
		panic(err)
	}

	return &zookeeper{
		regAddr,
		lookupAddr,
		regCon,
		lookupCon,
		ZK_SERVICE_URI}
}

func (self zookeeper) RegisteService(serviceUri, hostport, protoType string) bool {
	// /go-moa/services/demo_redis/localhost:13000?timeout=1000&protocol=redis
	servicePath := concat(ZK_MOA_ROOT_PATH, concat(serviceUri, "_", protoType), ZK_PATH_DELIMITER, hostport)
	err := self.createServiceNode(servicePath)
	if err != nil {
		return false
	}
	log.InfoLog("config_center", "zookeeper|RegisteService|SUCC|%s|%s|%s", hostport, serviceUri, protoType)
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
	servicePath := concat(ZK_MOA_ROOT_PATH, concat(serviceUri, "_", protoType))
	conn := self.regCon
	data, _, err := conn.Children(servicePath)
	if err != nil {
		return nil, err
	} else {
		if len(data) < 1 {
			return nil, errors.New("No Hosts! " + serviceUri + "?protocol=" + protoType)
		}
	}
	return data, nil
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
