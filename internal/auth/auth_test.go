package auth

import (
	"testing"
)

func TestHashKeyIsStableSHA256Hex(t *testing.T) {
	got := HashKey("secret")
	want := "2bb80d537b1da3e38bd30361aa855686bde0eacd7162fef6a25fe97bf527a25b"
	if got != want {
		t.Fatalf("hash = %q, want %q", got, want)
	}
}

func TestGenerateKeyReturnsDistinctPrintableSecrets(t *testing.T) {
	first, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal("two generated keys matched")
	}
	if len(first) < 32 || len(second) < 32 {
		t.Fatalf("generated key too short: %q %q", first, second)
	}
	if HashKey(first) == first {
		t.Fatal("hash should not equal plaintext key")
	}
}

func TestTierFromString(t *testing.T) {
	tests := []struct {
		raw  string
		want Tier
		ok   bool
	}{
		{raw: "free", want: TierFree, ok: true},
		{raw: "paid", want: TierPaid, ok: true},
		{raw: "enterprise", ok: false},
		{raw: "", ok: false},
	}

	for _, tt := range tests {
		got, ok := TierFromString(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("TierFromString(%q) = %q,%v; want %q,%v", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}
