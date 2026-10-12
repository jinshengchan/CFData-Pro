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

// pro 后台快照口径与官方优选对齐：扫描结果=RTT 阶段，测速结果=测速阶段
func TestProStatsSnapshot(t *testing.T) {
	s := &appSession{isTaskRunning: true, backgroundTask: true}
	s.applyBackgroundMessageLocked("pro_progress", map[string]interface{}{
		"msg": "test", "phase": "speed", "current": 6, "total": 10, "found": 4,
		"rttTested": 192, "rttValid": 50, "speedTested": 6, "speedQualified": 4,
	})
	if s.backgroundSnapshot.ScanSuccess != 50 || s.backgroundSnapshot.ScanFailed != 142 {
		t.Fatalf("scan stats = %d/%d, want 50/142",
			s.backgroundSnapshot.ScanSuccess, s.backgroundSnapshot.ScanFailed)
	}
	if s.backgroundSnapshot.SpeedSuccess != 4 || s.backgroundSnapshot.SpeedFailed != 2 {
		t.Fatalf("speed stats = %d/%d, want 4/2",
			s.backgroundSnapshot.SpeedSuccess, s.backgroundSnapshot.SpeedFailed)
	}
	if s.backgroundSnapshot.SpeedQualified != 4 {
		t.Fatalf("speedQualified = %d, want 4", s.backgroundSnapshot.SpeedQualified)
	}
}
