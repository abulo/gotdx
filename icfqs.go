package gotdx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 以下常量为 ICFQS TQLEX 服务的默认地址。
const (
	DefaultICFQSAddress    = "121.37.193.4:7615"
	DefaultICFQSHotAddress = "hot.icfqs.com:7615"
)

// ICFQSOption 是用于修改 ICFQSClient 配置的函数式选项。
type ICFQSOption func(*ICFQSClient)

// ICFQSClient 是访问 ICFQS TQLEX 接口的轻量 HTTP 客户端；须由 NewICFQS 或 NewICFQSHot 构造。
type ICFQSClient struct {
	address    string
	httpClient *http.Client
}

// ICFQSCode 表示 ICFQS HTTP 接口使用的市场/代码配对。
type ICFQSCode struct {
	Setcode string `json:"setcode"`
	Code    string `json:"code"`
}

// ICFQSTable 是 TQLEX 结果集以表格行列形式组织后的视图。
type ICFQSTable struct {
	Rows []map[string]any `json:"rows"`
}

// NewICFQS 创建访问默认 ICFQS 服务的 HTTP 客户端，并按传入顺序应用各选项覆盖默认配置。
func NewICFQS(opts ...ICFQSOption) *ICFQSClient {
	client := &ICFQSClient{
		address: DefaultICFQSAddress,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

// NewICFQSHot 创建访问热门话题站点 `hot.icfqs.com` 的 ICFQS 客户端，并应用传入选项。
func NewICFQSHot(opts ...ICFQSOption) *ICFQSClient {
	client := NewICFQS(WithICFQSAddress(DefaultICFQSHotAddress))
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

// WithICFQSAddress 设置 ICFQS 服务地址，格式为 `host:port` 或完整基础 URL；传入空串时保持原值。
func WithICFQSAddress(address string) ICFQSOption {
	return func(client *ICFQSClient) {
		if strings.TrimSpace(address) != "" {
			client.address = strings.TrimSpace(address)
		}
	}
}

// WithICFQSHTTPClient 设置该客户端使用的 `http.Client`；传入空指针时保持原值。
func WithICFQSHTTPClient(httpClient *http.Client) ICFQSOption {
	return func(client *ICFQSClient) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

// WithICFQSTimeout 设置 HTTP 请求超时时间；传入非正数时保持原值。
func WithICFQSTimeout(timeout time.Duration) ICFQSOption {
	return func(client *ICFQSClient) {
		if timeout > 0 {
			client.httpClient.Timeout = timeout
		}
	}
}

// PostTQL 按 TQLEX 协议发送带 `Params` 与 `oauth_zzfw` 参数的请求，并解析返回的响应对象。
func (client *ICFQSClient) PostTQL(ctx context.Context, entry string, params []any) (map[string]any, error) {
	body := map[string]any{
		"Params":     params,
		"oauth_zzfw": "1",
	}
	raw, err := client.post(ctx, entry, body)
	if err != nil {
		return nil, err
	}
	return parseICFQSTQLResponse(raw)
}

// PostJSON 向指定入口发送原始 JSON 请求体，并解码标准 JSON 响应。
func (client *ICFQSClient) PostJSON(ctx context.Context, entry string, body any) (map[string]any, error) {
	raw, err := client.post(ctx, entry, body)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ICFQSFormatTables 将结果集 `ResultSets` 转换为表格行集合，列名优先取 `ColName`，缺失时回退到
// `ColDes` 中的 `Name` 字段。
func ICFQSFormatTables(raw map[string]any) []ICFQSTable {
	resultSets, _ := raw["ResultSets"].([]any)
	tables := make([]ICFQSTable, 0, len(resultSets))
	for _, resultSet := range resultSets {
		rs, _ := resultSet.(map[string]any)
		columns := icfqsColumns(rs)
		content, _ := rs["Content"].([]any)
		table := ICFQSTable{Rows: make([]map[string]any, 0, len(content))}
		for _, rawRow := range content {
			values, _ := rawRow.([]any)
			row := make(map[string]any, len(columns))
			for i, column := range columns {
				// 按下标把数组形式的行值对齐到列名；列数多于值时补空值占位。
				if i < len(values) {
					row[column] = values[i]
				} else {
					row[column] = nil
				}
			}
			table.Rows = append(table.Rows, row)
		}
		tables = append(tables, table)
	}
	return tables
}

// ICFQSTopicListRaw 查询话题列表原始响应；`page` 非正时从第 1 页开始。
func (client *ICFQSClient) ICFQSTopicListRaw(ctx context.Context, category string, setcode string, page int) (map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_zy", []any{"00601", category + "|" + setcode, page})
}

// ICFQSSearchTopicsRaw 按关键词搜索话题，返回原始响应。
func (client *ICFQSClient) ICFQSSearchTopicsRaw(ctx context.Context, keyword string) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_zy", []any{"00102", keyword, "0"})
}

// ICFQSNewTopicsRaw 查询最新题材话题，返回原始响应。
func (client *ICFQSClient) ICFQSNewTopicsRaw(ctx context.Context) (map[string]any, error) {
	return client.PostTQL(ctx, "DataAggregation.zttz_xzgn2", []any{"01001", "", 1})
}

// ICFQSHotTopicsRaw 查询热门话题，返回原始响应。
func (client *ICFQSClient) ICFQSHotTopicsRaw(ctx context.Context) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_zy", []any{"00302", "", 1})
}

// ICFQSEventsRaw 查询题材事件，返回原始响应。
func (client *ICFQSClient) ICFQSEventsRaw(ctx context.Context) (map[string]any, error) {
	return client.PostTQL(ctx, "DataAggregation.zttz_jhqz", []any{"00401", "", 10})
}

// ICFQSTopTopicsRaw 查询排行前列的话题；`topN` 非正时使用默认数量，返回原始响应。
func (client *ICFQSClient) ICFQSTopTopicsRaw(ctx context.Context, topN int) (map[string]any, error) {
	if topN <= 0 {
		topN = 10
	}
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_zy", []any{"00101", "", topN})
}

// ICFQSTopicDetailRaw 查询指定话题详情，返回原始响应。
func (client *ICFQSClient) ICFQSTopicDetailRaw(ctx context.Context, code string, setcode string) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_xqy", []any{"00301", code, setcode})
}

// ICFQSTopicKLineRaw 查询话题走势 K 线数据，返回原始响应。
func (client *ICFQSClient) ICFQSTopicKLineRaw(ctx context.Context, code string, setcode string) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_xqy_v2_qsid", []any{"00501", code, 3, "", setcode})
}

// ICFQSTopicStocksRaw 分页查询话题成分股；`page`、`size` 非正时使用默认值，返回原始响应。
func (client *ICFQSClient) ICFQSTopicStocksRaw(ctx context.Context, code string, setcode string, page int, size int) (map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	return client.PostTQL(ctx, "CWServ.ph_tdxdatacenter_zttz_xggp", []any{"00901", code, setcode, 1, size, 0, page})
}

// ICFQSTopicQuotesRaw 批量查询话题成分股行情快照，返回原始响应。
func (client *ICFQSClient) ICFQSTopicQuotesRaw(ctx context.Context, codes []ICFQSCode) (map[string]any, error) {
	codeValues, setcodeValues := splitICFQSCodes(codes)
	body := []map[string]any{{
		"ReqId":    "200800",
		"modname":  "module_misc.dll",
		"Code":     codeValues,
		"PageSize": fmt.Sprintf("%d", len(codeValues)),
		"Page":     "0",
		"Desc":     "0",
		"Setcode":  setcodeValues,
		"Sort":     "0",
	}}
	return client.PostJSON(ctx, "HQServ.hq_nlp", body)
}

// ICFQSTopicRotationRaw 查询题材轮动数据；各项参数为 0 或空串时使用协议默认值，返回原始响应。
func (client *ICFQSClient) ICFQSTopicRotationRaw(ctx context.Context, dataNum int, dataType int, dataDate int, themeType string) (map[string]any, error) {
	if dataNum == 0 {
		dataNum = 1
	}
	if dataType == 0 {
		dataType = 1
	}
	if dataDate == 0 {
		dataDate = 2
	}
	if themeType == "" {
		themeType = "0"
	}
	body := []map[string]any{{
		"ReqId":     "200773",
		"modname":   "mod_copilot.dll",
		"dataDate":  fmt.Sprintf("%d", dataDate),
		"dataType":  fmt.Sprintf("%d", dataType),
		"dataNum":   fmt.Sprintf("%d", dataNum),
		"themeType": themeType,
	}}
	return client.PostJSON(ctx, "HQServ.hq_nlp_copilot", body)
}

// ICFQSQuotesBatchRaw 批量查询股票行情快照；未指定列时默认返回收盘价与最新价，返回原始响应。
func (client *ICFQSClient) ICFQSQuotesBatchRaw(ctx context.Context, codes []ICFQSCode, wantColumns []string) (map[string]any, error) {
	if len(wantColumns) == 0 {
		wantColumns = []string{"CLOSE", "NOW"}
	}
	codeValues, setcodeValues := splitICFQSCodes(codes)
	body := map[string]any{
		"Setcode": setcodeValues,
		"Head":    map[string]any{"Target": 0},
		"WantCol": wantColumns,
		"Code":    codeValues,
	}
	return client.PostJSON(ctx, "HQServ.PBCombHQ", body)
}

// ICFQSLHBDetailRaw 查询个股龙虎榜明细，返回原始响应。
func (client *ICFQSClient) ICFQSLHBDetailRaw(ctx context.Context, symbol string, startDate string, endDate string) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.cfg_fx_yzlhb", []any{"yybxq", startDate, endDate, symbol, "", 0, 2000})
}

// ICFQSYYBDetailRaw 查询营业部龙虎榜明细，返回原始响应。
func (client *ICFQSClient) ICFQSYYBDetailRaw(ctx context.Context, yybName string, startDate string, endDate string) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.cfg_fx_yzlhb", []any{"tjyyb", startDate, endDate, "", yybName, 0, 2000})
}

// ICFQSYZDetailRaw 查询主力资金明细，返回原始响应。
func (client *ICFQSClient) ICFQSYZDetailRaw(ctx context.Context, code string, startDate string, endDate string) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.cfg_fx_yzlhb", []any{"yzxq", startDate, endDate, code, "", 0, 2000})
}

// ICFQSMRFPRaw 按类型查询每日复盘数据；`limit` 非正时使用默认条数，返回原始响应。
func (client *ICFQSClient) ICFQSMRFPRaw(ctx context.Context, reviewType string, date string, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 30
	}
	return client.PostTQL(ctx, "CWServ.cfg_tk_mrfp", []any{date, reviewType, "", 0, limit})
}

// ICFQSMRFPLatestDateRaw 查询每日复盘数据可用的最新日期，返回原始响应。
func (client *ICFQSClient) ICFQSMRFPLatestDateRaw(ctx context.Context) (map[string]any, error) {
	return client.PostTQL(ctx, "CWServ.cfg_tk_mrfp", []any{"0", "rq", "", 0, 30})
}

// post 向指定 TQLEX 入口发送 JSON 请求并返回原始响应字节；非 2xx 状态码附带响应预览报错。
func (client *ICFQSClient) post(ctx context.Context, entry string, body any) ([]byte, error) {
	// 空上下文回退为无超时后台上下文，保证请求仍受 `http.Client.Timeout` 约束。
	if ctx == nil {
		ctx = context.Background()
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.tqlURL(entry), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 状态码不在 2xx 区间视为失败，截断响应体预览便于定位入口错误。
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("icfqs %s returned status %d: %s", entry, resp.StatusCode, icfqsPreview(string(raw), 200))
	}
	return raw, nil
}

// tqlURL 拼装 TQLEX 完整地址：去掉末尾斜杠，未带 `http://` 或 `https://` 前缀时补 `http://`，
// 再追加入口参数。
func (client *ICFQSClient) tqlURL(entry string) string {
	address := strings.TrimRight(client.address, "/")
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return address + "/TQLEX?Entry=" + entry
}

// parseICFQSTQLResponse 从可能带有前置文本的响应中截取第一个配对完整的 JSON 对象并解析。
// 括号配对时跳过字符串内部内容与转义字符，避免把字符串里的花括号误计为层级变化。
func parseICFQSTQLResponse(raw []byte) (map[string]any, error) {
	start := bytes.IndexByte(raw, '{')
	if start < 0 {
		return nil, fmt.Errorf("cannot parse icfqs response: %s", icfqsPreview(string(raw), 200))
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var result map[string]any
				if err := json.Unmarshal(raw[start:i+1], &result); err != nil {
					return nil, err
				}
				return result, nil
			}
		}
	}
	return nil, fmt.Errorf("cannot parse icfqs response: %s", icfqsPreview(string(raw), 200))
}

// icfqsColumns 提取结果集列名：优先使用 `ColName` 数组，缺失时回退到 `ColDes` 中的 `Name` 字段。
func icfqsColumns(resultSet map[string]any) []string {
	if rawColumns, ok := resultSet["ColName"].([]any); ok && len(rawColumns) > 0 {
		columns := make([]string, 0, len(rawColumns))
		for _, rawColumn := range rawColumns {
			columns = append(columns, fmt.Sprint(rawColumn))
		}
		return columns
	}
	if rawColumnDefs, ok := resultSet["ColDes"].([]any); ok {
		columns := make([]string, 0, len(rawColumnDefs))
		for _, rawColumnDef := range rawColumnDefs {
			if columnDef, ok := rawColumnDef.(map[string]any); ok {
				columns = append(columns, fmt.Sprint(columnDef["Name"]))
			}
		}
		return columns
	}
	return nil
}

// splitICFQSCodes 把市场/代码配对列表拆成两个平行数组，供批量行情请求按位置对齐传参。
func splitICFQSCodes(codes []ICFQSCode) ([]string, []string) {
	codeValues := make([]string, 0, len(codes))
	setcodeValues := make([]string, 0, len(codes))
	for _, code := range codes {
		codeValues = append(codeValues, code.Code)
		setcodeValues = append(setcodeValues, code.Setcode)
	}
	return codeValues, setcodeValues
}

// icfqsPreview 截断响应文本用于错误预览；`max` 非正或文本未超长时原样返回。
func icfqsPreview(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max]
}
