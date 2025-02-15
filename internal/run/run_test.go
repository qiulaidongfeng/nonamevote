//go:build goexperiment.synctest

package run

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestRun(t *testing.T) {
	synctest.Run(func() {
		var i int
		changed := Ticker(func() {
			i++
		})
		test(t, &i, 1)
		changed()
		test(t, &i, 3)
		cancel()
	})
}

func test(t *testing.T, i *int, strat int) {
	// 测试是否1分钟后运行i++
	time.Sleep(1 * time.Minute)
	synctest.Wait()
	if *i != strat {
		t.Fatalf("got %d, want %d", *i, strat)
	}

	// 测试是否再1分钟后不运行i++
	time.Sleep(1 * time.Minute)
	synctest.Wait()
	if *i != strat {
		t.Fatalf("got %d, want %d", *i, strat)
	}

	// 测试是否运行i++后365天才再运行i++
	const day365 = 24 * time.Hour * 365
	time.Sleep(day365 - time.Minute)
	synctest.Wait()
	if *i != strat+1 {
		t.Fatalf("got %d, want %d", *i, strat+1)
	}
}
