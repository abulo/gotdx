// Command stock_server_info 演示主站握手、服务器心跳、服务器信息、主站公告与交易所公告等服务端信息接口的查询与预览打印。
//
// 运行示例：`go run ./examples/stock_server_info`
package main

import (
	"log"
	"strings"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	if _, err := client.Connect(); err != nil {
		log.Fatalln(err)
	}

	// 查询服务器心跳并打印心跳日期。
	heartbeat, err := client.GetServerHeartbeat()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("heartbeat date=%d", heartbeat.Date)

	// 查询服务器时间、区域编号、开关位与信息文案；三个保留字段语义待确认，仅原样打印。
	info, err := client.GetServerInfo()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("server_info now=%s region=%d switch=%d info=%q",
		info.TimeNow, info.Region, info.MaybeSwitch, info.Info)
	log.Printf("server_info unknown1=%v unknown2=%v unknown3=%v",
		info.Unknown1, info.Unknown2, info.Unknown3)

	// 查询主站公告标题与正文，正文截断后预览。
	announcement, err := client.GetAnnouncement()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("announcement has_content=%t title=%q preview=%q",
		announcement.HasContent, announcement.Title, preview(announcement.Content, 80))

	// 查询交易所公告的版本号与正文预览。
	exchangeAnnouncement, err := client.GetExchangeAnnouncement()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("exchange_announcement version=%d preview=%q",
		exchangeAnnouncement.Version, preview(exchangeAnnouncement.Content, 80))
}

func preview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}
