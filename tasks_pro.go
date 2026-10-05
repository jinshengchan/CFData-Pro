package main

import (
	"context"
	"os"
	"strconv"
	"strings"

	"cfdatapro/pro"
)

// runProTask 执行增强优选扫描（guanfang-youxuan 引擎）。
//
// 与官方优选（runOfficialTask）的区别：按带宽而非延迟选最优（EWMA），
// 支持指定 IP 段、多端口、落地地区筛选、回源 52x 校验、输出 1/5/10 个结果。
// 进度经 pro_progress 实时推送，结果经 pro_complete 一次性返回。
func runProTask(ctx context.Context, session *appSession, p startProTaskRequest) {
	v4 := true
	if p.IPType == 6 {
		v4 = false
	}

	// 与主程序共用工作目录缓存：ips-v4.txt / locations.json 等
	// 两边从同一个 URL 下载、格式一致，避免重复下载
	cacheDir := "."
	if wd, err := os.Getwd(); err == nil {
		cacheDir = wd
	}

	cfg := pro.Config{
		V4:           v4,
		UseTLS:       p.UseTLS,
		Bandwidth:    p.Bandwidth,
		Ports:        p.Ports,
		Countries:    p.Countries,
		SNI:          strings.TrimSpace(p.SNI),
		WantCount:    p.WantCount,
		SpeedSeconds: p.SpeedSeconds,
		SpeedSource:  strings.TrimSpace(p.SpeedSource),
		SpeedURL:     strings.TrimSpace(p.SpeedURL),
		IPRanges:     p.IPRanges,
		CacheDir:     cacheDir,
	}

	session.sendWSMessage("log", "⚡ 增强优选引擎启动（带宽优选模式）")

	result := pro.Scan(ctx, cfg, func(msg string) {
		session.sendWSMessage("pro_progress", map[string]interface{}{"msg": msg})
	})

	if ctx.Err() != nil && result != nil && !result.Cancelled {
		result.Cancelled = true
		result.Error = "扫描已取消"
	}

	session.sendWSMessage("pro_complete", result)

	// 同时写一条日志，方便在日志框里看到摘要
	if result != nil {
		switch {
		case result.Cancelled:
			session.sendWSMessage("log", "⚡ 增强优选已取消")
		case result.IP != "":
			session.sendWSMessage("log", "⚡ 增强优选完成："+result.Address+
				" 实测约 "+strconv.Itoa(result.RealBandwidth)+" Mbps")
		default:
			session.sendWSMessage("log", "⚡ 增强优选结束："+result.Error)
		}
	}
}
