package accessjwt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const aud = "app-aud-tag"

type fixture struct {
	v       *Verifier
	key     *rsa.PrivateKey
	fetches *atomic.Int32
	now     time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{key: key, fetches: &atomic.Int32{}, now: time.Unix(1_800_000_000, 0)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.fetches.Add(1)
		if r.URL.Path != "/cdn-cgi/access/certs" {
			http.NotFound(w, r)
			return
		}
		b64 := base64.RawURLEncoding.EncodeToString
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)
	f.v = New("team.cloudflareaccess.com", aud)
	f.v.certsURL = srv.URL + "/cdn-cgi/access/certs"
	f.v.now = func() time.Time { return f.now }
	return f
}

func (f *fixture) sign(t *testing.T, kid, alg string, c map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signing := enc(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"}) + "." + enc(c)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (f *fixture) claims() map[string]any {
	return map[string]any{
		"iss": "https://team.cloudflareaccess.com", "aud": []string{aud},
		"exp": f.now.Add(time.Hour).Unix(), "nbf": f.now.Add(-time.Minute).Unix(),
		"email": "me@example.com",
	}
}

func TestVerify(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.v.Verify(ctx, f.sign(t, "k1", "RS256", f.claims())); err != nil {
		t.Fatalf("valid token: %v", err)
	}
	c := f.claims()
	c["aud"] = aud // a plain string is accepted too
	if err := f.v.Verify(ctx, f.sign(t, "k1", "RS256", c)); err != nil {
		t.Fatalf("string aud: %v", err)
	}

	bad := map[string]func(map[string]any){
		"other audience": func(c map[string]any) { c["aud"] = []string{"other"} },
		"other issuer":   func(c map[string]any) { c["iss"] = "https://evil.cloudflareaccess.com" },
		"expired":        func(c map[string]any) { c["exp"] = f.now.Add(-2 * time.Minute).Unix() },
		"no expiry":      func(c map[string]any) { delete(c, "exp") },
		"not yet valid":  func(c map[string]any) { c["nbf"] = f.now.Add(5 * time.Minute).Unix() },
	}
	for name, mutate := range bad {
		c := f.claims()
		mutate(c)
		if err := f.v.Verify(ctx, f.sign(t, "k1", "RS256", c)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	good := f.sign(t, "k1", "RS256", f.claims())
	parts := strings.Split(good, ".")
	tampered := f.claims()
	tampered["aud"] = []string{aud, "extra"}
	forged := parts[0] + "." + strings.Split(f.sign(t, "k1", "RS256", tampered), ".")[1] + "." + parts[2]
	for name, tok := range map[string]string{
		"empty":         "",
		"garbage":       "a.b",
		"swapped body":  forged,
		"alg none":      strings.Replace(good, parts[0], base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","kid":"k1"}`)), 1),
		"wrong alg tag": f.sign(t, "k1", "HS256", f.claims()),
	} {
		if err := f.v.Verify(ctx, tok); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestKeyFetchIsCachedAndRateLimited(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for range 3 {
		if err := f.v.Verify(ctx, f.sign(t, "k1", "RS256", f.claims())); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fetches.Load(); n != 1 {
		t.Fatalf("fetches = %d, want 1 (cached)", n)
	}
	for range 5 {
		_ = f.v.Verify(ctx, f.sign(t, "unknown", "RS256", f.claims()))
	}
	if n := f.fetches.Load(); n != 1 {
		t.Fatalf("fetches = %d, want no refetch within a minute", n)
	}
	f.now = f.now.Add(2 * time.Minute)
	_ = f.v.Verify(ctx, f.sign(t, "unknown", "RS256", f.claims()))
	if n := f.fetches.Load(); n != 2 {
		t.Fatalf("fetches = %d, want a refetch after a minute (key rotation)", n)
	}
}

func TestRequire(t *testing.T) {
	f := newFixture(t)
	h := f.v.Require(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for name, tc := range map[string]struct {
		token string
		want  int
	}{
		"valid":   {f.sign(t, "k1", "RS256", f.claims()), http.StatusNoContent},
		"missing": {"", http.StatusForbidden},
	} {
		req := httptest.NewRequest("POST", "/mcp", nil)
		if tc.token != "" {
			req.Header.Set(Header, tc.token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: status %d, want %d", name, rec.Code, tc.want)
		}
	}
}
