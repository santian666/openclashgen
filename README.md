# OpenClash 配置生成器

一个 Windows 本地网页工具，用于把 VLESS、VMess、Trojan、AnyTLS、Shadowsocks、SOCKS5 等节点分享链接转换为 OpenClash `proxies` 节点代码，并按设备 IP 自动生成策略组、规则和完整 YAML 配置。

项目不依赖公网订阅转换服务，内置 SubConverter-Extended v1.1.26，节点转换全部在本机完成。

## 下载成品

直接下载 Windows 单文件 EXE：[openclashgen.exe](https://github.com/santian666/openclashgen/releases/download/v0.0.1/openclashgen.exe)

也可以进入 [Releases](https://github.com/santian666/openclashgen/releases) 页面查看版本和发布说明。

## 主要功能

- 本地节点转换：调用内置 SubConverter-Extended，服务地址为 `127.0.0.1:25500`。
- 多协议支持：VLESS、VMess、Trojan、AnyTLS、SS、SSR、SOCKS5 等常见分享链接。
- 直连模式：输入节点后直接生成 OpenClash `proxies` 节点段。
- 链式模式：输入一个前置节点和多个普通节点，普通节点自动添加 `dialer-proxy`，值为前置节点名称。
- SOCKS5 简写格式：支持 `IP:端口:账号:密码`，节点名称自动使用 IP 地址。
- 自动读取节点名称：优先使用分享链接中 `#` 后的名称，未命名节点由转换核心生成。
- 自动生成 `pr: &pr`：只列出节点名称，不把节点详细信息混入名称列表。
- gfwairport 编号：输入编号后自动生成 `gfwairport1`、`gfwairport2` 等代理提供者名称。
- 策略数量：默认生成 20 个策略组，可自行修改。
- 设备 IP 规则：输入起始网段后按策略数量自动生成连续 `/24` 规则和结束网段。
- 完整配置生成：合并代理提供者、节点、策略组、设备 IP 规则、固定规则源和 DNS 配置。
- 复制与导出：一键复制完整配置，或导出 `openclash-config.yaml`。
- 自动清空结果：删除节点输入后，旧的节点名称和转换结果自动清空。
- 单文件 EXE：网页和转换核心嵌入程序，无需安装 Go、Node.js 或单独配置转换器。
- 自动退出：关闭网页后停止本地转换核心并退出主程序，刷新页面不会误退出。

## 直连与链式模式

直连模式是默认模式，节点直接连接目标服务器，适合普通 VLESS、VMess、Trojan、AnyTLS、SS 和 SOCKS5 节点。

链式模式需要填写一个前置节点。程序会把普通节点转换为带 `dialer-proxy: 前置节点名称` 的 OpenClash 节点。前置节点和普通节点会一起生成，不需要单独转换前置节点；前置节点只能输入一个，普通节点可以输入多个。

## 输入格式

普通协议每行输入一个完整分享链接，例如 `vless://...`、`vmess://...`、`trojan://...`、`anytls://...` 或 `ss://...`。多行节点会一次性转换。

SOCKS5 使用以下格式，每行一个节点：

`IP:端口:账号:密码`

例如：`192.168.1.10:1080:user123:pass123`。生成结果中的节点名称为 `192.168.1.10`。

VLESS 等链接如果包含 `#美国-204.42.251.66` 这样的名称，转换结果会保持该名称，不主动追加策略数量或其他后缀。

## 使用流程

1. 启动 `openclashgen.exe`。
2. 输入 `gfwairport` 编号，默认编号为 1。
3. 选择直连模式或链式模式；链式模式额外填写一个前置节点。
4. 在“输入节点协议”中粘贴节点，每行一个。
5. 点击“生成节点代码”，检查右侧节点名称和 `proxies` 结果。
6. 设置策略数量和设备 IP 起始网段，例如数量为 20、起始网段为 `10.0.1.0/24`。
7. 点击“转换并生成完整配置”。
8. 检查代理提供者、节点名称、规则和 DNS 配置。
9. 点击“复制配置”或“导出 YAML”，将配置导入 OpenClash。

## 生成配置内容

完整配置会生成 `proxy-providers` 和 `proxy-providers-config`，其中代理提供者名称根据编号显示为 `gfwairport1` 等；机场链接位置保留为空并带有中文提示，方便用户手动填写。

节点名称会写入 `pr: &pr` 的 `proxies` 列表，节点详细信息则单独写在下面的 `proxies:` 段中。设备 IP 规则会从起始网段开始，按策略数量递增，例如 `10.0.1.0/24`、`10.0.2.0/24`。

固定的规则源、策略组和 DNS 配置由程序模板生成，界面不会提供修改入口，避免误改核心配置。

## 本地运行原理

程序启动后从 EXE 内置资源释放 SubConverter-Extended v1.1.26 到用户本地缓存目录，并启动本地转换服务监听 `127.0.0.1:25500`。前端把多行节点按 subweb 规则合并后提交给本地 `/sub?target=clash` 接口，只提取转换结果中的 `proxies` 节点段，再写入 OpenClash 模板。

节点内容不会发送到公网订阅转换服务；转换所需核心文件随程序发布。页面关闭后程序会停止转换核心并退出。

## 目录结构

```text
.
├─ main.go                         # HTTP 服务、转换核心启动和嵌入资源释放
├─ index.html                      # 本地网页、配置模板和交互逻辑
├─ go.mod                          # Go 模块配置
├─ openclashgen.exe                # Windows 单文件发布程序
└─ converter/extended-v1.1.26/     # 内置 SubConverter-Extended 资源
```

## 开发与构建

开发环境需要 Go 1.22 或兼容版本。源码修改后可执行：

```powershell
gofmt -w main.go
go test .
node --check index.html
```

构建 Windows 单文件 GUI EXE：

```powershell
go build -buildvcs=false -ldflags="-s -w -H windowsgui" -o openclashgen.exe .
```

构建后的 EXE 会把网页和 SubConverter-Extended 一起嵌入，用户无需安装 Go、Node.js 或其他运行环境。

## 常见问题

### 提示本地节点转换器不可用

请重新启动程序。程序会自动启动内置转换核心并检查 `127.0.0.1:25500`。如果仍然失败，请查看 `%TEMP%\\openclashgen-error.log`。

### VLESS、VMess、Trojan 或 AnyTLS 无法转换

确认分享链接完整、没有被聊天软件截断；每行只放一个节点；协议名称和参数保持原始格式。程序使用 SubConverter-Extended v1.1.26 进行转换。

### 链式模式报前置节点错误

前置节点必须只输入一个，并且转换后能读取到节点名称。普通节点可以输入多个。

### 结束网段为空

检查起始网段是否是类似 `10.0.1.0` 的地址，并确认策略数量没有超过当前第三段网段的可用范围。

## 安全提示

- 不要把包含节点密码、UUID、私钥或订阅链接的完整配置上传到公开仓库。
- 导出的 YAML 可能包含敏感节点信息，请妥善保存。
- 本项目只在本机调用转换核心，不代表生成的节点本身安全或可用。

## 第三方组件

本项目使用 Go 标准库和原生 HTML/CSS/JavaScript，并集成 SubConverter-Extended v1.1.26 作为节点转换核心。第三方组件请遵守其各自的许可证和使用条款。
