# mcp233-game-config-excel

离线 Excel 游戏配表 MCP CLI。兼容本项目 `config233` 固定 Excel 格式，基于 `I18nTipsConfig`：第 5 行 `SERVER`、B 列起字段、数据从第 6 行开始。

- Go `1.26.0`
- stdio MCP / JSON-RPC 2.0
- 只读写本地 `.xlsx`；没有 HTTP、数据库、遥测或账号逻辑
- 内置 `I18nTipsConfig` 模板：`id:string`、`tips_CN:string`

## 安装与离线构建

发布包内含二进制。源码离线构建使用仓库随附 `vendor/`：

```powershell
go build -mod=vendor -o mcp233-game-config-excel.exe ./cmd/mcp233-game-config-excel
```

## CLI

```powershell
# 创建兼容 I18nTipsConfig 的模板
./mcp233-game-config-excel.exe init-i18n --file .\I18nTipsConfig.xlsx

# 验证固定行、字段名、id 唯一性
./mcp233-game-config-excel.exe validate --file .\I18nTipsConfig.xlsx --expected-columns id,tips_CN

# 读配置
./mcp233-game-config-excel.exe read --file .\I18nTipsConfig.xlsx

# 按 id 新增或更新。未知 SERVER 字段会拒绝写入。
./mcp233-game-config-excel.exe upsert --file .\I18nTipsConfig.xlsx --uid network_error --values '{"tips_CN":"网络异常，请重试"}'
```

## MCP 配置

`mcp233-game-config-excel` 默认启动 stdio MCP；`serve` 与 `--stdio` 等价。

```json
{
  "mcpServers": {
    "game-config-excel": {
      "command": "C:\\tools\\mcp233-game-config-excel.exe",
      "args": ["serve"]
    }
  }
}
```

工具：

| Tool | Purpose |
| --- | --- |
| `config_excel_inspect` | 查看 CLIENT / TYPE / SERVER 字段与数据行数 |
| `config_excel_validate` | 校验固定行、字段顺序、UID 唯一性 |
| `config_excel_read_rows` | 读取数据行，保留文本原值 |
| `config_excel_upsert_row` | 按 UID 新增或更新一行（写文件） |
| `config_excel_create_i18n_template` | 创建 `I18nTipsConfig` 兼容表（写文件） |

## config233 行格式

| Row | Column A | B+ |
| --- | --- | --- |
| 1 | 注释 | 字段注释 |
| 2 | 中文字段名 | 给策划阅读 |
| 3 | CLIENT | 客户端字段 |
| 4 | TYPE | `string`、`int`、`long` 等 |
| 5 | SERVER | Go `config233_column` 字段 |
| 6+ | 空 | 配置数据 |

交互式文档：[docs/index.html](docs/index.html)。
