# gotdx

<p align="center">通达信行情协议的 Go 客户端，覆盖主行情、扩展市场、MAC 板块/统一行情、F10/文件接口，以及一个可直接运行的 Web Viewer。</p>

<p align="center">
  <a href="#quickstart">快速开始</a> ·
  <a href="#capabilities">能力概览</a> ·
  <a href="#examples">示例</a> ·
  <a href="#viewer">Web Viewer</a> ·
  <a href="#api">API 速览</a> ·
  <a href="#testing">测试</a> ·
  <a href="TdxProtocol.md">协议文档</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.27+">
  <img src="https://img.shields.io/badge/Markets-Main%20%2F%20Extended-123B67" alt="Markets">
  <img src="https://img.shields.io/badge/MAC-Supported-0F766E" alt="MAC">
  <img src="https://img.shields.io/badge/WebViewer-Built--in-C86B36" alt="Web Viewer">
</p>

`gotdx` 把通达信协议里常用的查询能力整理成更偏 Go 的调用方式。你可以直接查主行情和扩展市场的快照、K 线、分时、逐笔，也可以继续往下钻到 F10、财务、板块文件、扩展表格和 MAC 板块协议；如果只是想先看接口返回长什么样，仓库里还带了一个可以直接打开的网页查看器。

## 为什么用 gotdx

- 一个 `Client` 同时管理主站和扩展市场，适合做统一监控和跨市场抓取。
- 提供两套入口：高阶统一接口 `Stock* / Ex* / MAC* / Goods*`，以及面向协议细节的底层 `Get* / ExGet*`。
- 地址池、超时、重试都可配，适合对接不稳定的真实行情站点。
- 内置主站、扩展、MAC、券商 host/IP 列表，并支持 TCP 测速选最快节点。
- 自带 30+ 个示例，覆盖从列表、快照、K 线、分时到 F10、板块、扩展表格。
- 自带 `cmd/webviewer`，不写代码也能直接调方法、填参数、看返回字段。
- 协议实现、示例和 Web Viewer 已整理为统一风格，便于继续补协议、核字段和排查差异。

<a id="quickstart"></a>
## 快速开始

### 安装

```bash
go get github.com/bensema/gotdx
```

### 最小示例

```go
package main

import (
	"log"

	"github.com/bensema/gotdx"
)

func main() {
	mainHosts := gotdx.MainHostAddresses()
	exHosts := gotdx.ExHostAddresses()

	client := gotdx.New(
		gotdx.WithTCPAddress(mainHosts[0]),
		gotdx.WithTCPAddressPool(mainHosts[1:]...),
		gotdx.WithExTCPAddress(exHosts[0]),
		gotdx.WithExTCPAddressPool(exHosts[1:]...),
		gotdx.WithTimeoutSec(6),
	)
	defer client.Disconnect()

	stocks, err := client.StockQuotesDetail(
		[]uint8{gotdx.MarketSZ, gotdx.MarketSH},
		[]string{"000001", "600519"},
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range stocks {
		log.Printf("stock code=%s price=%.2f turnover=%.2f%%", item.Code, item.Price, item.Turnover)
	}

	exQuotes, err := client.ExQuotes(
		[]uint8{gotdx.ExCategoryUSStock},
		[]string{"TSLA"},
	)
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range exQuotes {
		log.Printf("ex: %+v", item)
	}
}
```

### 从哪套 API 开始

- 想尽快拿到业务数据：优先用 `Stock* / Ex* / MAC*` 统一高阶入口。
- 想逐个协议排查字段：用 `Get* / ExGet* / GetMAC*` 底层接口。
- 想先确认请求参数和返回格式：直接运行 `go run ./cmd/webviewer`。

### 内置 Host 列表与测速

```go
results := gotdx.ProbeHosts(gotdx.MainHosts(), time.Second)
for _, item := range results[:3] {
	log.Printf("main host=%s addr=%s reachable=%v latency=%s",
		item.Name, item.Address, item.Reachable, item.Latency)
}

fastest, err := gotdx.FastestHost(gotdx.ExHosts(), time.Second)
if err != nil {
	log.Fatal(err)
}
log.Printf("fastest ex host: %s %s", fastest.Name, fastest.Address)

client := gotdx.New(
	gotdx.WithTimeoutSec(6),
)
```

可用列表包括：`MainHosts`、`BrokerHosts`、`ExHosts`、`MACHosts`、`MACExHosts`，以及对应的 `*HostAddresses` 便捷函数。

### 连接池

`gotdx.Client` 以**动态连接池**与行情服务器通信：`Connect`（或 `ConnectEx` / `ConnectMAC`）按 `WithPoolSize` 预建 N 条 TCP 连接，每条连接内部串行执行请求、连接之间并行，吞吐不再受单连接往返限制。建池走快路径：先按配置顺序尝试首个地址（默认或 `WithTCPAddress` 指定的主地址），网络正常时建池不引入探活耗时；首个地址不可用时才并发探活——探活为"拨号 + 模式握手验证"，握手成功才判定可用并按总耗时排序，池内全部连接统一指向当前最快可用服务器，避免请求被分散到慢服务器。单个地址拨号或握手失败自动按排序逐级回退下一候选（深度限制 3），先建成一条即降级可用、未建满由后台补建循环补齐。运行期间**每分钟**重新探活一次并切换到当前最快的服务器（候选收敛为池内地址与最近可达地址，每 5 次全量刷新一次），网络波动或服务器迁移时连接池自动跟随。

```go
client := gotdx.New(
	gotdx.WithPoolSize(3),       // 默认 3，非正数忽略并回退默认
	gotdx.WithTimeoutSec(6),
)
if _, err := client.Connect(); err != nil {
	log.Fatal(err)
}
defer client.Disconnect()
```

运行期某条连接被丢弃（网络错误、响应帧损坏）后，后台补建循环每秒检查池容量并自动补齐；`Disconnect` 关闭池内全部连接并停止补建。`CurrentAddress` 返回池内首条连接实际连通的服务器地址。各查询方法保持原有签名，可安全并发调用。

<a id="capabilities"></a>
## 能力概览

| 模块 | 典型能力 | 代表接口 |
| --- | --- | --- |
| 主行情 | 股票/指数列表、快照、K 线、分时、逐笔、指数工具、异动、集合竞价 | `StockQuotesDetail`, `StockKLine`, `StockIndexInfo`, `StockUnusual`, `StockAuction` |
| 扩展市场 | 美股/港股/期货等扩展标的列表、报价、K 线、历史成交、表格 | `ExQuotes`, `ExKLine`, `ExHistoryTransaction`, `ExTable` |
| TDX 商品语义 | 对齐 `goods_*` 命名的商品总览、列表、报价、K 线、分时、历史成交 | `GoodsCount`, `GoodsList`, `GoodsVarieties`, `GoodsQuotes`, `GoodsKLine` |
| F10 与文件 | 公司信息分类、正文、财务、除权除息、文件下载、板块文件 | `GetCompanyInfo`, `GetFinanceInfo`, `GetXDXRInfo`, `DownloadFullFile` |
| MAC 协议 | 板块列表、成分股、成分报价、动态成分报价、批量股票报价、单只快照、逐笔成交、竞价、多日分时、股票摘要、资金流向、服务端交易时段、K线偏移信息、文件查询/下载、市场监控、所属板块、统一 K 线 | `MACBoardList`, `MACBoardMembers`, `MACBoardMembersWithSort`, `MACBoardMembersQuotes`, `MACBoardMembersQuotesWithSort`, `MACBoardMembersQuotesDynamic`, `MACBoardMembersQuotesDynamicWithFilter`, `MACSymbolQuotes`, `MACQuotes`, `MACTransactions`, `MACAuction`, `MACTickCharts`, `MACSymbolInfo`, `MACCapitalFlow`, `MACServerInfo`, `MACKLineOffset`, `MACMarketMonitor`, `MACSymbolBars` |
| ICFQS HTTP | 主题投资、主题行情、批量行情、龙虎榜、每日复盘等 TQLEX/JSON 原始接口 | `NewICFQS`, `PostTQL`, `ICFQSTopicListRaw`, `ICFQSQuotesBatchRaw`, `ICFQSMRFPRaw` |
| 协议调试 | 原始协议响应、扩展实验接口、网页查看器 | `MainTodoB`, `MainClient26AD`, `ExExperiment2487`, `cmd/webviewer` |

## 项目结构

- `proto/`: 各协议的请求/响应序列化与反序列化实现。
- `cache/`: 基于 Redis 的缓存层基座（机制底座），按数据形态分两类存储：时间序列能力（K 线/分时/逐笔/竞价/MAC 时序）采用「ZSet 闭合历史 + String 实时」双结构并走 Lua 原子流转，ZSet 永久保存不自动淘汰；行情快照/列表/服务器/公司资料/文件等采用整段键值缓存；两类均含空标记防穿透、singleflight+分布式锁防击穿、熔断防雪崩，详见「缓存层」一节。
- `client_quote.go`: 主行情底层接口。
- `client_exquote.go`: 扩展市场底层接口。
- `client_mac.go`: MAC 协议接口。
- `client_unified.go` / `client_goods.go`: `Stock* / Ex* / MAC* / Goods*` 高阶统一入口。
- `cmd/webviewer/`: 浏览器调试界面。
- `examples/`: 可直接运行的示例。

<a id="examples"></a>
## 缓存层

`cache` 包为 gotdx 提供基于 Redis 的缓存层**基座**：当前已实现存储机制与防护底座，
对外查询能力（与 `gotdx.Client` 同名方法）在此基座上逐能力族构建，每个能力族一个
源文件。所有机制并发安全。

**时间序列能力（K 线、分时、逐笔、历史成交、竞价、MAC 时序等）**——凡属
「最新一根未闭合 + 历史已闭合」形态的数据，采用 **ZSet 闭合历史 + String 实时**
双结构存储：

- 未闭合的最新一根存入普通 String（键为 ZSet 键加 `:live` 后缀），值为
  「分数|更新时刻|base64 数据」三段拼接文本；tick 到达时由单条 Lua 脚本原子完成
  「同根覆盖 / 前一根闭合流转进 ZSet / 续写 TTL」，TTL 仅作行情源断流兜底。
- 已闭合的根按 score 存入 ZSet：score 统一为 Unix 毫秒（时间戳换算），member 为
  单根应答 MessagePack 编码后再 base64 的文本，与实时槽编码一致、可被 Lua 原样搬移。
- **ZSet 永久保存，不自动淘汰**（无后台任务、无 `Stop()`）；如需清理只能外部显式
  `DEL` 或 `ZREMRANGEBYSCORE`。
- 历史修正写入由 Lua 脚本原子完成：先按 score 删除同 score 旧成员再 ZADD（修正覆盖
  而不堆积）；逐笔、成交等「同分可多笔共存」的族以追加写入绕过同分唯一约束。
- 查询用一条 pipeline 同时完成空标记检查、实时槽读取与 ZSet 最新段读取（单次往返），
  合并后按分数降序（时间从新到旧）排序，按 start/count 窗口切片返回；实时槽超过
  刷新间隔未更新时拉一次最新一根做 tick 更新（同样受熔断保护）。

**普通键值能力（行情快照、证券列表、榜单、服务器信息、公司资料、文件等）**——整段
应答 MessagePack 编码后存入 Redis String（信封带版本号便于格式迁移），附带空标记，
不设体积上限。

**防护机制（两类通用）**：

- 穿透防护：回源失败或结果为空时写入短 TTL 空标记（默认 30 秒），存活期内不再回源，
  防恶意穿透打垮行情源；失败与空结果用不同标记值区分。
- 击穿防护：同进程 `singleflight` 合并同键并发；跨进程抢 Redis 分布式锁
  （SET NX EX，5 秒超时防死锁），持锁方二次确认后才回源，以 Lua 校验 token 解锁；
  抢锁失败直连回源兜底，同键并发只打一次行情服务器。
- 熔断防护：回源连续失败达到阈值（默认 5 次）打开熔断，冷却期内（默认 10 秒）未命中
  查询快速失败返回 `ErrCircuitOpen`，不再打行情源；冷却期满放行单个试探请求，成功即
  关断、失败重新打开。
- 降级：Redis 故障时自动退化为直连回源（查询、锁、熔断三层全部兜底），不因缓存层
  故障阻断行情。

```go
import (
    "github.com/bensema/gotdx"
    "github.com/bensema/gotdx/cache"
    "github.com/redis/go-redis/v9"
)

cli := gotdx.New()                    // 行情客户端，按需传 gotdx.WithTCPAddress 等选项，使用前 Connect
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
cached := cache.NewCacheClient(rdb, cli) // 可传 cache.WithKeyPrefix、cache.WithEmptyTTL、
                                         // cache.WithLockTimeout、cache.WithLiveTTL、
                                         // cache.WithCircuitFailThreshold、cache.WithCircuitOpenTimeout
// 非法选项值（阈值 0、负 TTL、空前缀）自动回退默认值。
// 能力层在此基座上按族构建（每个能力族一个源文件，覆盖 gotdx 全部查询能力）；
// 构建后调用方式与 gotdx.Client 同名方法一致，仅首位新增 context.Context。

// 运行时观测：Stats() 返回命中/空标记命中/回源/回源失败/熔断拒绝五项原子计数快照，
// 可据此计算缓存命中率并判断行情源健康度。
stats := cached.Stats()
fmt.Println(stats.Hits, stats.EmptyHits, stats.Fetches, stats.SourceErrors, stats.CircuitRejects)
```

基座文件：`client.go`（入口与配置）、`kv.go`（键值缓存机制）、`ts.go`（时序双结构
机制）、`lua.go`（原子脚本）、`breaker.go`（回源熔断器）。

## 示例

下面这些命令可以直接运行：

| 场景 | 命令 |
| --- | --- |
| 主行情快照 | `go run ./examples/stock_quotes` |
| 批量主行情 | `go run ./examples/stock_batch_quotes` |
| 列表与分页遍历 | `go run ./examples/stock_list` / `go run ./examples/stock_paged_list` |
| K 线与指数工具 | `go run ./examples/stock_kline` / `go run ./examples/stock_index_tools` |
| 分时、历史分时、逐笔 | `go run ./examples/stock_tick` / `go run ./examples/stock_history` / `go run ./examples/stock_transaction` |
| F10、公司资料、板块文件 | `go run ./examples/stock_f10_block` / `go run ./examples/stock_company_raw` / `go run ./examples/stock_block_raw` |
| 市场监控 | `go run ./examples/stock_market_watch` |
| 主机测速与地址池 | `go run ./examples/host_probe` |
| 主站服务与试验协议 | `go run ./examples/stock_server_info` / `go run ./examples/main_experimental` |
| 主行情兼容协议 | `go run ./examples/stock_list_old` / `go run ./examples/stock_feature_452` / `go run ./examples/stock_quotes_encrypt` / `go run ./examples/stock_kline_offset` / `go run ./examples/stock_history_transaction_with_trans` |
| 扩展市场单只/批量报价 | `go run ./examples/ex_quote` / `go run ./examples/ex_quotes` / `go run ./examples/ex_quotes2` |
| 扩展市场列表与分类 | `go run ./examples/ex_count` / `go run ./examples/ex_list` / `go run ./examples/ex_paged_list` / `go run ./examples/ex_category_list` |
| 扩展市场 K 线、分时、历史成交 | `go run ./examples/ex_kline` / `go run ./examples/ex_tick` / `go run ./examples/ex_history` |
| 扩展试验与补充协议 | `go run ./examples/ex_list_extra` / `go run ./examples/ex_board_list` / `go run ./examples/ex_experiment_2487` / `go run ./examples/ex_experiment_2488` / `go run ./examples/ex_kline2` / `go run ./examples/ex_mapping_2562` |
| 扩展市场表格 | `go run ./examples/ex_table` / `go run ./examples/ex_table_detail` |
| TDX 商品语义入口 | `go run ./examples/goods_overview` |
| MAC 协议 | `go run ./examples/mac_board_list` / `go run ./examples/mac_board_members` / `go run ./examples/mac_board_members_quotes` / `go run ./examples/mac_board_members_quotes_dynamic` / `go run ./examples/mac_symbol_quotes` / `go run ./examples/mac_market_monitor` / `go run ./examples/mac_quotes` / `go run ./examples/mac_transactions` / `go run ./examples/mac_auction` / `go run ./examples/mac_tick_charts` / `go run ./examples/mac_symbol_info` / `go run ./examples/mac_capital_flow` / `go run ./examples/mac_server_info` / `go run ./examples/mac_kline_offset` / `go run ./examples/mac_file_query` / `go run ./examples/mac_symbol_belong_board` / `go run ./examples/mac_symbol_bars` |
| ICFQS HTTP 原始接口 | `go run ./examples/icfqs_raw` |
| 统一监控示例 | `go run ./examples/unified_watchlist` |

<details>
<summary>查看完整示例目录</summary>

- `examples/stock_count`
- `examples/stock_server_info`
- `examples/stock_list`
- `examples/stock_list_old`
- `examples/stock_paged_list`
- `examples/stock_batch_quotes`
- `examples/stock_quotes`
- `examples/stock_quotes_encrypt`
- `examples/stock_lowlevel_quote`
- `examples/stock_quotes_list`
- `examples/stock_kline`
- `examples/stock_kline_offset`
- `examples/stock_feature_452`
- `examples/stock_index_tools`
- `examples/stock_tick`
- `examples/stock_history`
- `examples/stock_history_transaction_with_trans`
- `examples/stock_market_watch`
- `examples/host_probe`
- `examples/icfqs_raw`
- `examples/stock_transaction`
- `examples/stock_f10_block`
- `examples/stock_company_raw`
- `examples/stock_block_raw`
- `examples/main_experimental`
- `examples/ex_count`
- `examples/ex_quote`
- `examples/ex_list`
- `examples/ex_list_extra`
- `examples/ex_paged_list`
- `examples/ex_quotes`
- `examples/ex_quotes2`
- `examples/ex_quotes_list`
- `examples/ex_kline`
- `examples/ex_kline2`
- `examples/ex_board_list`
- `examples/ex_experiment_2487`
- `examples/ex_experiment_2488`
- `examples/ex_mapping_2562`
- `examples/goods_overview`
- `examples/ex_history`
- `examples/ex_tick`
- `examples/ex_server_info`
- `examples/ex_sampling`
- `examples/ex_category_list`
- `examples/ex_table`
- `examples/ex_table_detail`
- `examples/mac_board_list`
- `examples/mac_board_members`
- `examples/mac_board_members_quotes`
- `examples/mac_board_members_quotes_dynamic`
- `examples/mac_symbol_quotes`
- `examples/mac_market_monitor`
- `examples/mac_quotes`
- `examples/mac_transactions`
- `examples/mac_auction`
- `examples/mac_tick_charts`
- `examples/mac_symbol_info`
- `examples/mac_capital_flow`
- `examples/mac_server_info`
- `examples/mac_kline_offset`
- `examples/mac_file_query`
- `examples/mac_symbol_belong_board`
- `examples/mac_symbol_bars`
- `examples/unified_watchlist`

</details>

<a id="viewer"></a>
## Web Viewer

仓库内置了一个轻量的网页查看器，适合在这些场景下使用：

- 先确认某个方法应该填哪些参数。
- 快速查看返回字段，而不是先写一段测试代码。
- 对照 MAC 动态位图字段，直接看 `bit/name/format/description`，也可以用 TDX 商品语义分组调 `goods_*` 入口。
- 直接检查主站文件、表格文件和 CSV 文件的结构化结果。
- 对比不同主机返回的数据差异。
- 调试实验协议或原始接口。

![gotdx Web Viewer 截图](docs/images/webviewer-screenshot.png)

启动：

```bash
go run ./cmd/webviewer
```

默认地址：

```text
http://127.0.0.1:8080
```

Web Viewer 中的 ICFQS HTTP 原始接口已按 `主题投资`、`行情`、`龙虎榜`、`每日复盘` 分组；多表响应可用 `table_index` 选择展示的 `ResultSets`，完整响应保留在 Raw 面板。

<a id="api"></a>
## API 速览

### 高阶统一入口

- `StockQuotesDetail`、`StockQuotesList`、`StockQuotes` 会按证券列表里的 `DecimalPoint` 尽力修正非 2 位小数价格，并在可获取到流通股本时补齐 `Turnover`。
- 主行情：`StockCount`, `StockList`, `StockQuotesDetail`, `StockKLine`, `StockTickChart`, `StockIndexInfo`, `StockIndexMomentum`, `StockChartSampling`, `StockAuction`, `StockTopBoard`, `StockUnusual`, `StockVolumeProfile`, `StockHistoryOrders`, `StockHistoryTransaction`, `StockF10`
- 扩展市场：`ExCount`, `ExList`, `ExQuote`, `ExQuotes`, `ExKLine`, `ExTickChart`, `ExHistoryTransaction`, `ExTable`
- 商品语义：`GoodsCount`, `GoodsCategoryList`, `GoodsList`, `GoodsVarieties`, `GoodsQuote`, `GoodsQuotes`, `GoodsQuotesList`, `GoodsKLine`, `GoodsTickChart`, `GoodsChartSampling`, `GoodsHistoryTransaction`
- MAC：`MACBoardList`, `MACBoardMembers`, `MACBoardMembersWithSort`, `MACBoardMembersQuotes`, `MACBoardMembersQuotesWithSort`, `MACBoardMembersQuotesDynamic`, `MACBoardMembersQuotesDynamicWithFilter`, `MACSymbolQuotes`, `MACQuotes`, `MACQuotesWithDate`, `MACTransactions`, `MACTransactionsWithDate`, `MACAuction`, `MACTickCharts`, `MACSymbolInfo`, `MACCapitalFlow`, `MACFileList`, `MACDownloadFullFile`, `MACMarketMonitor`, `MACSymbolBelongBoard`, `MACSymbolBars`；`MACSymbolBars` 会基于返回的流通股本字段尽力补齐 `Turnover`。

### 常用底层接口

- 主行情：`GetSecurityCount`, `GetSecurityListRange`, `GetQuotesDetail`, `GetIndexInfo`, `GetIndexMomentum`, `GetVolumeProfile`, `GetMinuteTimeData`, `GetAuction`, `GetTopBoard`, `GetUnusual`, `GetTransactionData`, `GetHistoryOrders`
- F10/文件：`GetCompanyCategories`, `GetCompanyContent`, `GetFinanceInfo`, `GetXDXRInfo`, `DownloadFile`, `GetBlockFile`
- 扩展市场：`ExGetCategoryList`, `ExGetQuotesList`, `ExGetQuote`, `ExGetChartSampling`, `ExGetFileMeta`, `ExDownloadFile`
- MAC：`GetMACBoardCount`, `GetMACBoardList`, `GetMACBoardMembers`, `GetMACBoardMembersQuotes`, `GetMACBoardMembersQuotesDynamic`, `GetMACBoardMembersQuotesDynamicWithFilter`, `GetMACSymbolQuotes`, `GetMACQuotes`, `GetMACQuotesWithDate`, `GetMACTransactions`, `GetMACTransactionsWithDate`, `GetMACAuction`, `GetMACTickCharts`, `GetMACSymbolInfo`, `GetMACCapitalFlow`, `GetMACFileList`, `GetMACFileDownload`, `GetMACMarketMonitor`, `GetMACSymbolBelongBoard`, `GetMACSymbolBars`

### 地址池与测速

- 内置列表：`MainHosts`, `BrokerHosts`, `ExHosts`, `MACHosts`, `MACExHosts`
- 地址便捷函数：`MainHostAddresses`, `BrokerHostAddresses`, `ExHostAddresses`, `MACHostAddresses`, `MACExHostAddresses`
- 测速函数：`ProbeHosts`, `ProbeAddresses`, `FastestHost`, `FastestAddress`
- 客户端能力：`Client.ProbeHosts`, `Client.FastestHost`

### 试验与兼容协议

- 主站试验：`MainTodoB`, `MainTodoFDE`, `MainClient264B`, `MainClient26AC`, `MainClient26AD`, `MainClient26AE`, `MainClient26B1`
- 主站兼容：`StockListOld`, `StockFeature452`, `StockQuotesEncrypt`, `StockKLineOffset`, `StockHistoryTransactionWithTrans`
- 扩展试验：`ExListExtra`, `ExExperiment2487`, `ExExperiment2488`, `ExKLine2`, `ExMapping2562`
- TDX 商品语义：`GoodsCount`, `GoodsCategoryList`, `GoodsList`, `GoodsVarieties`, `GoodsQuote`, `GoodsQuotes`, `GoodsQuotesList`, `GoodsKLine`, `GoodsTickChart`, `GoodsChartSampling`, `GoodsHistoryTransaction`

<details>
<summary>查看更完整的接口分组</summary>

#### 主行情与通用接口

- `Connect`, `Disconnect`
- `GetExchangeAnnouncement`, `GetServerHeartbeat`, `GetAnnouncement`, `GetServerInfo`
- `GetSecurityCount`, `GetSecurityQuotes`, `GetQuotesDetail`, `GetQuotesEncrypt`
- `GetSecurityList`, `GetSecurityListOld`, `GetSecurityListRange`
- `GetKLine`, `GetSecurityBars`, `GetSecurityBarsOffset`, `GetIndexBars`
- `GetIndexMomentum`, `GetIndexInfo`, `GetMinuteTimeData`, `GetTickChart`, `GetHistoryMinuteTimeData`, `GetHistoryTickChart`
- `GetChartSampling`, `GetAuction`, `GetTopBoard`, `GetUnusual`
- `GetTransactionData`, `GetHistoryOrders`, `GetHistoryTransactionData`, `GetHistoryTransactionDataWithTrans`

#### F10、公司资料与文件

- `GetCompanyCategories`, `GetCompanyContent`, `GetFinanceInfo`, `GetXDXRInfo`, `GetCompanyInfo`
- `GetFileMeta`, `DownloadFile`, `DownloadFullFile`
- `GetBlockFile`, `GetTableFile`, `GetCSVFile`, `GetParsedBlockFile`, `GetGroupedBlockFile`

#### 扩展市场

- `ConnectEx`, `GetExServerInfo`
- `ExGetCount`, `ExGetCategoryList`, `ExGetList`, `ExGetListExtra`
- `ExGetQuotesList`, `ExGetQuote`, `ExGetQuotes`, `ExGetQuotes2`
- `ExGetKLine`, `ExGetKLine2`
- `ExGetHistoryTransaction`, `ExGetTickChart`, `ExGetHistoryTickChart`, `ExGetChartSampling`
- `ExGetBoardList`, `ExGetMapping2562`
- `ExGetFileMeta`, `ExDownloadFile`, `ExDownloadFullFile`
- `ExGetTable`, `ExGetTableDetail`

#### MAC 协议

- `NewMAC`, `NewMACEx`, `ConnectMAC`
- `GetMACBoardCount`, `GetMACBoardList`, `GetMACBoardMembers`, `GetMACBoardMembersQuotes`, `GetMACBoardMembersQuotesDynamic`, `GetMACBoardMembersQuotesDynamicWithFilter`, `GetMACSymbolQuotes`, `GetMACQuotes`, `GetMACQuotesWithDate`, `GetMACTransactions`, `GetMACTransactionsWithDate`, `GetMACAuction`, `GetMACTickCharts`, `GetMACSymbolInfo`, `GetMACCapitalFlow`, `GetMACServerInfo`, `GetMACKLineOffset`, `GetMACFileList`, `GetMACFileDownload`, `GetMACMarketMonitor`
- `GetMACSymbolBelongBoard`, `GetMACSymbolBars`
- `MACBoardCount`, `MACBoardList`, `MACBoardMembers`, `MACBoardMembersWithSort`, `MACBoardMembersQuotes`, `MACBoardMembersQuotesWithSort`, `MACBoardMembersQuotesDynamic`, `MACBoardMembersQuotesDynamicWithFilter`, `MACSymbolQuotes`, `MACQuotes`, `MACQuotesWithDate`, `MACTransactions`, `MACTransactionsWithDate`, `MACAuction`, `MACTickCharts`, `MACSymbolInfo`, `MACCapitalFlow`, `MACServerInfo`, `MACKLineOffset`, `MACFileList`, `MACDownloadFullFile`, `MACMarketMonitor`
- `MACSymbolBelongBoard`, `MACSymbolBars`
- 位图辅助：`MACFieldBit`, `MACPresetField`, `MACSortType`, `MACSortOrder`, `MACFilterType`, `MACFieldBitmap`, `MACFilterFlags`, `MACBoardMembersQuotesRequestBitmap`, `DefaultMACBoardMembersQuotesFieldBitmap`, `FullMACBoardMembersQuotesFieldBitmap`, `MACBoardMembersQuotesFieldBitmapFromBits`, `DefaultMACSymbolQuotesFieldBitmap`, `FullMACSymbolQuotesFieldBitmap`, `MACSymbolQuotesFieldBitmapFromBits`

#### ICFQS HTTP

- `NewICFQS`, `NewICFQSHot`, `WithICFQSAddress`, `WithICFQSTimeout`, `ICFQSHosts`, `ICFQSHostAddresses`
- `PostTQL`, `PostJSON`, `ICFQSFormatTables`
- `ICFQSTopicListRaw`, `ICFQSSearchTopicsRaw`, `ICFQSNewTopicsRaw`, `ICFQSHotTopicsRaw`, `ICFQSEventsRaw`, `ICFQSTopTopicsRaw`, `ICFQSTopicDetailRaw`, `ICFQSTopicKLineRaw`, `ICFQSTopicStocksRaw`
- `ICFQSTopicQuotesRaw`, `ICFQSTopicRotationRaw`, `ICFQSQuotesBatchRaw`
- `ICFQSLHBDetailRaw`, `ICFQSYYBDetailRaw`, `ICFQSYZDetailRaw`, `ICFQSMRFPRaw`, `ICFQSMRFPLatestDateRaw`

</details>

<a id="testing"></a>
## 测试

单元测试：

```bash
go test ./...
```

连接真实站点的集成测试默认跳过；如需开启：

```bash
GOTDX_INTEGRATION=1 go test ./...
```

## 相关文档

- [TdxProtocol.md](TdxProtocol.md): 协议分析笔记
- [docs/images/webviewer-screenshot.png](docs/images/webviewer-screenshot.png): Web Viewer 截图

## License

本项目使用 MIT License 开源，详见 [LICENSE](LICENSE)。
