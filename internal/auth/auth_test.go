package auth

import "testing"

func TestRandomState(t *testing.T) {
	a := randomState()
	b := randomState()
	if len(a) != 32 {
		t.Errorf("randomState length = %d, want 32 hex chars", len(a))
	}
	if a == b {
		t.Error("two random states should differ")
	}
	for _, r := range a {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("randomState contains non-hex rune %q", r)
		}
	}
}
