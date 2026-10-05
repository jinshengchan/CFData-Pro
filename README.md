# CFData-Pro

Cloudflare IP 优选工具：**CFData-WEB** 的平台 + **guanfang-youxuan** 的带宽优选引擎，二合一。

- Web 界面（本地服务，默认 `http://localhost:13335`）
- CLI 模式（`./cfdata-pro -cli`）
- 三种扫描模式：☁️ 官方优选（延迟优选）/ 📂 非标优选 / ⚡ 增强优选（带宽优选）

## 增强优选（新增）

顶部切换到 **⚡ 增强优选**，这是从 guanfang-youxuan 合并过来的扫描引擎：

- **按带宽选最优，而不是按延迟** —— 所有候选测完再按实测速度排序，速度取 EWMA 平均（抗 TCP 慢启动突发虚高）
- **指定 IP 段** —— 支持 CIDR / 单 IP / IPv4 起止范围（`104.16.0.0/13`、`104.16.0.0-104.16.3.255`），输入框边打边预检，填错立刻报错不悄悄回落
- **端口全覆盖** —— 明文 80/8080/8880/2052/2082/2086/2095，TLS 443/2053/2083/2087/2096/8443，逐端口实测
- **落地地区筛选** —— 从 `CF-RAY` 反查机房，先做廉价地区侦察再优选（注意：官方 IP 落地机房取决于运营商线路，筛得越窄耗时越长）
- **回源校验** —— 填了节点域名（SNI）后，拦截 52x 回源不通的 IP
- **输出 1 / 5 / 10 个** —— 要备选地址不用重复扫
- **测速源按运营商自动挑** —— 自动探测 ASN，移动线路换用移动友好源；也可手动指定测速地址测真实回源链路
- **测速诊断** —— 测速地址 404 / 文件太小 / 429 限流都有中文提示

## 原有功能（保留）

- 官方优选：扫描 CF IPv4/IPv6，按数据中心做详细延迟测试（一键/继续测速）
- 非标优选：上传 txt/csv 或填网络 URL，测任意 IP/域名+端口
- 结果导出：TXT/CSV，字段自选、筛选、排序
- 一键上传到 GitHub / edgetunnel
- IPv4 地址库精简、代理地区检测、Web 登录认证、后台任务（断线重连跟随）
- CLI 全功能（参数与原版一致，`./cfdata-pro -h` 查看）

## 构建

```bash
go build -o cfdata-pro .
```

需要 Go 1.25+。前端 `index.html` 通过 `go:embed` 打进二进制，无需额外步骤。

## 项目结构

```
main.go / server.go / session.go   Web 服务 + WebSocket 任务调度
tasks_official.go / tasks_nsb.go   官方优选 / 非标优选任务
tasks_pro.go                       增强优选任务（调用 pro 引擎）
tasks_speed.go / speed_url.go      测速实现与测速源
cli.go                             CLI 模式
pro/                               增强优选引擎（带宽优选核心）
  pro.go          对外 Go API（Scan / PreviewIPRanges / 选项元数据）
  scan.go         扫描引擎：RTT → 地区侦察 → 两阶段测速 → Top-N
  recon.go        廉价地区侦察（CF-RAY 反查机房）
  speedsource.go  按运营商选择测速源
  custom_range.go 指定 IP 段解析器
index.html                         前端（单文件，embed）
```

## 许可

[GPL-3.0](LICENSE)。

本项目合并了以下两个上游项目的代码与思路：

- [PoemMisty/CFData-WEB](https://github.com/PoemMisty/CFData-WEB)（GPL-3.0）—— 平台主体：WebSocket 服务、任务调度、官方/非标优选、导出、CLI
- [liyanan2016-stack/guanfang-youxuan](https://github.com/liyanan2016-stack/guanfang-youxuan)（MIT）—— `pro/` 引擎：带宽优选扫描、地区侦察、指定 IP 段、回源校验

按 GPL-3.0 要求，合并后的整体作品沿用 GPL-3.0 许可。
