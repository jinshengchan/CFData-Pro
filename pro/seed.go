package pro

import (
	_ "embed"
	"os"
	"path/filepath"
)

// 内置数据文件兜底。
//
// 为什么需要：首次运行时引擎要从 baipiao.eu.org 下载 ips-v4.txt 等数据
// 文件。在手机（APK）等网络环境下，这个下载可能瞬间失败（DNS/代理），
// 结果就是 IP 列表为空、扫描 0 秒"完成"、0 个子网 —— 用户看到的就是
// "点开始就完成了"，完全不知道是数据没下来。
//
// 有了内置种子：文件缺失时先用内置版本，扫描可以离线先跑起来；
// downloadAllData 原有的 24h 过期刷新逻辑不变，网络可用时会自动更新。

//go:embed seeddata/ips-v4.txt
var seedIPsV4 string

//go:embed seeddata/ips-v6.txt
var seedIPsV6 string

//go:embed seeddata/locations.json
var seedLocations string

//go:embed seeddata/url.txt
var seedURL string

// ensureSeeded 把缺失的数据文件用内置种子补上。只补缺失的，
// 不覆盖已有文件（已有文件的 24h 刷新逻辑在 downloadAllData 里）。
func ensureSeeded() {
	seeds := map[string]string{
		"ips-v4.txt":     seedIPsV4,
		"ips-v6.txt":     seedIPsV6,
		"locations.json": seedLocations,
		"url.txt":        seedURL,
	}
	for name, content := range seeds {
		if content == "" || fileExists(dataPath(name)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dataPath(name)), 0o755); err != nil {
			continue
		}
		// 种子写入失败也不阻断：后面的流程会按原逻辑报错，
		// 错误信息里会说清是哪个文件缺失
		_ = saveToFile(dataPath(name), content)
	}
}
