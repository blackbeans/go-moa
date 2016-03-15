package log4moa

import (
	"fmt"
	log "github.com/blackbeans/log4go"
	"github.com/go-errors/errors"
	"sync/atomic"
	"time"
)

const (
	MAX_ROTATE_SIZE = 10
	MOA_STAT_LOG    = "moa-stat"
)

//
type MoaStat struct {
	Recv       int64
	Proc       int64
	Error      int64
	RotateSize int32
	network    func() string
	MoaTicker  *time.Ticker
}

type MoaLog interface {
	StartLog()
	Destory()
}

func NewMoaStat(network func() string) *MoaStat {
	moaStat := &MoaStat{
		Recv:       0,
		Proc:       0,
		Error:      0,
		RotateSize: 0,
		network:    network}
	return moaStat
}

func (self *MoaStat) StartLog() {
	ticker := time.NewTicker(time.Second * 1)
	self.MoaTicker = ticker
	go func() {
		defer func() {
			if err := recover(); nil != err {
				var e error
				er, ok := err.(*errors.Error)
				if ok {
					stack := er.ErrorStack()
					e = errors.New(stack)
				} else {
					e = errors.New(fmt.Sprintf("time.ticker Call Err %s", err))
				}

				log.ErrorLog("stderr", "time.ticker|Invoke|FAIL|%s", e)
				// 销毁定时器
				self.Destory()
			}

		}()
		log.InfoLog(MOA_STAT_LOG, "RECV\tPROC\tERROR\tNetWork")
		for {
			<-ticker.C
			if self.RotateSize == MAX_ROTATE_SIZE {
				log.InfoLog(MOA_STAT_LOG, "RECV\tPROC\tERROR\tNetWork")
				log.InfoLog(MOA_STAT_LOG, "%d\t%d\t%d\t%s",
					self.Recv, self.Proc, self.Error, self.network())
				// self.RotateSize = 0
				atomic.StoreInt32(&self.RotateSize, 0)
			} else {
				log.InfoLog(MOA_STAT_LOG, "%d\t%d\t%d\t%s",
					self.Recv, self.Proc, self.Error, self.network())
				// self.RotateSize++
				atomic.AddInt32(&self.RotateSize, 1)
			}
			self.clear()
		}
	}()
}

func (self *MoaStat) IncreaseRecv() {
	atomic.AddInt64(&self.Recv, 1)
}

func (self *MoaStat) IncreaseProc() {
	atomic.AddInt64(&self.Proc, 1)
}

func (self *MoaStat) IncreaseError() {
	atomic.AddInt64(&self.Error, 1)
}

func (self *MoaStat) clear() {
	atomic.StoreInt64(&self.Recv, 0)
	atomic.StoreInt64(&self.Proc, 0)
	atomic.StoreInt64(&self.Error, 0)

}

func (self *MoaStat) Destory() {
	self.MoaTicker.Stop()
}
