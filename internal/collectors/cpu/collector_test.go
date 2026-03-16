package cpu

import "testing"

func TestFirstPercent(t *testing.T) {
	if got := firstPercent(nil); got != 0 {
		t.Fatalf("expected 0 for empty slice, got %f", got)
	}
	if got := firstPercent([]float64{12.5, 30}); got != 12.5 {
		t.Fatalf("expected 12.5, got %f", got)
	}
}

func TestAppendUnique(t *testing.T) {
	values := appendUnique([]string{"a"}, "b", "a", " ", "c")
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d (%v)", len(values), values)
	}
	if values[0] != "a" || values[1] != "b" || values[2] != "c" {
		t.Fatalf("unexpected order/value: %v", values)
	}
}
