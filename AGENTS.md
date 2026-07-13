# mcp233-game-config-excel

离线 `config233` Excel MCP。只操作调用方指定的本地 `.xlsx`。

## 配表约定

- 固定使用第 1~5 行：注释、中文名、`CLIENT`、`TYPE`、`SERVER`；数据从第 6 行开始。
- 新增列必须保留相邻列的列宽、填充、边框、字体、对齐等单元格样式；默认继承右侧列，没有右侧列时继承左侧列。
- 新增字符串列必须显式设为 Excel 文本格式（`@` / 49）；不要依赖 Excel 自动推断。
- 新增、删除、检查列都必须按真实 Excel 列位置处理，不能因 `SERVER` 空列压缩索引，避免写错 CLIENT 配置列。
- 修改列后执行列格式检查；需要文本字段时必须要求 `isTextFormat=true` 且无问题。

## 工程约定

- Go 代码运行 `gofmt`、`go test -mod=vendor ./...`；离线构建必须使用 `vendor/`。
- MCP 只用 stdio / JSON-RPC；禁止引入网络、数据库、遥测和账号逻辑。
- 发布前同步 README、`docs/index.html`、MCP 工具数量和 CLI 示例。
