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

type Method struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type InvokePerClient struct {
	Client      string   `json:"client"`
	ServiceName string   `json:"service_name"`
	Methods     []Method `json:"methods"`
}

type MoaInfo struct {
	Recv           int64 `json:"recv"`
	Proc           int64 `json:"proc"`
	Error          int64 `json:"error"`
	Timeout        int64 `json:"timeout"`
	MoaInvokePool  int64 `json:"invoke_gos"` //moa的调用Pool
	Connections    int64 `json:"conns"`
	TotalGoroutine int64 `json:"total_gos"`
}

type MoaStatistic struct {
	Recv    *turbo.Flow
	Proc    *turbo.Flow
	Error   *turbo.Flow
	Timeout *turbo.Flow
}

//
type MoaStat struct {
	preMoaInfo  MoaInfo
	currMoaInfo *MoaStatistic
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
		currMoaInfo: &MoaStatistic{
			Recv:    &turbo.Flow{},
			Proc:    &turbo.Flow{},
			Error:   &turbo.Flow{},
			Timeout: &turbo.Flow{},
		},
		invokePool: invokePool,
		RotateSize: 0,
		network:    network,
		monitor:    moniotr,
		hostname:   hostname,
		serviceUri: serviceUri}
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
		log.InfoLog(MOA_STAT_LOG, "RECV\tPROC\tERROR\tTIMEOUT\tGoroutine\tNetWork")
		for {
			<-ticker.C
			stat := self.network()
			size, invokeCap := self.invokePool.Monitor()
			self.preMoaInfo = MoaInfo{
				Recv:           int64(self.currMoaInfo.Recv.Changes()),
				Proc:           int64(self.currMoaInfo.Proc.Changes()),
				Error:          int64(self.currMoaInfo.Error.Changes()),
				Timeout:        int64(self.currMoaInfo.Timeout.Changes()),
				MoaInvokePool:  int64(size),
				Connections:    int64(stat.Connections),
				TotalGoroutine: int64(runtime.NumGoroutine()),
			}

			network := fmt.Sprintf("R:%dKB/%d\tW:%dKB/%d\tGo:%d/%d\tCONN:%d", stat.ReadBytes,
				stat.ReadCount,
				stat.WriteBytes, stat.WriteCount, stat.DisPoolSize, stat.DisPoolCap, stat.Connections)

			if self.RotateSize == MAX_ROTATE_SIZE {
				log.InfoLog(MOA_STAT_LOG, "RECV\tPROC\tERROR\tTIMEOUT\tGoroutine\tNetWork")

				log.InfoLog(MOA_STAT_LOG, "%d\t%d\t%d\t%d\t%d/%d\t%s",
					self.preMoaInfo.Recv,
					self.preMoaInfo.Proc,
					self.preMoaInfo.Error,
					self.preMoaInfo.Timeout,
					size, invokeCap, network)
				// self.RotateSize = 0
				atomic.StoreInt32(&self.RotateSize, 0)
			} else {
				log.InfoLog(MOA_STAT_LOG, "%d\t%d\t%d\t%d\t%d/%d\t%s",
					self.preMoaInfo.Recv,
					self.preMoaInfo.Proc,
					self.preMoaInfo.Error,
					self.preMoaInfo.Timeout,
					size, invokeCap, network)
				// self.RotateSize++
				atomic.AddInt32(&self.RotateSize, 1)
			}
			self.monitor(self.serviceUri, self.hostname, self.preMoaInfo)
		}
	}()
}

func (self *MoaStat) IncrRecv() {
	self.currMoaInfo.Recv.Incr(1)
}

func (self *MoaStat) IncrProc() {
	self.currMoaInfo.Proc.Incr(1)
}

func (self *MoaStat) IncrError() {
	self.currMoaInfo.Error.Incr(1)
}

func (self *MoaStat) IncrTimeout() {
	self.currMoaInfo.Timeout.Incr(1)
}

func (self *MoaStat) GetMoaInfo() MoaInfo {
	return self.preMoaInfo
}

func (self *MoaStat) Destroy() {
	self.MoaTicker.Stop()
}
