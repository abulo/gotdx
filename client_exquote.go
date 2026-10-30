package gotdx

import (
	"strings"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// GetExServerInfo 拉取扩展行情服务器信息；扩展站连接或协议执行失败时返回对应错误。
func (client *Client) GetExServerInfo() (*proto.ExServerInfoReply, error) {
	obj := proto.NewExServerInfo()
	return client.execute(obj)
}

// ExGetCount 查询扩展市场内可交易标的的总数。
func (client *Client) ExGetCount() (*proto.ExGetCountReply, error) {
	obj := proto.NewExGetCount()
	return client.execute(obj)
}

// ExGetCategoryList 查询扩展市场的品种分类列表。
func (client *Client) ExGetCategoryList() (*proto.ExGetCategoryListReply, error) {
	obj := proto.NewExGetCategoryList()
	return client.execute(obj)
}

// ExGetList 从 `start` 偏移开始拉取 `count` 条扩展市场标的列表；`count` 为 0 时使用默认条数。
func (client *Client) ExGetList(start uint32, count uint16) (*proto.ExGetListReply, error) {
	if count == 0 {
		count = types.DefaultExListCount
	}
	obj := proto.NewExGetList(&proto.ExGetListRequest{Start: start, Count: count})
	return client.execute(obj)
}

// ExGetListExtra 调用扩展市场试验协议拉取标的列表；参数 `a`、`b` 的具体含义语义待确认，
// `count` 为请求条数。
func (client *Client) ExGetListExtra(a uint16, b uint16, count uint16) (*proto.ExGetListExtraReply, error) {
	obj := proto.NewExGetListExtra(&proto.ExGetListExtraRequest{A: a, B: b, Count: count})
	return client.execute(obj)
}

// ExGetQuotesList 按分类拉取排序后的扩展市场行情列表；`start` 为起始偏移，`count` 为请求条数
// （为 0 时使用默认条数），`sortType` 为排序方式，`reverse` 为 `true` 时倒序排列。
func (client *Client) ExGetQuotesList(category uint8, start uint16, count uint16, sortType uint16, reverse bool) (*proto.ExGetQuotesListReply, error) {
	if count == 0 {
		count = types.DefaultExQuotesCount
	}
	obj := proto.NewExGetQuotesList(&proto.ExGetQuotesListRequest{
		Category:    category,
		SortType:    sortType,
		Start:       start,
		Count:       count,
		SortReverse: quotesSortReverse(sortType, reverse),
	})
	return client.execute(obj)
}

// ExGetQuote 查询单个扩展市场标的的最新行情；`category` 为市场类别，`code` 为标的代码。
func (client *Client) ExGetQuote(category uint8, code string) (*proto.ExGetQuoteReply, error) {
	obj := proto.NewExGetQuote(&proto.ExGetQuoteRequest{Category: category, Code: makeCode9(code)})
	return client.execute(obj)
}

// ExGetQuotes 批量查询扩展市场标的行情；`categories` 与 `codes` 长度必须一致，
// 配对长度不一致时返回错误。
func (client *Client) ExGetQuotes(categories []uint8, codes []string) (*proto.ExGetQuotesReply, error) {
	stocks, err := makeExStocks(categories, codes)
	if err != nil {
		return nil, err
	}

	obj := proto.NewExGetQuotes(&proto.ExGetQuotesRequest{Stocks: stocks})
	return client.execute(obj)
}

// ExGetQuotes2 批量查询扩展市场行情，使用与 `pytdx2` 第二种批量接口对应的协议；
// `categories` 与 `codes` 长度必须一致，配对长度不一致时返回错误。
func (client *Client) ExGetQuotes2(categories []uint8, codes []string) (*proto.ExGetQuotesReply, error) {
	stocks, err := makeExStocks(categories, codes)
	if err != nil {
		return nil, err
	}

	obj := proto.NewExGetQuotes2(&proto.ExGetQuotesRequest{Stocks: stocks})
	return client.execute(obj)
}

// ExGetKLine 查询扩展市场标的 K 线序列；`category` 为市场类别，`code` 为标的代码，
// `period` 为周期类型，`start` 为起始偏移，`count` 为请求条数，`times` 为周期倍数。
func (client *Client) ExGetKLine(category uint8, code string, period uint16, start uint32, count uint16, times uint16) (*proto.ExGetKLineReply, error) {
	obj := proto.NewExGetKLine(&proto.ExGetKLineRequest{
		Category: category,
		Code:     makeCode9(code),
		Period:   period,
		Times:    times,
		Start:    start,
		Count:    count,
	})
	return client.execute(obj)
}

// ExGetExperiment2487 调用扩展市场试验报价协议 `0x2487` 查询单个标的行情；
// 协议部分字段语义待确认。
func (client *Client) ExGetExperiment2487(category uint8, code string) (*proto.ExExperiment2487Reply, error) {
	obj := proto.NewExExperiment2487(&proto.ExExperiment2487Request{Category: category, Code: makeCode23(code)})
	return client.execute(obj)
}

// ExGetExperiment2488 调用扩展市场试验报价协议 `0x2488` 查询多组试验行情；
// `category` 为市场类别，`code` 为标的代码，`mode` 为查询模式，部分字段语义待确认。
func (client *Client) ExGetExperiment2488(category uint8, code string, mode uint16) (*proto.ExExperiment2488Reply, error) {
	obj := proto.NewExExperiment2488(&proto.ExExperiment2488Request{
		Category: category,
		Code:     makeCode23(code),
		Mode:     mode,
	})
	return client.execute(obj)
}

// ExGetKLine2 调用扩展市场 K 线协议 `0x2489` 查询 K 线序列；参数含义同 ExGetKLine，
// 仅请求条数为 `uint32` 且使用 23 字节代码编码。
func (client *Client) ExGetKLine2(category uint8, code string, period uint16, start uint32, count uint32, times uint16) (*proto.ExGetKLine2Reply, error) {
	obj := proto.NewExGetKLine2(&proto.ExGetKLine2Request{
		Category: category,
		Code:     makeCode23(code),
		Period:   period,
		Times:    times,
		Start:    start,
		Count:    count,
	})
	return client.execute(obj)
}

// ExGetHistoryTransaction 查询指定交易日的扩展市场历史逐笔成交；`date` 为交易日期，
// `category` 为市场类别，`code` 为标的代码。
func (client *Client) ExGetHistoryTransaction(date uint32, category uint8, code string) (*proto.ExGetHistoryTransactionReply, error) {
	obj := proto.NewExGetHistoryTransaction(&proto.ExGetHistoryTransactionRequest{
		Date:     date,
		Category: category,
		Code:     makeFixed43(code),
	})
	return client.execute(obj)
}

// ExGetTickChart 查询扩展市场标的的当日分时图；`category` 为市场类别，`code` 为标的代码。
func (client *Client) ExGetTickChart(category uint8, code string) (*proto.ExGetTickChartReply, error) {
	obj := proto.NewExGetTickChart(&proto.ExGetTickChartRequest{
		Category: category,
		Code:     makeCode23(code),
	})
	return client.execute(obj)
}

// ExGetHistoryTickChart 查询指定交易日的扩展市场历史分时图；`date` 为交易日期，
// `category` 为市场类别，`code` 为标的代码。
func (client *Client) ExGetHistoryTickChart(date uint32, category uint8, code string) (*proto.ExGetHistoryTickChartReply, error) {
	obj := proto.NewExGetHistoryTickChart(&proto.ExGetHistoryTickChartRequest{
		Date:     date,
		Category: category,
		Code:     makeCode23(code),
	})
	return client.execute(obj)
}

// ExGetChartSampling 查询扩展市场标的的分时缩略采样价格序列；`category` 为市场类别，
// `code` 为标的代码。
func (client *Client) ExGetChartSampling(category uint8, code string) (*proto.ExGetChartSamplingReply, error) {
	obj := proto.NewExGetChartSampling(&proto.ExGetChartSamplingRequest{
		Category: uint16(category),
		Code:     makeCode22(code),
	})
	return client.execute(obj)
}

// ExGetBoardList 按板块类型分页拉取扩展市场板块榜单；`boardType` 为板块类型，
// `start` 为起始偏移，`pageSize` 为每页条数。
func (client *Client) ExGetBoardList(boardType uint16, start uint16, pageSize uint16) (*proto.ExGetBoardListReply, error) {
	obj := proto.NewExGetBoardList(&proto.ExGetBoardListRequest{
		PageSize:  pageSize,
		BoardType: boardType,
		Start:     start,
	})
	return client.execute(obj)
}

// ExGetMapping2562 查询扩展市场代码映射表；`market` 为市场代码，`start` 为起始偏移，
// `count` 为请求条数。
func (client *Client) ExGetMapping2562(market uint16, start uint32, count uint32) (*proto.ExMapping2562Reply, error) {
	obj := proto.NewExMapping2562(&proto.ExMapping2562Request{Market: market, Start: start, Count: count})
	return client.execute(obj)
}

// ExGetFileMeta 查询扩展市场文件的元信息，包括文件大小与哈希校验值等字段。
func (client *Client) ExGetFileMeta(filename string) (*proto.GetFileMetaReply, error) {
	obj := proto.NewExGetFileMeta(&proto.GetFileMetaRequest{Filename: makeFixed40(filename)})
	return client.execute(obj)
}

// ExDownloadFile 从 `start` 偏移下载 `size` 字节的扩展市场文件片段；片段大小为 0 表示文件已取完。
func (client *Client) ExDownloadFile(filename string, start uint32, size uint32) (*proto.DownloadFileReply, error) {
	obj := proto.NewExDownloadFile(&proto.ExDownloadFileRequest{
		Start:    start,
		Size:     size,
		Filename: makeFixed40(filename),
	})
	return client.execute(obj)
}

// ExDownloadFullFile 循环分块下载完整的扩展市场文件；`size` 为 0 时先通过文件元信息获取
// 总大小，服务器返回空块或达到目标大小即结束，任一片段下载失败时整体返回错误。
func (client *Client) ExDownloadFullFile(filename string, size uint32) ([]byte, error) {
	if size == 0 {
		meta, err := client.ExGetFileMeta(filename)
		if err == nil {
			size = meta.Size
		}
	}

	var result []byte
	var downloaded uint32
	for {
		reply, err := client.ExDownloadFile(filename, downloaded, types.DefaultDownloadSize)
		if err != nil {
			return nil, err
		}
		if reply.Size == 0 {
			break
		}
		result = append(result, reply.Data...)
		downloaded += reply.Size
		if size != 0 && downloaded >= size {
			break
		}
	}
	return result, nil
}

// ExGetTable 分页拉取并拼接扩展市场普通表格文件的全部文本内容。
func (client *Client) ExGetTable() (string, error) {
	return client.getExTable(false)
}

// ExGetTableDetail 分页拉取并拼接扩展市场详细表格文件的全部文本内容。
func (client *Client) ExGetTableDetail() (string, error) {
	return client.getExTable(true)
}

// getExTable 按 `detail` 选择普通或详细表格协议，循环读取分块直到服务端返回空计数；
// 任一分块拉取失败即中断并返回错误。
func (client *Client) getExTable(detail bool) (string, error) {
	start := uint32(0)
	var content strings.Builder

	for {
		var (
			reply *proto.ExGetTableChunkReply
			err   error
		)
		if detail {
			obj := proto.NewExGetTableDetail(start)
			reply, err = client.execute(obj.ExGetTableChunk)
		} else {
			obj := proto.NewExGetTable(start)
			reply, err = client.execute(obj.ExGetTableChunk)
		}
		if err != nil {
			return "", err
		}
		content.WriteString(reply.Content)
		if reply.Count == 0 {
			break
		}
		start += reply.Count
	}

	return content.String(), nil
}
