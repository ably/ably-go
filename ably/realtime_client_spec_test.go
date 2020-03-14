package ably

import "testing"

func Test_RTC1a_EchoMessagesTrueByDefault(t *testing.T) {
	o := NewClientOptions()
	if e, g := true, o.EchoMessages; e != g {
		t.Errorf("expected %v, got %v", e, g)
	}
}

func Test_RTC1b_AutoConnectTrueByDefault(t *testing.T) {
	o := NewClientOptions()
	if e, g := true, o.AutoConnect; e != g {
		t.Errorf("expected %v, got %v", e, g)
	}
}
