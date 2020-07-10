package core

import (
	"fmt"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

// prometheus metrics
type MoaMetrics struct {
	RpcReceiveTotalCounter prometheus.Counter
	RpcProcessTotalCounter prometheus.Counter
	RpcErrorTotalCounter   prometheus.Counter
	RpcTimeoutTotalCounter prometheus.Counter
	InvokePoolMaxGauge     prometheus.Gauge
	InvokePoolInuseGauge   prometheus.Gauge
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
	MoaMetrics  *MoaMetrics
}

type MoaLog interface {
	StartLog()
	Destroy()
}

func NewMoaStat(hostname, serviceUri string,
	invokePool *turbo.GPool,
	moniotr func(serviceUri, host string, moainfo MoaInfo), network func() turbo.NetworkStat) *MoaStat {

	receiveTotalCounter := promauto.NewCounter(prometheus.CounterOpts{
		Name: "moa_server_rpc_receive_total",
		Help: "The total number of received rpc call of a service's moa server",
	})
	processTotalCounter := promauto.NewCounter(prometheus.CounterOpts{
		Name: "moa_server_rpc_process_total",
		Help: "The total number of processed rpc call of a service's moa server",
	})
	errorTotalCounter := promauto.NewCounter(prometheus.CounterOpts{
		Name: "moa_server_rpc_error_total",
		Help: "The total number of error rpc call of a service's moa server",
	})
	timeoutTotalCounter := promauto.NewCounter(prometheus.CounterOpts{
		Name: "moa_server_rpc_timeout_total",
		Help: "The total number of timeout rpc call of a service's moa server",
	})
	poolMaxGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "moa_server_invoke_max_pool",
		Help: "The max cap of invoke pool",
	})
	poolInuseGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "moa_server_invoke_inuse_pool",
		Help: "The current inuse invoke pool",
	})

	moaStat := &MoaStat{
		currMoaInfo: &MoaStatistic{
			Recv:    &turbo.Flow{},
			Proc:    &turbo.Flow{},
			Error:   &turbo.Flow{},
			Timeout: &turbo.Flow{},
		},
		MoaMetrics: &MoaMetrics{
			RpcReceiveTotalCounter: receiveTotalCounter,
			RpcProcessTotalCounter: processTotalCounter,
			RpcErrorTotalCounter:   errorTotalCounter,
			RpcTimeoutTotalCounter: timeoutTotalCounter,
			InvokePoolMaxGauge:     poolMaxGauge,
			InvokePoolInuseGauge:   poolInuseGauge,
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

			self.MoaMetrics.InvokePoolInuseGauge.Set(float64(size))
			self.MoaMetrics.InvokePoolMaxGauge.Set(float64(invokeCap))

			self.preMoaInfo = MoaInfo{
				Recv:           int64(self.currMoaInfo.Recv.Changes()),
				Proc:           int64(self.currMoaInfo.Proc.Changes()),
				Error:          int64(self.currMoaInfo.Error.Changes()),
				Timeout:        int64(self.currMoaInfo.Timeout.Changes()),
				MoaInvokePool:  int64(size),
				Connections:    int64(stat.Connections),
				TotalGoroutine: int64(runtime.NumGoroutine()),
			}

			network := fmt.Sprintf("R:%dKB/%d\tW:%dKB/%d\tGo:%d/%d\tCONN:%d", stat.ReadBytes/1024,
				stat.ReadCount,
				stat.WriteBytes/1024, stat.WriteCount, stat.DisPoolSize, stat.DisPoolCap, stat.Connections)

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
	self.MoaMetrics.RpcReceiveTotalCounter.Inc()
}

func (self *MoaStat) IncrProc() {
	self.currMoaInfo.Proc.Incr(1)
	self.MoaMetrics.RpcProcessTotalCounter.Inc()
}

func (self *MoaStat) IncrError() {
	self.currMoaInfo.Error.Incr(1)
	self.MoaMetrics.RpcErrorTotalCounter.Inc()
}

func (self *MoaStat) IncrTimeout() {
	self.currMoaInfo.Timeout.Incr(1)
	self.MoaMetrics.RpcTimeoutTotalCounter.Inc()
}

func (self *MoaStat) GetMoaInfo() MoaInfo {
	return self.preMoaInfo
}

func (self *MoaStat) Destroy() {
	self.MoaTicker.Stop()
}
