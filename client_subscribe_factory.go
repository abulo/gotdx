// Package gotdx 通用行情订阅工厂：对任意数据源建立"轮询 diff 推送"订阅。
//
// 工厂与具体的查询方法解耦：调用方提供拉取函数（`Fetch`）与可选的变化判定
// （`Changed`），工厂负责按间隔轮询、快照对比与变化回调，因此 K 线、分时、逐笔、
// 指数、扩展市场等所有查询能力都可以用同一套订阅语义接入，无需为每种数据单独
// 实现推送循环。
package gotdx

import (
	"errors"
	"reflect"
	"sync"
	"time"
)

// defaultSubscribeInterval 为未指定间隔时的默认轮询间隔：行情快照秒级刷新即可
// 满足大多数订阅场景，过短间隔会增加对服务器的轮询压力。
const defaultSubscribeInterval = 3 * time.Second

// Subscriber 是通用行情订阅工厂：绑定一个客户端连接，通过 `Subscribe` 对任意
// 数据源建立订阅；客户端 `Disconnect` 时工厂内全部订阅自动停止。
type Subscriber struct {
	client *Client // 绑定的客户端，其 `done` 通道作为全局停止信号
}

// Subscriber 返回绑定当前客户端的通用订阅工厂：通过工厂可对报价、K 线、分时、
// 逐笔、指数、扩展市场等任意数据源建立订阅（见 `Subscriber.Subscribe`），客户端
// `Disconnect` 时工厂内全部订阅自动停止。
func (client *Client) Subscriber() *Subscriber {
	return &Subscriber{client: client}
}

// SubscribeSource 描述一个可订阅的数据源：`Fetch` 拉取最新快照，`Changed` 判断
// 两次快照是否发生变化（省略时默认做深比较，即任何字段变化都视为变化）。
type SubscribeSource[T any] struct {
	Fetch   func() (T, error)     // 拉取最新快照，返回错误时跳过本轮轮询
	Changed func(old, new T) bool // 两次快照是否不同；`nil` 时使用深比较
}

// Subscribe 对一个数据源建立订阅：注册后立即拉取一次并推送建立基线，此后按
// `interval` 轮询拉取，快照发生变化时回调 `handler`。
//
// 参数 `source` 描述数据源（`Fetch` 必填）；`interval` 为轮询间隔，非正数时使用
// 默认 3 秒；`handler` 接收变化后的最新快照，返回 `false` 即取消本次订阅。返回
// 取消函数，调用后停止该订阅的轮询与推送。拉取失败（网络波动）自动跳过本轮
// 并下轮重试，不中断订阅。
func (f *Subscriber) Subscribe[T any](source SubscribeSource[T], interval time.Duration, handler func(T) bool) (cancel func(), err error) {
	if source.Fetch == nil {
		return nil, errors.New("nil subscribe fetch")
	}
	if handler == nil {
		return nil, errors.New("nil subscribe handler")
	}
	if interval <= 0 {
		interval = defaultSubscribeInterval
	}
	if source.Changed == nil {
		source.Changed = func(old, new T) bool { return !reflect.DeepEqual(old, new) }
	}

	stop := make(chan struct{})
	var once sync.Once
	cancel = func() { once.Do(func() { close(stop) }) }

	go f.subscribeLoop(source, interval, handler, stop)
	return cancel, nil
}

// subscribeLoop 单个订阅的轮询循环：持有该订阅独立的快照状态（`last`/`hasLast`），
// 按 `interval` 周期拉取并 diff 推送；取消、客户端关闭或处理函数返回 `false` 时退出。
func (f *Subscriber) subscribeLoop[T any](source SubscribeSource[T], interval time.Duration, handler func(T) bool, stop <-chan struct{}) {
	var last T
	hasLast := false

	// 首次立即拉取建立基线：失败不阻断循环，下轮轮询重试。
	if v, err := source.Fetch(); err == nil {
		last, hasLast = v, true
		if !handler(v) {
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-f.client.done:
			return
		case <-ticker.C:
			v, err := source.Fetch()
			if err != nil {
				continue // 网络波动，跳过本轮
			}
			if hasLast && !source.Changed(last, v) {
				continue // 快照无变化，不推送
			}
			last, hasLast = v, true
			if !handler(v) {
				return
			}
		}
	}
}
