package core

import (
	"fmt"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"

	"sync"
	"sync/atomic"
	"time"

	"runtime"
)

const (
	MAX_ROTATE_SIZE = 10
	MOA_STAT_LOG    = "moa-stat"
)

type MoaInfo struct {
	Recv            int64 `json:"received_Count"`
	Proc            int64 `json:"processed_Count"`
	Error           int64 `json:"error_Count"`
	Timeout         int64 `json:"error_timeout_Count"`
	MoaInvokePool   int64 `json:"invoke_goroutine"` //moa的调用Pool
	ConnectionCount int64 `json:"connection_count"`
	TotalGoroutine  int64 `json:"total_goroutine"`
}

//
type MoaStat struct {
	preMoaInfo  *MoaInfo
	currMoaInfo *MoaInfo
	invokePool  *turbo.GPool
	RotateSize  int32
	network     func() turbo.NetworkStat
	MoaTicker   *time.Ticker
	lock        sync.RWMutex
	monitor     func(serviceUri, host string, moainfo MoaInfo)
	hostname    string
	serviceUri  string
}

type MoaLog interface {
	StartLog()
	Destroy()
}

func NewMoaStat(hostname, serviceUri string,
	invokePool *turbo.GPool,
	moniotr func(serviceUri, host string, moainfo MoaInfo), network func() turbo.NetworkStat) *MoaStat {
	moaStat := &MoaStat{
		currMoaInfo: &MoaInfo{},
		invokePool:  invokePool,
		RotateSize:  0,
		network:     network,
		monitor:     moniotr,
		hostname:    hostname,
		serviceUri:  serviceUri}
	return moaStat
}

func (self *MoaStat) StartLog() {
	ticker := time.NewTicker(time.Second * 1)
	self.MoaTicker = ticker
	go func() {
		defer func() {
			if err := recover(); nil != err {
				log.ErrorLog("stderr", "time.ticker|Invoke|FAIL|%v", err)
				// 销毁定时器
				self.Destroy()
			}

		}()
		log.InfoLog(MOA_STAT_LOG, "RECV\tPROC\tERROR\tTIMEOUT\tGoroutine\tMoaQueue\tNetWork")
		for {
			<-ticker.C
			stat := self.network()
			network := fmt.Sprintf("R:%dKB/%d\tW:%dKB/%d\tGo:%d/%d\tCONN:%d", stat.ReadBytes/1024,
				stat.ReadCount,
				stat.WriteBytes/1024, stat.WriteCount, stat.DisPoolSize, stat.DisPoolCap, stat.Connections)
			if self.RotateSize == MAX_ROTATE_SIZE {
				log.InfoLog(MOA_STAT_LOG, "RECV\tPROC\tERROR\tTIMEOUT\tGoroutine\ttNetWork")
				log.InfoLog(MOA_STAT_LOG, "%d\t%d\t%d\t%d\t%d\t%d\t%s",
					self.currMoaInfo.Recv, self.currMoaInfo.Proc, self.currMoaInfo.Error,
					self.currMoaInfo.Timeout, self.currMoaInfo.MoaInvokePool, network)
				// self.RotateSize = 0
				atomic.StoreInt32(&self.RotateSize, 0)
			} else {
				log.InfoLog(MOA_STAT_LOG, "%d\t%d\t%d\t%d\t%d\t%s",
					self.currMoaInfo.Recv, self.currMoaInfo.Proc, self.currMoaInfo.Error,
					self.currMoaInfo.Timeout, self.currMoaInfo.MoaInvokePool, network)
				// self.RotateSize++
				atomic.AddInt32(&self.RotateSize, 1)
			}

			//send data
			self.currMoaInfo.ConnectionCount = int64(stat.Connections)
			self.currMoaInfo.MoaInvokePool = int64(stat.DisPoolSize)
			self.currMoaInfo.TotalGoroutine = int64(runtime.NumGoroutine())
			self.monitor(self.serviceUri, self.hostname, *self.currMoaInfo)
			self.reset()
		}
	}()
}

func (self *MoaStat) IncreaseRecv() {
	atomic.AddInt64(&self.currMoaInfo.Recv, 1)
}

func (self *MoaStat) IncreaseProc() {
	atomic.AddInt64(&self.currMoaInfo.Proc, 1)
}

func (self *MoaStat) IncreaseError() {
	atomic.AddInt64(&self.currMoaInfo.Error, 1)
}

func (self *MoaStat) IncreaseTimeout() {
	atomic.AddInt64(&self.currMoaInfo.Timeout, 1)
}

func (self *MoaStat) GetMoaInfo() MoaInfo {
	return *self.currMoaInfo
}

func (self *MoaStat) reset() {
	self.preMoaInfo = self.currMoaInfo
	self.currMoaInfo = &MoaInfo{}
}

func (self *MoaStat) Destroy() {
	self.MoaTicker.Stop()
}
