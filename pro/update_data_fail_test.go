package pro

import (
	"context"
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
