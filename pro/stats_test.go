package pro

import "testing"

func TestStatsCumulative(t *testing.T) {
	resetProgressStats()
	addRTTStats(200, 0)
	addRTTStats(200, 5)
	st := GetProgressState()
	if st.RTTTested != 400 || st.RTTValid != 5 {
		t.Fatalf("RTT stats = %d/%d, want 400/5", st.RTTTested, st.RTTValid)
	}
	addSpeedStats(1, 2)
	addSpeedStats(1, 3)
	addSpeedStats(1, 2) // qualified 回落不影响最大值
	st = GetProgressState()
	if st.SpeedTested != 3 || st.SpeedQualified != 3 {
		t.Fatalf("speed stats = %d/%d, want 3/3", st.SpeedTested, st.SpeedQualified)
	}
	resetProgressStats()
	st = GetProgressState()
	if st.RTTTested != 0 || st.SpeedTested != 0 {
		t.Fatal("reset failed")
	}
}
