// Package accessjwt checks the Cloudflare Access token (Cf-Access-Jwt-Assertion)
// that Access attaches to every request it lets through. Cloudflare requires
// this check before Access can act as the OAuth server for an MCP endpoint.
package accessjwt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// Header is where Access puts the signed token for the origin.
const Header = "Cf-Access-Jwt-Assertion"

const (
	leeway          = time.Minute
	minRefreshDelay = time.Minute
)

// Verifier checks tokens issued by one Access team for one application.
type Verifier struct {
	issuer   string // https://<team>.cloudflareaccess.com
	audience string // the application's Audience (AUD) tag
	certsURL string
	client   *http.Client
	now      func() time.Time

	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	lastRefresh time.Time
}

// New returns a Verifier for teamDomain (e.g. "myteam.cloudflareaccess.com")
// and the application's audience tag.
func New(teamDomain, audience string) *Verifier {
	teamDomain = strings.TrimSuffix(strings.TrimPrefix(teamDomain, "https://"), "/")
	return &Verifier{
		issuer:   "https://" + teamDomain,
		audience: audience,
		certsURL: "https://" + teamDomain + "/cdn-cgi/access/certs",
		client:   &http.Client{Timeout: 10 * time.Second},
		now:      time.Now,
	}
}

// Require rejects requests without a valid Access token.
func (v *Verifier) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := v.Verify(r.Context(), r.Header.Get(Header)); err != nil {
			log.Printf("access token rejected on %s: %v", r.URL.Path, err)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"missing or invalid Cloudflare Access token"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type claims struct {
	Iss string   `json:"iss"`
	Aud audience `json:"aud"`
	Exp int64    `json:"exp"`
	Nbf int64    `json:"nbf"`
}

// audience accepts the "aud" claim as a string or a list of strings.
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

// Verify checks the token's RS256 signature, issuer, audience and lifetime.
func (v *Verifier) Verify(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("no token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("malformed token")
	}
	var h header
	if err := decodePart(parts[0], &h); err != nil {
		return fmt.Errorf("header: %w", err)
	}
	if h.Alg != "RS256" {
		return fmt.Errorf("unexpected algorithm %q", h.Alg)
	}
	key, err := v.key(ctx, h.Kid)
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("signature: %w", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return errors.New("bad signature")
	}

	var c claims
	if err := decodePart(parts[1], &c); err != nil {
		return fmt.Errorf("claims: %w", err)
	}
	now := v.now()
	switch {
	case c.Iss != v.issuer:
		return fmt.Errorf("issuer %q, want %q", c.Iss, v.issuer)
	case !slices.Contains(c.Aud, v.audience):
		return errors.New("token is for another application")
	case c.Exp == 0 || now.After(time.Unix(c.Exp, 0).Add(leeway)):
		return errors.New("token expired")
	case c.Nbf != 0 && now.Add(leeway).Before(time.Unix(c.Nbf, 0)):
		return errors.New("token not valid yet")
	}
	return nil
}

func decodePart(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// key returns the signing key with this id, fetching Access's current keys
// when it is unknown (Access rotates them). Refetches are rate limited so a
// stream of bad tokens can't hammer Cloudflare.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	if !v.lastRefresh.IsZero() && v.now().Sub(v.lastRefresh) < minRefreshDelay {
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	v.lastRefresh = v.now()
	keys, err := v.fetchKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch Access keys: %w", err)
	}
	v.keys = keys
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key %q", kid)
}

func (v *Verifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range body.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("no RSA keys in response")
	}
	return keys, nil
}
