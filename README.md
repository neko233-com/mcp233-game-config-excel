# mcp233-game-config-excel

离线 Excel 游戏配表 MCP CLI。兼容本项目 `config233` 固定 Excel 格式，基于 `I18nTipsConfig`：第 5 行 `SERVER`、B 列起字段、数据从第 6 行开始。

- Go `1.26.0`
- stdio MCP / JSON-RPC 2.0
- 只读写本地 `.xlsx`；没有 HTTP、数据库、遥测或账号逻辑
- 内置 `I18nTipsConfig` 模板：`id:string`、`tips_CN:string`

## 安装与离线构建

发布包内含二进制。源码离线构建使用仓库随附 `vendor/`：

```powershell
go build -mod=vendor -o .\\dist\\mcp233-game-config-excel.exe ./cmd/mcp233-game-config-excel
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

# 新增 CLIENT / TYPE / SERVER 列：继承相邻列样式，并统一设为 Excel 文本格式
./mcp233-game-config-excel.exe add-column --file .\FishingWeaponConfig.xlsx --name handbookIconPath --client-name handbookIconPath --display-name "图鉴显示的武器 icon" --type string --comment "图鉴显示的武器 icon" --after-column skillId

# 删除列，或检查目标列全部单元格是否保持文本格式
./mcp233-game-config-excel.exe delete-column --file .\FishingWeaponConfig.xlsx --name handbookIconPath
./mcp233-game-config-excel.exe check-column --file .\FishingWeaponConfig.xlsx --name handbookIconPath --require-text true

# 全量导出：自动识别 tips_CN / tips_EN 等列；每个语言生成独立 JSON
./mcp233-game-config-excel.exe export-i18n --file .\I18nTipsConfig.xlsx --output-dir .\i18n --format json --mode full

# 增量导出：与上一版 Excel 比较，输出 upsert 与 delete
./mcp233-game-config-excel.exe export-i18n --file .\I18nTipsConfig.xlsx --output-dir .\i18n-delta --format tsv --mode incremental --baseline-file .\I18nTipsConfig.previous.xlsx

# 递归搜索项目业务配表；支持正则并按匹配值去重
./mcp233-game-config-excel.exe search --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt --query 'icon_handbook_.*' --regex

# 默认只预览，返回逐单元格修改前后表格；确认后才加 --apply 写入
./mcp233-game-config-excel.exe replace --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt --query 'icon_handbook_old' --replacement 'icon_handbook_new'
./mcp233-game-config-excel.exe replace --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt --query 'icon_handbook_old' --replacement 'icon_handbook_new' --apply

# 检查所有 string 列是否被 Excel 转为日期或其它非文本单元格
./mcp233-game-config-excel.exe check-formats --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt

# 深度自动化校验：结构、字符串非文本风险、Excel 打开锁文件
./mcp233-game-config-excel.exe deep-validate --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt

# 缓存统计；同一 MCP 进程内复用未变化工作簿索引
./mcp233-game-config-excel.exe cache

# 对话代理先预览、确认后配置业务表目录
./mcp233-game-config-excel.exe configure-project --file .\mcp233-game-config-excel.txt --config-dirs Team-Resources/BusinessConfig
./mcp233-game-config-excel.exe configure-project --file .\mcp233-game-config-excel.txt --config-dirs Team-Resources/BusinessConfig --apply

# 项目规范全量审计：固定 id、唯一性、战斗数值 100 = 游戏显示 1.00、格式与打开锁
./mcp233-game-config-excel.exe validate-project --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt
./mcp233-game-config-excel.exe self-check --project-config D:\Code\Poko-Dev-Projects\tuanjie-project-sf\mcp233-game-config-excel.txt

# 离线自动更新：启动器自动选择已验证的最新本地版本；不从网络下载
./mcp233-game-config-excel.exe update-status
./mcp233-game-config-excel.exe update-activate --version auto --apply
./mcp233-game-config-excel.exe update-rollback --apply
```

## 多语言导出

`export-i18n` 自动提取大写语言后缀列：`tips_CN`、`tips_EN`、`name_JP`，也支持区域格式 `tips_zh_CN`。非标准列可用 `--language-columns title=CN,description=EN` 显式映射语言码。

- `full`：每个语言输出完整字典。JSON 为 `{ "key": "text" }`；CSV/TSV 为 `key,value`。
- `incremental`：必须提供 `--baseline-file`。JSON 为 `{ "upsert": {...}, "delete": [...] }`；CSV/TSV 为 `operation,key,value`。
- 同一行有多个可本地化字段时，键自动成为 `id.字段前缀`，避免 `name_CN` 与 `tips_CN` 冲突。
- 输出始终为无 BOM UTF-8，适合 Git diff 和客户端流水线。

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
| `config_excel_export_i18n` | 导出 JSON / CSV / TSV；支持全量与基线增量（写文件） |
| `config_excel_add_column` | 新增列，继承相邻单元格样式，默认使用 Excel 文本格式（写文件） |
| `config_excel_delete_column` | 按字段名删除整列（写文件） |
| `config_excel_check_column_format` | 检查列位置、文本格式与异常单元格 |
| `config_excel_search` | 递归搜索目录/指定表；支持正则、命中位置和匹配值去重 |
| `config_excel_replace` | 批量 literal/正则替换；默认预览，返回修改前后对比，`apply=true` 才写盘 |
| `config_excel_check_text_formats` | 批量检查所有 string 字段的文本格式与日期自动转换风险 |
| `config_excel_deep_validate` | 深度自动化验证：结构、格式风险、已打开 Excel 锁文件 |
| `config_excel_cache` | 查看或重置 MCP 进程内工作簿索引缓存 |
| `config_excel_configure_project` | 对话代理预览或写入业务配表目录配置；默认不写盘 |
| `config_excel_validate_project` | 全项目规则审计：固定 `id` 唯一主键、战斗数值规则、格式和锁定表 |
| `config_excel_self_check` | MCP 深度自检，返回完整项目规则审计结果，不写 Excel |
| `config_excel_update_status` | 查看离线本地版本、自动选择策略；网络更新永久关闭 |
| `config_excel_update_activate` | 预览或切换到已构建本地版本；下次 MCP 启动生效 |
| `config_excel_update_rollback` | 预览或回退到上一已构建本地版本；不删除二进制 |

## 项目目录配置

在项目根放置 `mcp233-game-config-excel.txt`，搜索、替换和格式检查可通过 `projectConfig` / `--project-config` 指定。支持 properties 与 TOML 数组：

```toml
config_dirs = ["Team-Resources/BusinessConfig"]
id_column = "id"
battle_value_scale = 100
battle_attribute_patterns = ["(?i)(attr|talent|speed)"]
```

也支持多行 properties：`config_dir=Team-Resources/BusinessConfig`。目录相对该 `.txt` 文件解析。

Excel 被打开时通常会出现同目录 `~$*.xlsx` 锁文件。MCP 会明确列出这些表：仍可搜索/检查，但任何写入操作会在开始前整体拒绝，要求先关闭 Excel，避免部分写入。

## 固定数据规范

- 可导出的 config233 表唯一主键列只能叫小写 `id`；数据行不可为空，整表不可重复。
- 列名（SERVER 或 CLIENT）含 `attr`、`talent`、`speed` 且 `TYPE=int` 的字段属于战斗属性。游戏显示值固定为 `原始值 / battle_value_scale`；默认 `100 → 1.00`。例如原始值 `235` 显示 `2.35`，禁止填写 `2.35`。
- 字段模式只匹配 SERVER/CLIENT 名。代理应先调用 `config_excel_configure_project` 预览规则，再以 `apply=true` 写入，避免误把普通数值列作为战斗属性。

## 缓存与低 token 返回

`config_excel_search` 将未变化工作簿解析结果保留在 MCP 进程内；文件大小或修改时间变化、或 MCP 写入后自动失效。搜索默认最多返回 100 个位置，仍保留完整命中数和值去重；用 `maxResults` 可调整。`config_excel_cache` 可查看命中/未命中统计或重置内存缓存。

## 离线自动更新

MCP 不联网下载、执行远程更新或覆盖正在运行的 exe。`mcp233-game-config-excel-launcher.exe` 每次启动从同目录选择最新的 `mcp233-game-config-excel-vX.Y.Z.exe`；可用 `config_excel_update_activate` 固定版本或重新设为 `auto`，并用 `config_excel_update_rollback` 安全回退。版本选择下次 MCP 重启生效。

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
