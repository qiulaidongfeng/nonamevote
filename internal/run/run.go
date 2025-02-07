package run

import (
	"sync/atomic"
	"time"
)

// Ticker 间隔一定时间执行f
// 默认间隔1分钟执行1次
// 然后间隔365天执行1次f
// 除非change被调用，将恢复间隔至1分钟
func Ticker(f func()) (change func()) {
	interval := 1 * time.Minute
	sig := make(chan struct{})
	send := atomic.Bool{}
	go func() {
		for {
			t := time.AfterFunc(interval, func() {
				send.Store(true)
				f()
				interval = 24 * time.Hour * 365
			})
			select {
			case <-sig:
				t.Stop()
				interval = 1 * time.Minute
			case <-t.C:
			}
		}
	}()
	return func() {
		//使用一个原子变量，这样大多数时候，这里只是一个cas(性能高)，而不是一个channal send(性能差).
		if send.CompareAndSwap(true, false) {
			sig <- struct{}{}
		}
	}
}
