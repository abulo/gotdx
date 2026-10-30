package gotdx

import (
	"encoding/csv"
	"strings"

	"github.com/bensema/gotdx/proto"
	"github.com/bensema/gotdx/types"
)

// CompanyInfoSection 表示 F10 公司信息中的一个章节，包含章节名称与正文内容。
type CompanyInfoSection struct {
	Name    string
	Content string
}

// CompanyInfoBundle 聚合返回的公司信息，包含 F10 章节、除权除息记录与财务信息。
type CompanyInfoBundle struct {
	Sections []CompanyInfoSection
	XDXR     []proto.XDXRItem
	Finance  *proto.GetFinanceInfoReply
}

// GetCompanyInfo 聚合获取指定证券的公司信息，包含 F10 章节、除权除息记录与财务信息。
// 除权除息与财务信息为尽力而为：拉取失败时保留已取到的章节，不中断整体结果。
func (client *Client) GetCompanyInfo(market uint8, code string) (*CompanyInfoBundle, error) {
	categories, err := client.GetCompanyCategories(market, code)
	if err != nil {
		return nil, err
	}

	result := &CompanyInfoBundle{}
	// 按目录逐项拉取章节正文，章节内容失败属于硬性错误，直接返回。
	for _, category := range categories.Categories {
		content, err := client.GetCompanyContent(market, code, category.Filename, category.Start, category.Length)
		if err != nil {
			return nil, err
		}
		result.Sections = append(result.Sections, CompanyInfoSection{
			Name:    category.Name,
			Content: content.Content,
		})
	}

	// 除权除息、财务信息允许缺失：仅在成功时填入结果。
	xdxr, err := client.GetXDXRInfo(market, code)
	if err == nil {
		result.XDXR = xdxr.List
	}

	finance, err := client.GetFinanceInfo(market, code)
	if err == nil {
		result.Finance = finance
	}

	return result, nil
}

// DownloadFullFile 按固定块大小循环下载完整文件；参数 `size` 为 0 时先取文件元信息作为总大小，
// 服务器返回空块或达到目标大小即结束。
func (client *Client) DownloadFullFile(filename string, size uint32) ([]byte, error) {
	// 未指定大小时从元信息获取文件大小作为下载上限。
	if size == 0 {
		meta, err := client.GetFileMeta(filename)
		if err == nil {
			size = meta.Size
		}
	}

	var result []byte
	var downloaded uint32
	for {
		reply, err := client.DownloadFile(filename, downloaded, types.DefaultDownloadSize)
		if err != nil {
			return nil, err
		}
		// 空块表示文件已下载完毕。
		if reply.Size == 0 {
			break
		}
		result = append(result, reply.Data...)
		downloaded += reply.Size
		// 达到目标大小停止；`size` 为 0 时仅靠空块结束循环。
		if size != 0 && downloaded >= size {
			break
		}
	}
	return result, nil
}

// GetBlockFile 下载并返回指定板块文件的完整字节内容。
func (client *Client) GetBlockFile(filename string) ([]byte, error) {
	meta, err := client.GetFileMeta(filename)
	if err != nil {
		return nil, err
	}
	return client.DownloadFullFile(filename, meta.Size)
}

// GetTableFile 下载并解析以竖线分隔的表格文件，返回二维字符串表格。
func (client *Client) GetTableFile(filename string) ([][]string, error) {
	content, err := client.DownloadFullFile(filename, 0)
	if err != nil {
		return nil, err
	}
	return parsePipeTableContent(content), nil
}

// GetCSVFile 下载并解析指定 CSV 文件，返回二维字符串表格。
func (client *Client) GetCSVFile(filename string) ([][]string, error) {
	content, err := client.DownloadFullFile(filename, 0)
	if err != nil {
		return nil, err
	}
	return parseCSVContent(content)
}

// parsePipeTableContent 把 GBK 编码的竖线分隔表格转成二维字符串，跳过空行与首尾空白。
func parsePipeTableContent(content []byte) [][]string {
	lines := strings.Split(proto.Utf8ToGbk(content), "\n")
	result := make([][]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, strings.Split(line, "|"))
	}
	return result
}

// parseCSVContent 解析 GBK 编码的 CSV；`FieldsPerRecord` 置为 -1 允许每行字段数不一致。
func parseCSVContent(content []byte) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(proto.Utf8ToGbk(content)))
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}
