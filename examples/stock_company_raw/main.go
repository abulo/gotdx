// Command stock_company_raw 演示公司资料分类、章节正文、财务摘要、除权除息记录以及聚合打包接口的原始结果获取与打印。
//
// 运行示例：`go run ./examples/stock_company_raw`
package main

import (
	"log"
	"strings"

	"github.com/bensema/gotdx/examples/internal/exampleutil"
	"github.com/bensema/gotdx/types"
)

func main() {
	client := exampleutil.NewMainClient()
	defer client.Disconnect()

	if _, err := client.Connect(); err != nil {
		log.Fatalln(err)
	}

	// 查询深市 000001 公司资料包含的章节分类清单。
	categories, err := client.GetCompanyCategories(types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("company_categories count=%d", categories.Count)

	if len(categories.Categories) > 0 {
		first := categories.Categories[0]
		// 取首个分类，按其给出的文件名、偏移与长度读取对应章节正文。
		content, err := client.GetCompanyContent(types.MarketSZ.Uint8(), "000001", first.Filename, first.Start, first.Length)
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("company_content name=%s filename=%s preview=%s",
			first.Name, first.Filename, preview(content.Content, 80))
	}

	// 查询财务摘要：总股本、营业收入、净利润等字段。
	finance, err := client.GetFinanceInfo(types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("finance code=%s updated=%d total_shares=%.2f revenue=%.2f profit=%.2f",
		finance.Code, finance.UpdatedDate, finance.TotalShares, finance.OperatingRevenue, finance.NetProfit)

	// 查询除权除息历史记录，仅打印前 5 条。
	xdxr, err := client.GetXDXRInfo(types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("xdxr code=%s count=%d", xdxr.Code, xdxr.Count)
	for _, item := range xdxr.List[:min(5, len(xdxr.List))] {
		log.Printf("xdxr date=%s category=%d name=%s", item.Date.Format("2006-01-02"), item.Category, item.Name)
	}

	// 查询聚合后的公司资料打包结果，一次性汇总章节、除权除息与财务信息。
	bundle, err := client.GetCompanyInfo(types.MarketSZ.Uint8(), "000001")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("company_bundle sections=%d xdxr=%d finance_nil=%t",
		len(bundle.Sections), len(bundle.XDXR), bundle.Finance == nil)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func preview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}
