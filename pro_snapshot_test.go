package main

import "testing"

// pro 任务的进度必须进入后台快照，否则切后台回来后看不到实时状态。
func TestProProgressSnapshot(t *testing.T) {
	s := &appSession{isTaskRunning: true, backgroundTask: true}
	s.applyBackgroundMessageLocked("pro_progress", map[string]interface{}{
		"msg": "测速中 10/50", "phase": "speed", "current": 10, "total": 50, "found": 3,
	})
	if s.backgroundSnapshot.ResultCount != 3 {
		t.Fatalf("ResultCount = %d, want 3", s.backgroundSnapshot.ResultCount)
	}
	if s.backgroundSnapshot.Current != 10 || s.backgroundSnapshot.Total != 50 {
		t.Fatalf("progress = %d/%d, want 10/50", s.backgroundSnapshot.Current, s.backgroundSnapshot.Total)
	}
	// pro_complete 要缓存结果，供跟随后补发
	payload := map[string]interface{}{"ip": "1.2.3.4"}
	s.applyBackgroundMessageLocked("pro_complete", payload)
	s.proMutex.Lock()
	cached := s.proCompletePayload
	s.proMutex.Unlock()
	if cached == nil {
		t.Fatal("pro_complete payload not cached")
	}
	if s.backgroundSnapshot.Phase != "完成" {
		t.Fatalf("Phase = %q, want 完成", s.backgroundSnapshot.Phase)
	}
}
