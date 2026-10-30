// Command stock_index_tools 演示指数实时信息、指数动量序列、指数 K 线以及分时采样价格四类指数相关接口的查询与打印。
//
// 运行示例：`go run ./examples/stock_index_tools`
package main

import (
	"fmt"
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	// 查询沪市 000001 指数的实时信息：点位、开高低收与上涨、下跌家数。
	info, err := client.StockIndexInfo(types.MarketSH.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("index_info code=%s time=%s close=%.2f open=%.2f high=%.2f low=%.2f up=%d down=%d",
		info.Code, info.ServerTime, info.Close, info.Open, info.High, info.Low, info.UpCount, info.DownCount)

	// 查询指数动量序列，仅打印序列长度与最后一个取值。
	momentum, err := client.StockIndexMomentum(types.MarketSH.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("index_momentum count=%d last=%d", len(momentum), momentum[len(momentum)-1])

	// 查询指数日 K 线，从偏移 0 开始取最近 1 根。
	bars, err := client.GetIndexBars(types.KLINE_TYPE_DAILY, types.MarketSH.Uint8(), "000001", 0, 1)
	if err != nil {
		log.Fatalln(err)
	}
	for _, item := range bars.List {
		log.Printf("index_bar time=%s close=%.2f high=%.2f low=%.2f", item.DateTime, item.Close, item.High, item.Low)
	}

	// 分时采样接口在部分主站主机上不可用，先逐个主机尝试再决定是否打印结果。
	sampling, err := getSampling()
	if err != nil {
		log.Printf("sampling unavailable: %v", err)
	} else {
		// 采样价格序列较长，仅打印前 10 个价格点。
		for i, price := range sampling[:min(10, len(sampling))] {
			log.Printf("sampling index=%d price=%.2f", i, price)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getSampling 逐个尝试主站主机查询分时采样数据，遇到首个可用主机即返回其结果；全部失败时返回最后一次错误。
func getSampling() ([]float64, error) {
	var lastErr error
	for _, host := range exampleutil.MainHosts() {
		client := exampleutil.NewMainClientForHost(host)
		reply, err := client.StockChartSampling(types.MarketSZ.Uint8(), "000001")
		_ = client.Disconnect()
		if err == nil {
			return reply, nil
		}
		lastErr = fmt.Errorf("%s sampling failed: %w", host, err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no main quote hosts configured")
	}
	return nil, lastErr
}
