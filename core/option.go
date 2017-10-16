package core

import (
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/naoina/toml"
)

type HostPort struct {
	Hosts string
}

//配置信息
type Option struct {
	//server配置
	Server struct {
		RunMode     string
		BindAddress string
		Compress    string // compres=snappy
	}

	//client配置
	Client struct {
		RunMode          string
		Compress         string // compres=snappy
		SelectorStrategy string //selectorstrategy="random"

	}
	Clusters map[string]Cluster //各集群的配置
}

//----------------------------------------
//Cluster配置
type Cluster struct {
	Registry          string        //配置中心
	ProcessTimeout    time.Duration //处理超时 5 s单位
	IdleTimeout       time.Duration //链接空闲时间 5 * 60s
	MaxDispatcherSize int           //=8000//最大分发处理协程数
	ReadBufferSize    int           //=16 * 1024 //读取缓冲大小
	WriteBufferSize   int           //=16 * 1024 //写入缓冲大小
	WriteChannelSize  int           //=1000 //写异步channel长度
	ReadChannelSize   int           //=1000 //读异步channel长度
	LogFile           string        //log4go的文件路径
}

func LoadConfiruation(path string) (Option, error) {
	f, err := os.Open(path)
	if err != nil {
		return Option{}, err
	}
	defer f.Close()
	buff, rerr := ioutil.ReadAll(f)
	if nil != rerr {
		return Option{}, err
	}

	//读取配置
	var option Option
	err = toml.Unmarshal(buff, &option)
	if nil != err {
		return Option{}, err
	}
	clusters := make(map[string]Cluster, len(option.Clusters))
	//设置默认值
	for name, cluster := range option.Clusters {
		if cluster.MaxDispatcherSize <= 0 {
			cluster.MaxDispatcherSize = 8000 //最大分发处理协程数
		}

		if cluster.ReadBufferSize <= 0 {
			cluster.ReadBufferSize = 16 * 1024 //读取缓冲大小
		}

		if cluster.WriteBufferSize <= 0 {
			cluster.WriteBufferSize = 16 * 1024 //写入缓冲大小
		}

		if cluster.WriteChannelSize <= 0 {
			cluster.WriteChannelSize = 1000 //写异步channel长度
		}

		if cluster.ReadChannelSize <= 0 {
			cluster.ReadChannelSize = 1000 //读异步channel长度
		}

		//链接空闲时间
		if cluster.IdleTimeout <= 0 {
			cluster.IdleTimeout = 5 * 60
		}

		cluster.IdleTimeout =
			time.Duration(int64(cluster.IdleTimeout) * int64(time.Second))
		if cluster.ProcessTimeout <= 0 {
			cluster.ProcessTimeout = 5
		}

		cluster.ProcessTimeout =
			time.Duration(int64(cluster.ProcessTimeout) * int64(time.Second))
		clusters[name] = cluster
	}
	option.Clusters = clusters
	return option, nil

}

//初始化客户端的Option
func InitClientOption(option Option) Option {

	cluster, ok := option.Clusters[option.Client.RunMode]
	if ok {
		//默认开启snappy
		if len(option.Client.Compress) <= 0 {
			option.Client.Compress = "snappy"
		}
	} else {
		panic("Client RunMode Conf Not Found!")
	}

	strategy := STRATEGY_RANDOM
	switch strings.ToUpper(option.Client.SelectorStrategy) {
	case "KETAMA":
		strategy = STRATEGY_KETAMA
	case "RANDOM":
		fallthrough
	default:
		strategy = STRATEGY_RANDOM
	}

	option.Client.SelectorStrategy = strategy
	option.Clusters[option.Client.RunMode] = cluster
	return option
}

//初始化server的配置
func InitServerOption(option Option) Option {
	//------------寻找匹配的网卡IP段，进行匹配
	split := strings.Split(option.Server.BindAddress, ":")
	regx := split[0]

	inters, err := net.Interfaces()
	if nil != err {
		panic(err)
	} else {
		hasMatched := false
		//如果没有IP匹配表达式则用默认的
		if len(regx) <= 0 {
			option.Server.BindAddress = "0.0.0.0:" + split[1]
			hasMatched = true
		} else {
			for _, inter := range inters {
				addrs, _ := inter.Addrs()
				for _, addr := range addrs {
					if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
						if nil != ip.IP.To4() {
							match, _ := regexp.MatchString(regx, ip.IP.To4().String())
							if match {
								option.Server.BindAddress = ip.IP.To4().String() + ":" + split[1]
								hasMatched = true
								break
							}
						}
					}
				}
			}
		}
		//没有匹配的IP直接用0.0.0.0的IP绑定
		if !hasMatched {
			for _, inter := range inters {
				addrs, _ := inter.Addrs()
				loopback := false
				for _, addr := range addrs {
					ip, ok := addr.(*net.IPNet)
					if ok && ip.IP.IsLoopback() {
						loopback = true
						//skipped
						break
					}
				}

				if !loopback && len(addrs) > 0 {
					for _, addr := range addrs {
						if ip, ok := addr.(*net.IPNet); ok &&
							!ip.IP.IsLoopback() && nil != ip.IP.To4() {
							option.Server.BindAddress = ip.IP.To4().String() + ":" + split[1]
							hasMatched = true
							break
						}
					}
				}
			}

			if !hasMatched {
				option.Server.BindAddress = "0.0.0.0" + ":" + split[1]

			}
		}
	}

	cluster, ok := option.Clusters[option.Server.RunMode]
	if ok {
		//默认开启snappy
		if len(option.Server.Compress) <= 0 {
			option.Server.Compress = "snappy"
		}
	} else {
		panic("Server RunMode Conf Not Found!")
	}

	option.Clusters[option.Server.RunMode] = cluster
	return option
}
