// Command mac_symbol_info 演示通过 MAC 协议接口 `MACSymbolInfo` 查询单只证券的行情摘要信息并打印。
//
// 运行示例：`go run ./examples/mac_symbol_info`
package main

import (
	"log"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMACClient()
	defer client.Disconnect()

	// 查询深市证券 000001 的行情摘要；部分字段原始语义待确认，示例仅按返回值原样打印。
	reply, err := client.MACSymbolInfo(types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("mac_symbol_info code=%s name=%s time=%s close=%.2f pre_close=%.2f activity=%d turnover=%.2f vr=%.2f",
		reply.Code, reply.Name, reply.DateTime, reply.Close, reply.PreClose, reply.Activity, reply.Turnover, reply.VR)
}
