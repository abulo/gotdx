package gotdx

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/bensema/gotdx/proto"
)

// TestSubscriberPushesEveryChange 验证假数据源每次快照变化都推送、取消后停止：
// `Fetch` 返回递增计数，默认深比较判定每次变化，推送次数应等于轮询次数。
func TestSubscriberPushesEveryChange(t *testing.T) {
	addr, stop := fakeTDXServer(t, 0)
	defer stop()

	c := New(WithTCPAddress(addr), WithTimeoutSec(5), WithTCPAddressPool(), WithPoolSize(1))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer c.Disconnect()

	var seed atomic.Int64
	var pushes atomic.Int64
	cancel, err := c.Subscriber().Subscribe(
		SubscribeSource[int64]{
			Fetch: func() (int64, error) { return seed.Add(1), nil },
		},
		100*time.Millisecond,
		func(v int64) bool {
			pushes.Add(1)
			return true
		})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// 基线 + 至少两次轮询推送。
	deadline := time.Now().Add(2 * time.Second)
	for pushes.Load() < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("expected at least 3 pushes, got %d", pushes.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	time.Sleep(300 * time.Millisecond)
	after := pushes.Load()
	time.Sleep(200 * time.Millisecond)
	if got := pushes.Load(); got != after {
		t.Fatalf("pushes must stop after cancel: before=%d after=%d", after, got)
	}
}

// TestSubscriberSkipsUnchanged 验证快照不变时不推送：固定值数据源只推送基线一次。
func TestSubscriberSkipsUnchanged(t *testing.T) {
	addr, stop := fakeTDXServer(t, 0)
	defer stop()

	c := New(WithTCPAddress(addr), WithTimeoutSec(5), WithTCPAddressPool(), WithPoolSize(1))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer c.Disconnect()

	var pushes atomic.Int64
	cancel, err := c.Subscriber().Subscribe(
		SubscribeSource[string]{
			Fetch: func() (string, error) { return "fixed", nil },
		},
		100*time.Millisecond,
		func(v string) bool {
			pushes.Add(1)
			return true
		})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer cancel()

	deadline := time.Now().Add(2 * time.Second)
	for pushes.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("baseline push never happened")
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(350 * time.Millisecond) // 覆盖至少 3 个轮询周期
	if got := pushes.Load(); got != 1 {
		t.Fatalf("expected only baseline push, got %d", got)
	}
}

// TestSubscriberCustomChanged 验证自定义变化判定生效：变化幅度不足阈值时不推送。
func TestSubscriberCustomChanged(t *testing.T) {
	addr, stop := fakeTDXServer(t, 0)
	defer stop()

	c := New(WithTCPAddress(addr), WithTimeoutSec(5), WithTCPAddressPool(), WithPoolSize(1))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer c.Disconnect()

	var seed atomic.Int64 // 每次 +10，但阈值 50：每 5 次拉取算一次"变化"
	var pushes atomic.Int64
	cancel, err := c.Subscriber().Subscribe(
		SubscribeSource[int64]{
			Fetch: func() (int64, error) { return seed.Add(10), nil },
			Changed: func(old, new int64) bool {
				return new-old >= 50
			},
		},
		50*time.Millisecond,
		func(v int64) bool {
			pushes.Add(1)
			return true
		})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer cancel()

	// 基线一次；随后每 5 次拉取（250ms）推一次，500ms 内应只多推约 2 次。
	deadline := time.Now().Add(2 * time.Second)
	for pushes.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("baseline push never happened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	got := pushes.Load()
	if got < 2 || got > 4 {
		t.Fatalf("expected ~3 pushes (baseline + 2 threshold hits) in 500ms, got %d", got)
	}
}

// TestSubscriberQuotesViaFactory 验证工厂可直接订阅真实报价数据源（复用
// `GetSecurityQuotes` 拉取链路与 fake 服务器的固定报价响应）。
func TestSubscriberQuotesViaFactory(t *testing.T) {
	addr, stop := fakeTDXServer(t, 0)
	defer stop()

	c := New(WithTCPAddress(addr), WithTimeoutSec(5), WithTCPAddressPool(), WithPoolSize(1))
	if _, err := c.Connect(); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer c.Disconnect()

	var pushes atomic.Int64
	cancel, err := c.Subscriber().Subscribe(
		SubscribeSource[*proto.GetSecurityQuotesReply]{
			Fetch: func() (*proto.GetSecurityQuotesReply, error) {
				return c.GetSecurityQuotes([]uint8{0}, []string{"000000"})
			},
		},
		200*time.Millisecond,
		func(q *proto.GetSecurityQuotesReply) bool {
			pushes.Add(1)
			if q.Count != 1 || len(q.List) != 1 {
				t.Fatalf("unexpected quotes reply: count=%d list=%d", q.Count, len(q.List))
			}
			return true
		})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer cancel()

	deadline := time.Now().Add(2 * time.Second)
	for pushes.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("baseline push never happened")
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(400 * time.Millisecond)
	if got := pushes.Load(); got != 1 {
		t.Fatalf("fixed quotes must push once (baseline), got %d", got)
	}
}
