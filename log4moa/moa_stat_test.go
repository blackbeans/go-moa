package log4moa

import (
	"testing"
	"time"
)

func TestPrint(t *testing.T) {
	moaStat := NewMoaStat(func() string {
		return ""
	})
	// start a timer to log
	moaStat.StartLog()
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Millisecond * 300)
			// 模拟调用
			moaStat.IncreaseRecv()
			moaStat.IncreaseProc()
			moaStat.IncreaseError()
		}
	}()
	// 暂停等待Moa-stat打印统计日志
	time.Sleep(time.Second * 7)
	// moaStat.Destory()
}
