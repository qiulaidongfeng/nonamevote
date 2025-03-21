// Package run 实现动态定时运行事件
package run

import (
	"context"
	"sync/atomic"
	"time"
)

// 测试用
var stop, cancel = context.WithCancel(context.Background())

// Ticker 间隔一定时间执行f
// 默认间隔1分钟执行1次
// 然后间隔365天执行1次f
// 除非change被调用，将恢复间隔至1分钟
func Ticker(f func()) (change func()) {
	interval := atomic.Value{}
	interval.Store(1 * time.Minute)
	sig := make(chan struct{})
	send := atomic.Bool{}

	go func() {
		for {
			currentInterval := interval.Load().(time.Duration)
			t := time.NewTimer(currentInterval)
			select {
			case <-sig:
				interval.Store(1 * time.Minute)
			case <-t.C:
				send.Store(true)
				f()
				interval.Store(24 * time.Hour * 365)
			case <-stop.Done():
				return
			}
		}
	}()

	return func() {
		if send.CompareAndSwap(true, false) {
			sig <- struct{}{}
		}
	}
}
