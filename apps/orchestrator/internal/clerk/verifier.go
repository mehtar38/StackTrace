// Package clerk verifies Clerk-issued JWTs without using the Clerk SDK.
// Tokens are validated against Clerk's public JWKS endpoint using standard
// RS256 JWT verification. The `sub` claim is the Clerk user ID.
package clerk

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the validated JWT claims from a Clerk session token.
type Claims struct {
	// Sub is the Clerk user ID
	Sub string
	// Email is included if the Clerk JWT template includes it.
	// Falls back to empty string if not present.
	Email string
}

// JWTVerifier validates Clerk JWTs using JWKS.
type JWTVerifier struct {
	jwksURL string

	// JWKS cache — refreshed when a key ID is not found.
	mu          sync.RWMutex
	keyCache    map[string]*rsa.PublicKey
	lastFetched time.Time
}

const jwksCacheTTL = 1 * time.Hour

// NewJWTVerifier constructs a JWTVerifier.
// jwksURL: Clerk's JWKS endpoint
func NewJWTVerifier(jwksURL string) *JWTVerifier {
	return &JWTVerifier{
		jwksURL:  jwksURL,
		keyCache: make(map[string]*rsa.PublicKey),
	}
}

// Verify validates a Clerk JWT and returns the extracted claims.
// Returns an error if the token is invalid, expired, or uses an unknown key.
func (v *JWTVerifier) Verify(ctx context.Context, tokenString string) (*Claims, error) {
	var clerkClaims struct {
		jwt.RegisteredClaims
		Email string `json:"email"`
	}

	token, err := jwt.ParseWithClaims(tokenString, &clerkClaims, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token uses RSA signing
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in JWT header")
		}

		return v.getPublicKey(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("jwt parse: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	sub, err := clerkClaims.GetSubject()
	if err != nil || sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	return &Claims{
		Sub:   sub,
		Email: clerkClaims.Email,
	}, nil
}

// getPublicKey returns the RSA public key for the given key ID.
// Fetches and caches JWKS on first call or when the key is unknown.
func (v *JWTVerifier) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Fast path: key is in cache
	v.mu.RLock()
	key, ok := v.keyCache[kid]
	v.mu.RUnlock()
	if ok {
		return key, nil
	}

	// Slow path: fetch JWKS
	if err := v.fetchJWKS(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	key, ok = v.keyCache[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key ID %q not found in Clerk JWKS", kid)
	}
	return key, nil
}

// jwksResponse is the JSON structure returned by Clerk's JWKS endpoint.
type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"` // RSA modulus (base64url)
	E   string `json:"e"` // RSA exponent (base64url)
}

// fetchJWKS downloads Clerk's JWKS and populates the key cache.
func (v *JWTVerifier) fetchJWKS(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check: another goroutine may have fetched while we were waiting for the lock
	if time.Since(v.lastFetched) < jwksCacheTTL && len(v.keyCache) > 0 {
		return nil
	}

	slog.Debug("fetching Clerk JWKS", "url", v.jwksURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("build JWKS request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read JWKS body: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("parse JWKS: %w", err)
	}

	newCache := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" || key.Use != "sig" {
			continue
		}
		pub, err := jwkToRSA(key)
		if err != nil {
			slog.Warn("skipping invalid JWK", "kid", key.Kid, "error", err)
			continue
		}
		newCache[key.Kid] = pub
	}

	v.keyCache = newCache
	v.lastFetched = time.Now()
	slog.Debug("JWKS cached", "keys", len(newCache))
	return nil
}

// jwkToRSA converts a JWK to an *rsa.PublicKey.
func jwkToRSA(key jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
