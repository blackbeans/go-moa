package core

import (
	"fmt"
	"github.com/blackbeans/turbo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
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
	// rpc请求数量
	RpcReceiveTotalCounter prometheus.Counter
	RpcProcessTotalCounter prometheus.Counter
	RpcErrorTotalCounter   prometheus.Counter
	RpcTimeoutTotalCounter prometheus.Counter
	// rpc请求耗时
	RpcInvokeDurationSummary *prometheus.SummaryVec
	// rpc gopool用量
	InvokePoolMaxGauge   prometheus.Gauge
	InvokePoolInuseGauge prometheus.Gauge

	cllectors []prometheus.Collector
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

	// 初始化指标
	// rpc请求数量
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
	// rpc 请求耗时
	invokeDurationSummary := promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name:       "moa_server_rpc_invoke_duration_seconds",
		Help:       "Duration of rpc invoke cost",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"method"})
	// rpc gopool 用量
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
			RpcReceiveTotalCounter:   receiveTotalCounter,
			RpcProcessTotalCounter:   processTotalCounter,
			RpcErrorTotalCounter:     errorTotalCounter,
			RpcTimeoutTotalCounter:   timeoutTotalCounter,
			RpcInvokeDurationSummary: invokeDurationSummary,
			InvokePoolMaxGauge:       poolMaxGauge,
			InvokePoolInuseGauge:     poolInuseGauge,
			cllectors: []prometheus.Collector{
				receiveTotalCounter,
				processTotalCounter,
				errorTotalCounter,
				timeoutTotalCounter,
				invokeDurationSummary,
				poolMaxGauge,
				poolInuseGauge,
			},
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
				log.Errorf("time.ticker|Invoke|FAIL|%v", err)
				// 销毁定时器
				self.Destroy()
			}

		}()
		log.Infof("RECV PROC ERROR TIMEOUT Goroutine NetWork")
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

			network := fmt.Sprintf("R:%dKB/%d W:%dKB/%d Go:%d/%d CONN:%d", stat.ReadBytes/1024,
				stat.ReadCount,
				stat.WriteBytes/1024, stat.WriteCount, stat.DisPoolSize, stat.DisPoolCap, stat.Connections)

			if self.RotateSize == MAX_ROTATE_SIZE {
				log.Infof("REV PROC ERROR TIMEOUT Goroutine NetWork")

				log.Infof("%d %d %d %d %d/%d %s",
					self.preMoaInfo.Recv,
					self.preMoaInfo.Proc,
					self.preMoaInfo.Error,
					self.preMoaInfo.Timeout,
					size, invokeCap, network)
				// self.RotateSize = 0
				atomic.StoreInt32(&self.RotateSize, 0)
			} else {
				log.Infof("%d %d %d %d %d/%d %s",
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
	if nil != self.MoaTicker {
		self.MoaTicker.Stop()
	}

	if nil != self.MoaMetrics {
		for _, c := range self.MoaMetrics.cllectors {
			prometheus.DefaultRegisterer.Unregister(c)
		}
	}
}
