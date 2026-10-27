package embed

import "testing"

func TestNormalizeDomains(t *testing.T) {
	got, err := normalizeDomains([]string{" https://WWW.Example.com/page?x=1 ", "*.example.org", "localhost:5500", "www.example.com", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"www.example.com", "*.example.org", "localhost:5500"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	for _, bad := range []string{"exa mple.com", "javascript:alert(1)", "'self'", "*", "a.com;b"} {
		if _, err := normalizeDomains([]string{bad}); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestGenerateTokenMatchesPattern(t *testing.T) {
	for i := 0; i < 50; i++ {
		token, err := generateToken()
		if err != nil {
			t.Fatal(err)
		}
		if !tokenPattern.MatchString(token) {
			t.Fatalf("token %q does not match pattern", token)
		}
	}
	if tokenPattern.MatchString("vt_short") || tokenPattern.MatchString("../../etc") {
		t.Fatal("pattern accepted an invalid token")
	}
}
