// Package exampleutil 提供 `examples` 目录下各示例共用的客户端构造、内置主机地址与小工具函数，
// 屏蔽主站、扩展站、MAC 主站、MAC 扩展站之间的地址配置差异，使示例代码聚焦于协议调用本身。
package exampleutil

import "github.com/bensema/gotdx"

// NewMainClient 使用内置主行情服务器地址构造主站客户端：首条地址作为主地址，其余进入候选地址池；
// 构造时不拨号，连接在首次请求时按需建立。调用方用完应调用 `Disconnect` 释放连接。
func NewMainClient() *gotdx.Client {
	primary, pool := splitHosts(gotdx.MainHostAddresses())
	return gotdx.New(
		gotdx.WithTCPAddress(primary),
		gotdx.WithTCPAddressPool(pool...),
	)
}

// NewMainClientForHost 使用指定的主站 `host:port` 地址构造主站客户端，拨号超时固定为 6 秒；
// 适用于示例需要单独指定探测到的可用主机的场景。
func NewMainClientForHost(host string) *gotdx.Client {
	return gotdx.New(
		gotdx.WithTCPAddress(host),
		gotdx.WithTimeoutSec(6),
	)
}

// NewExClient 使用内置扩展行情服务器地址构造扩展站客户端，超时固定为 6 秒；
// 首条地址作为主地址，其余进入候选地址池，连接在首次请求时按需建立。
func NewExClient() *gotdx.Client {
	primary, pool := splitHosts(gotdx.ExHostAddresses())
	return gotdx.New(
		gotdx.WithExTCPAddress(primary),
		gotdx.WithExTCPAddressPool(pool...),
		gotdx.WithTimeoutSec(6),
	)
}

// NewExClientForHost 使用指定的扩展站 `host:port` 地址构造扩展站客户端，拨号超时固定为 6 秒。
func NewExClientForHost(host string) *gotdx.Client {
	return gotdx.New(
		gotdx.WithExTCPAddress(host),
		gotdx.WithTimeoutSec(6),
	)
}

// NewUnifiedClient 构造同时配置主站与扩展站两套候选地址的统一客户端，超时固定为 6 秒；
// 返回的客户端可分别调用主站与扩展站接口。
func NewUnifiedClient() *gotdx.Client {
	mainPrimary, mainPool := splitHosts(gotdx.MainHostAddresses())
	exPrimary, exPool := splitHosts(gotdx.ExHostAddresses())
	return gotdx.New(
		gotdx.WithTCPAddress(mainPrimary),
		gotdx.WithTCPAddressPool(mainPool...),
		gotdx.WithExTCPAddress(exPrimary),
		gotdx.WithExTCPAddressPool(exPool...),
		gotdx.WithTimeoutSec(6),
	)
}

// NewMACClient 使用内置 MAC 主站地址构造 MAC 客户端，超时固定为 6 秒；
// 首条地址作为主地址，其余进入候选地址池，连接在首次请求时按需建立。
func NewMACClient() *gotdx.Client {
	primary, pool := splitHosts(gotdx.MACHostAddresses())
	return gotdx.NewMAC(
		gotdx.WithMacTCPAddress(primary),
		gotdx.WithMacTCPAddressPool(pool...),
		gotdx.WithTimeoutSec(6),
	)
}

// NewMACExClient 使用内置 MAC 扩展站地址构造 MAC 扩展客户端，超时固定为 6 秒；
// 首条地址作为主地址，其余进入候选地址池，连接在首次请求时按需建立。
func NewMACExClient() *gotdx.Client {
	primary, pool := splitHosts(gotdx.MACExHostAddresses())
	return gotdx.NewMACEx(
		gotdx.WithMacExTCPAddress(primary),
		gotdx.WithMacExTCPAddressPool(pool...),
		gotdx.WithTimeoutSec(6),
	)
}

// MainHosts 返回内置主行情服务器的 `host:port` 地址列表副本。
func MainHosts() []string {
	return gotdx.MainHostAddresses()
}

// ExHosts 返回内置扩展行情服务器的 `host:port` 地址列表副本。
func ExHosts() []string {
	return gotdx.ExHostAddresses()
}

// MACHosts 返回内置 MAC 主行情服务器的 `host:port` 地址列表副本。
func MACHosts() []string {
	return gotdx.MACHostAddresses()
}

// MACExHosts 返回内置 MAC 扩展行情服务器的 `host:port` 地址列表副本。
func MACExHosts() []string {
	return gotdx.MACExHostAddresses()
}

// splitHosts 把地址列表拆成主地址与候选池：空列表返回空主地址与空池；
// 候选池返回切片副本，避免调用方改动影响内置地址列表。
func splitHosts(hosts []string) (string, []string) {
	if len(hosts) == 0 {
		return "", nil
	}
	return hosts[0], append([]string(nil), hosts[1:]...)
}

// Min 返回两个整数中较小的一个，供示例在截取有限条数时使用。
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PreviewHex 在 `value` 长度不超过 `limit` 时原样返回，否则截取前 `limit` 个字符，
// 用于在日志中预览过长的原始十六进制片段。
func PreviewHex(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
