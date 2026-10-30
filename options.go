package gotdx

import (
	"path/filepath"
	"runtime"
)

const (
	_defaultTCPAddress      = "119.147.212.81:7709"
	_defaultExTCPAddress    = "112.74.214.43:7727"
	_defaultMacTCPAddress   = "121.36.248.138:7709"
	_defaultMacExTCPAddress = "116.205.135.205:7727"
	_defaultRetryTimes      = 3
	_defaultTimeoutSec      = 8
)

// Options 保存客户端创建时使用的主站、扩展站、MAC 站地址与地址池、重试次数、超时等全部配置。
//
// 该结构体通常不直接构造：`New` 会先应用内置默认配置，再按传入顺序应用各 `Option` 覆盖
// 对应字段，后传入的选项优先生效。各字段的零值表示"沿用对应默认值"，字段级并发读写
// 应在构造阶段完成，连接建立后不应再修改。
type Options struct {
	TCPAddress          string   // 主行情服务器地址
	TCPAddressPool      []string // 主行情服务器地址池
	ExTCPAddress        string   // 扩展行情服务器地址
	ExTCPAddressPool    []string // 扩展行情服务器地址池
	MacTCPAddress       string   // MAC 主行情服务器地址
	MacTCPAddressPool   []string // MAC 主行情服务器地址池
	MacExTCPAddress     string   // MAC 扩展行情服务器地址
	MacExTCPAddressPool []string // MAC 扩展行情服务器地址池
	AutoSelectFastest   bool     // 连接前先对地址池做 TCP 测速并优先尝试低延迟节点
	MaxRetryTimes       int      // 重试次数
	TimeoutSec          int      // 连接和读写超时时间，单位秒
	FuturesCalendarPath string   // 期货交易日历 SQLite 路径；为空时不转换 MAC 期货自然时间
}

// defaultOptions 构造内置默认配置：各站点使用内置主机列表与兜底地址，并设置默认重试次数、超时与日历路径。
func defaultOptions() *Options {
	mainAddress, mainPool := defaultAddressAndPool(MainHostAddresses(), _defaultTCPAddress)
	exAddress, exPool := defaultAddressAndPool(ExHostAddresses(), _defaultExTCPAddress)
	macAddress, macPool := defaultAddressAndPool(MACHostAddresses(), _defaultMacTCPAddress)
	macExAddress, macExPool := defaultAddressAndPool(MACExHostAddresses(), _defaultMacExTCPAddress)

	return &Options{
		TCPAddress:          mainAddress,
		TCPAddressPool:      mainPool,
		ExTCPAddress:        exAddress,
		ExTCPAddressPool:    exPool,
		MacTCPAddress:       macAddress,
		MacTCPAddressPool:   macPool,
		MacExTCPAddress:     macExAddress,
		MacExTCPAddressPool: macExPool,
		MaxRetryTimes:       _defaultRetryTimes,
		TimeoutSec:          _defaultTimeoutSec,
		FuturesCalendarPath: defaultFuturesCalendarPath(),
	}
}

// defaultFuturesCalendarPath 返回随 `gotdx` 源码发布的默认期货交易日历路径。
func defaultFuturesCalendarPath() string {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("data", "futures-trade-calendar.db")
	}
	return filepath.Join(filepath.Dir(sourceFile), "data", "futures-trade-calendar.db")
}

// applyOptions 先填充默认配置，再按传入顺序依次执行 `Option` 覆盖对应字段；后传入的选项优先生效。
func applyOptions(opts ...Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Option 是用于修改 `Options` 的函数式选项，返回闭包在合并阶段按顺序应用到配置上。
type Option func(options *Options)

// WithTCPAddress 设置主行情服务器地址，格式为 `host:port`。
func WithTCPAddress(tcpAddress string) Option {
	return func(o *Options) {
		o.TCPAddress = tcpAddress
	}
}

// WithTCPAddressPool 设置主行情服务器地址池，连接时按顺序或测速结果依次尝试。
func WithTCPAddressPool(ips ...string) Option {
	return func(o *Options) {
		o.TCPAddressPool = ips
	}
}

// WithExTCPAddress 设置扩展行情服务器地址，格式为 `host:port`。
func WithExTCPAddress(tcpAddress string) Option {
	return func(o *Options) {
		o.ExTCPAddress = tcpAddress
	}
}

// WithExTCPAddressPool 设置扩展行情服务器地址池，连接时按顺序或测速结果依次尝试。
func WithExTCPAddressPool(ips ...string) Option {
	return func(o *Options) {
		o.ExTCPAddressPool = ips
	}
}

// WithMacTCPAddress 设置 MAC 主行情服务器地址，格式为 `host:port`。
func WithMacTCPAddress(tcpAddress string) Option {
	return func(o *Options) {
		o.MacTCPAddress = tcpAddress
	}
}

// WithMacTCPAddressPool 设置 MAC 主行情服务器地址池，连接时按顺序或测速结果依次尝试。
func WithMacTCPAddressPool(ips ...string) Option {
	return func(o *Options) {
		o.MacTCPAddressPool = ips
	}
}

// WithMacExTCPAddress 设置 MAC 扩展行情服务器地址，格式为 `host:port`。
func WithMacExTCPAddress(tcpAddress string) Option {
	return func(o *Options) {
		o.MacExTCPAddress = tcpAddress
	}
}

// WithMacExTCPAddressPool 设置 MAC 扩展行情服务器地址池，连接时按顺序或测速结果依次尝试。
func WithMacExTCPAddressPool(ips ...string) Option {
	return func(o *Options) {
		o.MacExTCPAddressPool = ips
	}
}

// WithTimeoutSec 设置连接与读写超时时间，单位秒；传入非正数时保持默认超时。
func WithTimeoutSec(timeoutSec int) Option {
	return func(o *Options) {
		if timeoutSec > 0 {
			o.TimeoutSec = timeoutSec
		}
	}
}

// WithFuturesCalendarPath 设置 MAC 期货 K 线自然时间转换使用的 `SQLite` 交易日历路径。
func WithFuturesCalendarPath(path string) Option {
	return func(o *Options) {
		o.FuturesCalendarPath = path
	}
}

// WithAutoSelectFastest 启用或关闭连接前的地址池 TCP 测速排序，启用后优先连接低延迟节点。
func WithAutoSelectFastest(enabled bool) Option {
	return func(o *Options) {
		o.AutoSelectFastest = enabled
	}
}
