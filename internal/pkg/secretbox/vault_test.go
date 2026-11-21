package secretbox

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	v, err := New("test-master-key-please-change")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := v.Seal("plain-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "plain-secret-value" {
		t.Fatal("expected encrypted form")
	}
	out, err := v.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if out != "plain-secret-value" {
		t.Fatalf("got %q", out)
	}
	// 兼容明文
	plain, err := v.Open("legacy-plain")
	if err != nil || plain != "legacy-plain" {
		t.Fatalf("legacy open failed: %v %q", err, plain)
	}
}
