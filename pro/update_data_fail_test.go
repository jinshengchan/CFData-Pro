package pro

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 模拟手机上下载失败：UpdateDataFiles 不得删空缓存，
// 必须用内置种子兜底并如实报告。
func TestUpdateDataFilesDownloadFailure(t *testing.T) {
	oldClient := downloadClient
	downloadClient = &http.Client{Timeout: 2 * time.Second, Transport: roundTripFail{}}
	defer func() { downloadClient = oldClient }()

	dir := t.TempDir()
	msg := UpdateDataFiles(context.Background(), dir)
	t.Logf("msg: %s", msg)
	if !strings.Contains(msg, "已保留本地数据") {
		t.Fatalf("期望失败时保留本地数据的提示，实际: %q", msg)
	}
	for _, f := range []string{"locations.json", "ips-v4.txt", "ips-v6.txt", "url.txt"} {
		fp := filepath.Join(dir, f)
		st, err := os.Stat(fp)
		if err != nil || st.Size() == 0 {
			t.Fatalf("下载失败后 %s 应由种子补上，实际: %v", f, err)
		}
	}
}

type roundTripFail struct{}

func (roundTripFail) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &urlError{msg: "模拟断网"}
}

type urlError struct{ msg string }

func (e *urlError) Error() string { return e.msg }

type roundTripSwitch struct {
	failHost string
	body     string
}

func (r roundTripSwitch) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, r.failHost) {
		return nil, fmt.Errorf("dial tcp: no such host")
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

// 主源挂掉时应自动切到备用镜像，而不是直接报错。
func TestGetURLContentFirstFallback(t *testing.T) {
	oldClient := downloadClient
	oldChains := dataSourceChains
	defer func() { downloadClient = oldClient; dataSourceChains = oldChains }()

	downloadClient = &http.Client{Transport: roundTripSwitch{failHost: "primary.invalid", body: "mirror-ok"}}
	dataSourceChains = map[string][]string{
		"t": {"https://primary.invalid/x", "https://mirror.invalid/x"},
	}
	body, err := getURLContentFirst("t")
	if err != nil {
		t.Fatalf("fallback err: %v", err)
	}
	if body != "mirror-ok" {
		t.Fatalf("body = %q", body)
	}
}

// 非 2xx 响应应视为失败，触发备用源（而不是把错误页存成数据文件）。
func TestGetURLContentRejectsNon2xx(t *testing.T) {
	oldClient := downloadClient
	defer func() { downloadClient = oldClient }()
	downloadClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("<html>blocked</html>")),
			Header: make(http.Header), Request: req}, nil
	})}
	if _, err := getURLContent("https://primary.invalid/x"); err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
