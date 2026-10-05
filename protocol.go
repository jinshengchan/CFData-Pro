package main

import "encoding/json"

type wsRequest struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type startTaskRequest struct {
	IPType             int     `json:"ipType"`
	Threads            int     `json:"threads"`
	Port               int     `json:"port"`
	Delay              int     `json:"delay"`
	ScanMode           string  `json:"scanMode"`
	AutoSpeed          bool    `json:"autoSpeed"`
	OfficialTargetDC   string  `json:"officialTargetDC"`
	OfficialSpeedPort  int     `json:"officialSpeedPort"`
	OfficialSpeedURL   string  `json:"officialSpeedURL"`
	OfficialSpeedMin   float64 `json:"officialSpeedMin"`
	OfficialSpeedLimit int     `json:"officialSpeedLimit"`
}

type startTestRequest struct {
	DC       string `json:"dc"`
	Port     int    `json:"port"`
	Delay    int    `json:"delay"`
	ScanMode string `json:"scanMode"`
}

// startProTaskRequest 增强优选（guanfang-youxuan 引擎）的启动参数。
type startProTaskRequest struct {
	IPType       int    `json:"ipType"`       // 4 或 6
	UseTLS       bool   `json:"useTLS"`       // 是否启用 TLS
	Bandwidth    int    `json:"bandwidth"`    // 期望带宽 Mbps，0 用默认 1
	Ports        string `json:"ports"`        // 端口 CSV，如 "443,2053"，空用默认
	Countries    string `json:"countries"`    // 落地国家 CSV，如 "HK,JP"，空不限
	SNI          string `json:"sni"`          // 自定义 SNI/节点域名，空用默认
	WantCount    int    `json:"wantCount"`    // 输出数量，收敛到 1/5/10
	SpeedSeconds int    `json:"speedSeconds"` // 测速时长，收敛到 5/10/15
	SpeedSource  string `json:"speedSource"`  // 测速源标识，空或 auto 自动
	SpeedURL     string `json:"speedURL"`     // 手动测速地址，非空优先
	IPRanges     string `json:"ipRanges"`     // 指定 IP 段原文，空用官方列表
}

type proPreviewRangesRequest struct {
	IPRanges string `json:"ipRanges"`
}

type startSpeedTestRequest struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

type startOfficialSpeedBatchRequest struct {
	Port       int          `json:"port"`
	URL        string       `json:"url"`
	SpeedMin   float64      `json:"speedMin"`
	SpeedLimit int          `json:"speedLimit"`
	Results    []TestResult `json:"results"`
	SkipTested bool         `json:"skipTested"`
}

type startNSBTaskRequest struct {
	FileName     string  `json:"fileName"`
	FileContent  string  `json:"fileContent"`
	SourceURL    string  `json:"sourceURL"`
	OutFile      string  `json:"outFile"`
	MaxThreads   int     `json:"maxThreads"`
	FallbackPort int     `json:"fallbackPort"`
	SpeedTest    int     `json:"speedTest"`
	SpeedURL     string  `json:"speedURL"`
	EnableTLS    bool    `json:"enableTLS"`
	Delay        int     `json:"delay"`
	ResultLimit  int     `json:"resultLimit"`
	DC           string  `json:"dc"`
	SpeedMin     float64 `json:"speedMin"`
	SpeedLimit   int     `json:"speedLimit"`
	Compact      bool    `json:"compact"`
	ScanMode     string  `json:"scanMode"`
}

type startNSBSpeedBatchRequest struct {
	Results    []nsbScanMessage `json:"results"`
	SpeedTest  int              `json:"speedTest"`
	SpeedURL   string           `json:"speedURL"`
	EnableTLS  bool             `json:"enableTLS"`
	SpeedMin   float64          `json:"speedMin"`
	SpeedLimit int              `json:"speedLimit"`
	SkipTested bool             `json:"skipTested"`
	Compact    bool             `json:"compact"`
}

type edgetunnelUploadRequest struct {
	Host     string `json:"host"`
	Password string `json:"password"`
	Content  string `json:"content"`
	Mode     string `json:"mode"`
	Silent   bool   `json:"silent"`
}

type githubUploadRequest struct {
	Token   string `json:"token"`
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Content string `json:"content"`
	Silent  bool   `json:"silent"`
}
