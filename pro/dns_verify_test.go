package pro

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 兜底 DNS 能解析出 IP（沙箱会劫持到代理 IP，但机制上能返回即算通）。
func TestResolveIPFallback(t *testing.T) {
	for _, h := range []string{"www.baipiao.eu.org", "cdn.jsdelivr.net", "223.5.5.5"} {
		ip, err := ResolveIP(h)
		if err != nil {
			t.Fatalf("ResolveIP(%s): %v", h, err)
		}
		if net.ParseIP(ip) == nil {
			t.Fatalf("ResolveIP(%s) = %q, not an IP", h, ip)
		}
		t.Logf("%s -> %s", h, ip)
	}
}

// 自定义 DialContext 的传输链路正常（用 IP 字面量，不依赖外部 DNS）。
func TestDownloadTransportIPLiteral(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello-dns-fix")
	}))
	defer srv.Close()
	// httptest 给的是 127.0.0.1:port，ResolveIP 直接透传
	body, err := getURLContent(srv.URL)
	if err != nil {
		t.Fatalf("getURLContent: %v", err)
	}
	if body != "hello-dns-fix" {
		t.Fatalf("body = %q", body)
	}
}
