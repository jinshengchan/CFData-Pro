// Package pro 是 guanfang-youxuan 扫描引擎在 CFData-Pro 中的形态。
//
// 与上游不同：这里没有 gomobile 导出层，对外是干净的 Go API ——
// 调用方传 context.Context 做取消、传 ProgressFunc 收进度，
// 拿回结构化的 *ScanResult。包内一次只允许跑一个扫描（scanMu 串行化），
// 因为引擎内部有包级全局状态（取消上下文、测速诊断、数据目录等）。
package pro

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config 一次增强优选扫描的全部参数，对应上游 GetIPs 的 11 个参数。
type Config struct {
	V4           bool   // true=IPv4, false=IPv6
	UseTLS       bool   // 是否启用 TLS 握手
	Bandwidth    int    // 期望带宽 Mbps，0 则用默认 1
	Ports        string // 要测的端口 CSV（如 "443,2053"），空则只测默认端口
	Countries    string // 允许的落地国家代码 CSV（如 "HK,JP"），空则不限
	SNI          string // 自定义 SNI/Host，空则用默认
	WantCount    int    // 输出几个结果，收敛到 1/5/10 之一
	SpeedSeconds int    // 正式测速时长（秒），收敛到 5/10/15 之一，0 用默认 5
	SpeedSource  string // 测速源标识（见 SpeedSources），空或 "auto" 按运营商自动挑选
	SpeedURL     string // 手动测速地址，非空时优先于 SpeedSource
	IPRanges     string // 指定 IP 段原文，空则用官方列表
	CacheDir     string // 数据缓存目录，空则用系统缓存目录
}

// ProgressFunc 进度回调。调用在引擎内部 goroutine，直接转发到
// CFData-Pro 的 WebSocket 推送即可（实现方注意线程安全）。
// 第二个参数是触发本次回调时的结构化进度快照（phase/current/total），
// 由 fireProgressHook 在调用回调前读取，保证文案和数字是同一时刻的。
type ProgressFunc func(msg string, st ProgressState)

const libVersion = "1.22"

// Version 返回引擎版本号，供界面显示。
func Version() string { return libVersion }

// ResultCounts 返回允许的输出数量 CSV（如 "1,5,10"），供界面构建选项。
func ResultCounts() string { return joinInts(allowedResultCounts) }

// SpeedSecondsList 返回允许的测速时长档位 CSV（如 "5,10,15"），供界面构建选项。
func SpeedSecondsList() string { return joinInts(allowedSpeedSeconds) }

// HTTPPorts 返回明文模式可选端口的 CSV，供界面构建选项。
func HTTPPorts() string { return joinInts(cfHTTPPorts) }

// HTTPSPorts 返回 TLS 模式可选端口的 CSV，供界面构建选项。
func HTTPSPorts() string { return joinInts(cfHTTPSPorts) }

// SpeedSourceOptions 返回测速源「标识:中文名」列表，供界面构建下拉框。
func SpeedSourceOptions() []string {
	srcs := strings.Split(SpeedSources(), ",")
	labels := strings.Split(SpeedSourceLabels(), ",")
	var out []string
	for i, s := range srcs {
		label := s
		if i < len(labels) {
			label = labels[i]
		}
		out = append(out, s+":"+label)
	}
	return out
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}

// ScanResult 扫描结果（字段与上游 better.ScanResult 一致）。
type ScanResult struct {
	IP string `json:"ip"`
	// Port 实测通过的端口。必须输出：只给 IP 不给端口，
	// 用户拿去接跑在 2053 的节点而工具验证的是 443，就会报 closed pipe。
	Port          int    `json:"port"`
	Address       string `json:"address"`
	Bandwidth     int    `json:"bandwidth"`     // 期望带宽 Mbps
	RealBandwidth int    `json:"realBandwidth"` // 实测带宽 Mbps
	MaxSpeed      int    `json:"maxSpeed"`      // 峰值速度 kB/s
	LatencyMs     int    `json:"latencyMs"`
	DataCenter    string `json:"dataCenter"`
	Country       string `json:"country"`
	Elapsed       int    `json:"elapsed"` // 总计用时 秒
	// Cancelled 区分「用户主动取消」与「没找到」。
	Cancelled bool `json:"cancelled"`
	// BelowTarget 有结果但未达到期望带宽，IP 仍然可用。
	BelowTarget bool   `json:"belowTarget"`
	Error       string `json:"error"`

	// Results 全部结果，按实测速度降序。
	Results []ScanItem `json:"results"`
	// WantCount 实际生效的输出数量（已收敛到 1/5/10 之一）。
	WantCount int `json:"wantCount"`
	// FoundCount 实际达标的结果个数。
	FoundCount int `json:"foundCount"`

	SpeedSeconds      int    `json:"speedSeconds"`
	SpeedTarget       string `json:"speedTarget"`
	SpeedSourceISP    string `json:"speedSourceISP"`
	UsingCustomRanges bool   `json:"usingCustomRanges"`
	// SpeedHint 测速层面的诊断提示（地址全部 404、文件太小等）。
	SpeedHint string `json:"speedHint"`
}

// ScanItem 单条优选结果。
type ScanItem struct {
	IP            string `json:"ip"`
	Port          int    `json:"port"`
	Address       string `json:"address"`
	RealBandwidth int    `json:"realBandwidth"`
	MaxSpeed      int    `json:"maxSpeed"`
	LatencyMs     int    `json:"latencyMs"`
	DataCenter    string `json:"dataCenter"`
	Country       string `json:"country"`
}

// scanMu 引擎全局串行锁。引擎内部有包级全局状态（取消上下文、
// 测速诊断、数据目录、进度字符串），不支持并发扫描。
var scanMu sync.Mutex

// progressHook 当前扫描的进度回调，由 Scan 设置、结束时清空。
// setProgress / setScanProgress 在写全局进度字符串的同时调用它。
var (
	progressHook   ProgressFunc
	progressHookMu sync.Mutex
)

func setProgressHook(fn ProgressFunc) {
	progressHookMu.Lock()
	progressHook = fn
	progressHookMu.Unlock()
}

func fireProgressHook(s string) {
	progressHookMu.Lock()
	fn := progressHook
	progressHookMu.Unlock()
	if fn != nil {
		fn(s, GetProgressState())
	}
}

// ProgressState 结构化进度，供前端画"扫描中 x/y（p%)"进度条。
// 关键扫描节点用 setProgressState 同步快照，fireProgressHook 在触发回调时
// 把快照一起带出去，保证文案和数字是同一时刻的。
type ProgressState struct {
	Phase   string `json:"phase"`   // init | rtt | recon | speed | done
	Current int    `json:"current"` // 已完成数
	Total   int    `json:"total"`   // 总数
}

var (
	progressStateMu sync.Mutex
	progressState   ProgressState
)

// setProgressState 记录结构化进度快照。
func setProgressState(phase string, current, total int) {
	progressStateMu.Lock()
	progressState = ProgressState{Phase: phase, Current: current, Total: total}
	progressStateMu.Unlock()
}

// GetProgressState 返回最近一次结构化进度快照。
func GetProgressState() ProgressState {
	progressStateMu.Lock()
	defer progressStateMu.Unlock()
	return progressState
}

// scanStarted 本次扫描启动时刻，用于进度文案附「已用时间」。
var scanStarted time.Time

func markScanStart() {
	progressMu.Lock()
	scanStarted = time.Now()
	progressMu.Unlock()
}

func setProgress(s string) {
	progressMu.Lock()
	progress = s
	progressMu.Unlock()
	fireProgressHook(s)
}

func setScanProgress(s string) {
	progressMu.Lock()
	if !scanStarted.IsZero() {
		if elapsed := int(time.Since(scanStarted).Seconds()); elapsed > 0 {
			s = fmt.Sprintf("%s（已用 %d 秒）", s, elapsed)
		}
	}
	progress = s
	progressMu.Unlock()
	fireProgressHook(s)
}

// useContext 把调用方的 context 设为本次扫描的取消上下文。
// CFData-Pro 的任务取消（WebSocket 断开 / 用户点停止）会 cancel 这个 ctx，
// 引擎内的 isCancelled() 检查随之生效，中断所有网络操作。
func useContext(ctx context.Context) {
	cancelMu.Lock()
	defer cancelMu.Unlock()
	cancelCtx, cancelCancel = context.WithCancel(ctx)
	taskRequested = false
}

func isCancelled() bool {
	cancelMu.Lock()
	defer cancelMu.Unlock()
	if cancelCtx == nil {
		return false
	}
	select {
	case <-cancelCtx.Done():
		return true
	default:
		return false
	}
}

// recoverToMsg 把 panic 转成一句人话。扫描是最长最复杂的路径，
// 炸了也要能说清为什么，而不是把整个服务带崩。
func recoverToMsg(what string, dst *string) {
	if r := recover(); r != nil {
		*dst = fmt.Sprintf("%s时发生内部错误：%v", what, r)
		fmt.Printf("panic in %s: %v\n%s\n", what, r, debug.Stack())
	}
}

// Scan 运行一次增强优选扫描。阻塞调用，调用方应在后台 goroutine 中执行。
//
// 进度通过 onProgress 回调实时推送；取消通过 ctx（ctx.Done 后引擎会尽快
// 收尾并返回 Cancelled=true 的结果）。
func Scan(ctx context.Context, cfg Config, onProgress ProgressFunc) (result *ScanResult) {
	if !scanMu.TryLock() {
		return &ScanResult{Error: "增强引擎已有扫描在运行，请等待完成后再试"}
	}
	defer scanMu.Unlock()

	// panic 兜底：不能让一次扫描的 panic 拖垮整个 CFData-Pro 服务
	defer func() {
		var msg string
		recoverToMsg("增强扫描", &msg)
		if msg != "" {
			setProgress(msg)
			result = &ScanResult{Error: msg}
		}
	}()

	useContext(ctx)
	// 包装进度回调：记住最后一条进度消息。数据文件缺失/下载失败时，
	// 引擎只通过进度消息说出真实原因（"下载 IP 列表失败: ..."），
	// 收尾时要把它带进结果，否则用户只会看到"未找到可用 IP"，
	// 完全不知道是数据没下来。
	var lastProgressMsg string
	setProgressHook(func(msg string, _ ProgressState) {
		lastProgressMsg = msg
		if onProgress != nil {
			onProgress(msg, GetProgressState())
		}
	})
	defer setProgressHook(nil)
	setProgressState("init", 0, 0)
	markScanStart()
	setProgress("正在初始化...")

	// 数据缓存目录
	dir := cfg.CacheDir
	if dir == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			dir = filepath.Join(cache, "cfdata-pro")
		} else {
			dir = "cfdata-pro-cache"
		}
	}
	dataDir = dir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return &ScanResult{Error: "创建缓存目录失败：" + err.Error()}
	}
	// 内置数据种子：首次运行或数据文件丢失时先用内置版本，
	// 避免"点开始就完成"（0 子网 0 轮 0 秒）
	ensureSeeded()

	// 指定 IP 段要在任何耗时操作之前解析：填错了立刻报错，
	// 不要让用户等到下载完数据、跑完侦察才看到「格式不对」
	ranges, err := parseCustomRanges(cfg.IPRanges)
	if err != nil {
		msg := "IP 段填写有误：" + err.Error()
		setProgress(msg)
		return &ScanResult{Error: msg}
	}

	bandwidth := cfg.Bandwidth
	if bandwidth <= 0 {
		bandwidth = 1
	}
	// 带宽上限保护：填个 999999 除了让扫描永远达不到目标、
	// 白跑满 10 轮之外没有任何意义
	if bandwidth > maxBandwidthMbps {
		bandwidth = maxBandwidthMbps
	}

	filter := scanFilter{
		Ports:     parsePortsCSV(cfg.Ports, cfg.UseTLS),
		Countries: parseCountriesCSV(cfg.Countries),
	}

	// 转为 kB/s
	speedTarget := bandwidth * 128

	ipType := 4
	if !cfg.V4 {
		ipType = 6
	}

	startTime := timeNow()

	out := cloudflareTest(ipType, cfg.UseTLS, defaultTaskNum, speedTarget, filter,
		strings.TrimSpace(cfg.SNI), normalizeResultCount(cfg.WantCount),
		cfg.SpeedSeconds, cfg.SpeedSource, cfg.SpeedURL, ranges)

	realBandwidth := out.MaxSpeed / 128
	elapsed := int(timeSince(startTime).Seconds())

	result = &ScanResult{
		IP:                out.IP,
		Port:              out.Port,
		Bandwidth:         bandwidth,
		RealBandwidth:     realBandwidth,
		MaxSpeed:          out.MaxSpeed,
		LatencyMs:         out.LatencyMs,
		DataCenter:        out.DataCenter,
		Country:           out.Country,
		Elapsed:           elapsed,
		WantCount:         normalizeResultCount(cfg.WantCount),
		SpeedSeconds:      out.SpeedSeconds,
		SpeedTarget:       out.SpeedTarget,
		SpeedSourceISP:    out.SpeedSourceISP,
		SpeedHint:         out.SpeedHint,
		UsingCustomRanges: out.UsingCustomRanges,
	}
	if out.IP != "" && out.Port > 0 {
		result.Address = net.JoinHostPort(out.IP, strconv.Itoa(out.Port))
	}
	for _, it := range out.Results {
		item := ScanItem{
			IP:            it.IP,
			Port:          it.Port,
			RealBandwidth: it.MaxSpeed / 128,
			MaxSpeed:      it.MaxSpeed,
			LatencyMs:     it.LatencyMs,
			DataCenter:    it.DataCenter,
			Country:       it.Country,
		}
		if it.IP != "" && it.Port > 0 {
			item.Address = net.JoinHostPort(it.IP, strconv.Itoa(it.Port))
		}
		result.Results = append(result.Results, item)
		if it.MaxSpeed >= speedTarget {
			result.FoundCount++
		}
	}

	switch {
	case isCancelled():
		// 用户主动取消：不能报"未找到"，否则界面会误导用户
		result.Cancelled = true
		result.IP = ""
		result.Address = ""
		result.Results = nil
		result.FoundCount = 0
		result.Error = "扫描已取消"
		setProgress("扫描已取消")

	case out.IP == "":
		scope := ""
		if len(filter.Countries) > 0 {
			scope = "（限定地区：" + strings.ToUpper(cfg.Countries) + "）"
		}
		if out.PoolExhausted {
			result.Error = fmt.Sprintf("%d 个子网已全部测过，没有一个 IP 符合条件%s（用时 %d 秒）",
				out.PoolSize, scope, elapsed)
		} else {
			result.Error = fmt.Sprintf("已测试 %d 个子网（共 %d 轮），未找到可用 IP%s（用时 %d 秒）",
				out.Tested, out.RoundsRun, scope, elapsed)
		}
		if len(filter.Countries) > 0 {
			result.Error += "。官方 IP 的落地地区取决于运营商线路，缩小地区会大幅降低命中率，可放宽后重试"
		}
		if out.UsingCustomRanges {
			result.Error += "。这些是你指定的 IP 段，请确认它们确实是 Cloudflare 的段且在你的网络下可达；也可以清空 IP 段改用官方列表"
		}
		// 一个子网都没测就结束了：大概率是数据文件没下来。引擎的真实原因
		// 只写在进度消息里，这里把它摆到台面上，而不是让用户对着
		// "未找到可用 IP" 猜。
		if out.PoolSize == 0 && out.Tested == 0 && isDataFailureMsg(lastProgressMsg) {
			result.Error = "扫描未能开始：" + lastProgressMsg
		}
		setProgress(fmt.Sprintf("扫描结束，用时 %d 秒", elapsed))

	case out.BelowTarget:
		result.BelowTarget = true
		if out.PoolExhausted {
			result.Error = fmt.Sprintf("%d 个子网已全部测过，均未达到 %d Mbps，返回最佳结果 %d Mbps",
				out.PoolSize, bandwidth, realBandwidth)
		} else {
			result.Error = fmt.Sprintf("已测试 %d 个子网（共 %d 轮）未达到 %d Mbps，返回最佳结果 %d Mbps",
				out.Tested, out.RoundsRun, bandwidth, realBandwidth)
		}
		if result.WantCount > 1 {
			result.Error += fmt.Sprintf("；共返回 %d 个结果", len(result.Results))
		}
		setProgress(fmt.Sprintf("扫描完成，用时 %d 秒", elapsed))

	case result.WantCount > 1 && result.FoundCount < result.WantCount:
		if out.PoolExhausted {
			result.Error = fmt.Sprintf("%d 个子网已全部测过，只找到 %d 个达标 IP（要求 %d 个）",
				out.PoolSize, result.FoundCount, result.WantCount)
		} else {
			result.Error = fmt.Sprintf("已测试 %d 个子网（共 %d 轮），只找到 %d 个达标 IP（要求 %d 个）",
				out.Tested, out.RoundsRun, result.FoundCount, result.WantCount)
		}
		setProgress(fmt.Sprintf("扫描完成，找到 %d 个，用时 %d 秒", result.FoundCount, elapsed))

	default:
		if result.WantCount > 1 {
			setProgress(fmt.Sprintf("扫描完成，找到 %d 个，用时 %d 秒", result.FoundCount, elapsed))
		} else {
			setProgress(fmt.Sprintf("扫描完成，用时 %d 秒", elapsed))
		}
	}

	// 测速诊断挂在 Error 末尾
	if result.SpeedHint != "" {
		if result.Error == "" {
			result.Error = result.SpeedHint
		} else {
			result.Error += "。" + result.SpeedHint
		}
	}

	return result
}

// isDataFailureMsg 判断一条进度消息是否在说数据文件出了问题
// （下载失败、读取失败、列表为空）。用于收尾时把真实原因摆到台面上。
func isDataFailureMsg(msg string) bool {
	return strings.Contains(msg, "失败") || strings.Contains(msg, "为空")
}

// PreviewResult IP 段预检结果（结构体版，供 WS 直接序列化）。
type PreviewResult struct {
	OK        bool   `json:"ok"`
	V4        int    `json:"v4"`
	V6        int    `json:"v6"`
	Truncated bool   `json:"truncated"`
	Summary   string `json:"summary"`
	Error     string `json:"error,omitempty"`
}

// PreviewIPRanges 预检用户填写的 IP 段。界面在输入时调用，
// 即时显示「已识别 N 个子网」或具体错在哪一条，不用等一轮扫描。
func PreviewIPRanges(raw string) PreviewResult {
	var msg string
	defer func() {
		recoverToMsg("预检 IP 段", &msg)
	}()

	ranges, err := parseCustomRanges(raw)
	if err != nil {
		return PreviewResult{OK: false, Error: err.Error()}
	}

	p := PreviewResult{
		OK:        true,
		V4:        len(ranges.V4),
		V6:        len(ranges.V6),
		Truncated: ranges.Truncated,
	}
	switch {
	case ranges.empty():
		p.Summary = "未指定，将使用官方 IP 段列表"
	default:
		var parts []string
		if p.V4 > 0 {
			parts = append(parts, fmt.Sprintf("%d 个 IPv4 /%d 子网", p.V4, customV4SplitPrefix))
		}
		if p.V6 > 0 {
			parts = append(parts, fmt.Sprintf("%d 个 IPv6 /%d 子网", p.V6, customV6SplitPrefix))
		}
		p.Summary = "已识别 " + strings.Join(parts, "、")
		if p.Truncated {
			p.Summary += fmt.Sprintf("（超出上限，已截断到 %d 个）", maxCustomSubnets)
		}
	}
	if msg != "" {
		return PreviewResult{OK: false, Error: msg}
	}
	return p
}

// PreviewIPRangesJSON 预检结果的 JSON 字符串版（兼容旧调用习惯）。
func PreviewIPRangesJSON(raw string) string {
	b, _ := json.Marshal(PreviewIPRanges(raw))
	return string(b)
}

// MaxIPRangeSubnets 返回指定 IP 段展开后的子网数量上限，供界面提示。
func MaxIPRangeSubnets() int { return maxCustomSubnets }

// dataFiles 需要下载与缓存的数据文件。
var dataFiles = []string{"locations.json", "ips-v4.txt", "ips-v6.txt", "url.txt"}

// UpdateDataFiles 重新下载所有数据文件。返回空字符串表示成功，
// 否则是给用户看的失败原因。阻塞调用，调用方应在后台执行。
func UpdateDataFiles(ctx context.Context, cacheDir string) string {
	if !scanMu.TryLock() {
		return "增强引擎正在扫描中，请等待完成后再更新数据"
	}
	defer scanMu.Unlock()

	var msg string
	defer func() { recoverToMsg("更新数据", &msg) }()

	useContext(ctx)
	if cacheDir != "" {
		dataDir = cacheDir
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "创建缓存目录失败：" + err.Error()
	}

	setProgress("正在更新数据...")
	// 先用内置种子兜底：任何情况下缓存不为空，下载失败也不丢数据。
	// （之前是先删文件再下载，下载失败就导致缓存全空、扫描不可用）
	ensureSeeded()
	// 把文件时间戳改旧，强制 downloadAllData 重新下载；
	// 下载失败时它原有的「暂用本地副本」逻辑会保留旧文件
	old := time.Now().Add(-dataMaxAge - time.Hour)
	for _, f := range dataFiles {
		fp := dataPath(f)
		if fileExists(fp) {
			_ = os.Chtimes(fp, old, old)
		}
	}
	initLocations(true)

	if isCancelled() {
		setProgress("数据更新已取消")
		return "数据更新已取消"
	}
	if msg != "" {
		setProgress(msg)
		return msg
	}
	// 下载失败时文件仍在（种子或旧副本）：如实告知，不要报"缺少文件"
	var missing, stale []string
	for _, f := range dataFiles {
		fp := dataPath(f)
		if !fileExists(fp) {
			missing = append(missing, f)
		} else if !dataFresh(fp) {
			stale = append(stale, f)
		}
	}
	if len(missing) > 0 {
		m := fmt.Sprintf("数据更新未完成，缺少 %s（检查网络后重试）",
			strings.Join(missing, "、"))
		setProgress(m)
		return m
	}
	if len(stale) > 0 {
		m := fmt.Sprintf("数据下载失败（%s），已保留本地数据，扫描可正常使用",
			strings.Join(stale, "、"))
		setProgress(m)
		return m
	}
	setProgress("数据更新完成")
	return ""
}

// ClearCache 清除引擎缓存的数据文件。
func ClearCache(cacheDir string) {
	if cacheDir != "" {
		dataDir = cacheDir
	}
	setProgress("正在清除缓存...")
	for _, f := range dataFiles {
		removeFile(dataPath(f))
	}
	setProgress("缓存已清除")
}
