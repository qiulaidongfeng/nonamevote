package run

import (
	"sync/atomic"
	"time"
)

// Ticker 间隔一定时间执行f
// 默认间隔1分钟执行1次
// 如果返回false,表示执行f没有变化
// 将按间隔10分钟，100分钟，1000分钟，1天顺序延迟间隔直至1天执行一次f
// 除非change被调用，将恢复间隔至1分钟
func Ticker(f func() (changed bool)) (change func()) {
	interval := 1 * time.Minute
	sig := make(chan struct{})
	send := atomic.Bool{}
	go func() {
		for {
			t := time.AfterFunc(interval, func() {
				c := f()
				if !c {
					interval = min(interval*10, 24*time.Hour)
					send.Store(true)
				} else {
					interval = 1 * time.Minute
				}
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
