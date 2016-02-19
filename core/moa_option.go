package core

import (
	"errors"
	"flag"
	log "github.com/blackbeans/log4go"
	"github.com/naoina/toml"
	"io/ioutil"
	"os"
	"time"
)

type HostPort struct {
	Hosts string
}

//配置信息
type Option struct {
	Env struct {
		Name        string
		RunMode     string
		BindAddress string
	}

	//使用的环境
	Momokeeper map[string]HostPort //momokeeper的配置
	Clusters   map[string]Cluster  //各集群的配置
}

//----------------------------------------
//Cluster配置
type Cluster struct {
	Env               string //当前环境使用的是dev还是online
	ProcessTimeout    int    //处理超时 5 s单位
	MaxDispatcherSize int    //=8000//最大分发处理协程数
	ReadBufferSize    int    //=16 * 1024 //读取缓冲大小
	WriteBufferSize   int    //=16 * 1024 //写入缓冲大小
	WriteChannelSize  int    //=1000 //写异步channel长度
	ReadChannelSize   int    //=1000 //读异步channel长度
}

//---------最终需要的Option
type MOAOption struct {
	name              string
	mkhosts           string
	hostport          string
	processTimeout    time.Duration
	maxDispatcherSize int           //=8000//最大分发处理协程数
	readBufferSize    int           //=16 * 1024 //读取缓冲大小
	writeBufferSize   int           //=16 * 1024 //写入缓冲大小
	writeChannelSize  int           //=1000 //写异步channel长度
	readChannelSize   int           //=1000 //读异步channel长度
	idleDuration      time.Duration //=60s //连接空闲时间
}

func LoadConfiruation(path string) (*MOAOption, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buff, rerr := ioutil.ReadAll(f)
	if nil != rerr {
		return nil, rerr
	}
	log.DebugLog("application", "LoadConfiruation|Parse|toml:%s", string(buff))
	//读取配置
	var option Option
	err = toml.Unmarshal(buff, &option)
	if nil != err {
		log.ErrorLog("application", "LoadConfiruation|Parse|FAIL|%s", err)
		return nil, err
	}

	//若没有使用设置启动模式默认dev
	runMode := flag.String("runMode", "dev", "-runMode=dev/online")
	flag.Parse()
	if nil == runMode || *runMode != "online" {
		*runMode = "dev"
	}
	option.Env.RunMode = *runMode

	cluster, ok := option.Clusters[option.Env.RunMode]
	if !ok {
		return nil, errors.New("no cluster config for " + option.Env.RunMode)
	}

	zk, exist := option.Momokeeper[option.Env.RunMode]
	if !exist {
		return nil, errors.New("no zk  for " + option.Env.RunMode + ":" + cluster.Env)
	}

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

	//拼装为可用的MOA参数
	mop := &MOAOption{}
	mop.name = option.Env.Name
	mop.hostport = option.Env.BindAddress
	mop.mkhosts = zk.Hosts
	mop.processTimeout = time.Duration(int64(cluster.ProcessTimeout) * int64(time.Second))
	mop.maxDispatcherSize = cluster.MaxDispatcherSize //最大分发处理协程数
	mop.readBufferSize = cluster.ReadBufferSize       //读取缓冲大小
	mop.writeBufferSize = cluster.WriteBufferSize     //写入缓冲大小
	mop.writeChannelSize = cluster.WriteChannelSize   //写异步channel长度
	mop.readChannelSize = cluster.ReadChannelSize     //读异步channel长度
	mop.idleDuration = 60 * time.Second
	return mop, nil

}
