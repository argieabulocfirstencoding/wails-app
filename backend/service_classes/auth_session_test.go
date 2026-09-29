package service_classes

import "testing"

func TestBubbleSessionDuration(t *testing.T) {
	valid, err := bubbleSessionDuration(float64(31536000))
	if err != nil {
		t.Fatalf("expected valid duration: %v", err)
	}
	if valid.Seconds() != 31536000 {
		t.Fatalf("expected 31536000 seconds, got %v", valid.Seconds())
	}

	if _, err := bubbleSessionDuration("3600"); err != nil {
		t.Fatalf("expected string duration to be accepted: %v", err)
	}
	if _, err := bubbleSessionDuration(float64(1.5)); err == nil {
		t.Fatal("expected fractional duration to be rejected")
	}
	if _, err := bubbleSessionDuration(nil); err == nil {
		t.Fatal("expected missing duration to be rejected")
	}
	if _, err := bubbleSessionDuration(float64(0)); err == nil {
		t.Fatal("expected zero duration to be rejected")
	}
}
