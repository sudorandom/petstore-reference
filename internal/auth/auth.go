package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"connectrpc.com/connect"
)

type contextKey struct{}

var claimsKey = contextKey{}

// Claims holds authenticated identity details propagated from an upstream proxy or bearer token.
type Claims struct {
	Subject  string   `json:"sub"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Provider string   `json:"provider"` // "iap", "oauth2-proxy", "bearer", "dev"
	RawToken string   `json:"-"`
}

// FromContext retrieves the authenticated claims from the context.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	return claims, ok
}

// WithClaims injects claims into the context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

const DefaultDevEmail = "developer@local.test"

// UserEmailFromContext retrieves the authenticated user's email, or a fallback dev email if in dev mode or unset.
func UserEmailFromContext(ctx context.Context) string {
	if claims, ok := FromContext(ctx); ok && claims != nil && claims.Email != "" {
		return claims.Email
	}
	return DefaultDevEmail
}

// DevIdentityMiddleware returns an HTTP middleware that injects local development identity headers
// (simulating an upstream Google Cloud IAP or OAuth2 proxy) when no identity headers are present on the request.
func DevIdentityMiddleware(devEmail, devSubject string) func(http.Handler) http.Handler {
	if devEmail == "" {
		devEmail = DefaultDevEmail
	}
	if devSubject == "" {
		devSubject = "dev-user-001"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only inject if no identity or authorization headers are already present
			if r.Header.Get("X-Goog-Authenticated-User-Email") == "" &&
				r.Header.Get("X-Forwarded-Email") == "" &&
				r.Header.Get("Authorization") == "" {
				r.Header.Set("X-Goog-Authenticated-User-Email", "accounts.google.com:"+devEmail)
				r.Header.Set("X-Goog-Authenticated-User-Id", "accounts.google.com:"+devSubject)
				r.Header.Set("X-Forwarded-Email", devEmail)
				r.Header.Set("X-Forwarded-User", devSubject)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Config defines the authentication interceptor settings.
type Config struct {
	// Enabled determines whether authentication is enforced.
	Enabled bool

	// DevMode, when true, injects a mock identity if no upstream proxy headers or tokens are present.
	DevMode bool

	// RequireProxyHeaders, when true, requires valid IAP or OAuth proxy headers in non-dev environments.
	RequireProxyHeaders bool

	// StaticTokens is an optional list of valid bearer tokens (e.g. for service-to-service or CLI).
	StaticTokens []string

	// SkipProcedures is a set of RPC procedures that bypass authentication (e.g. "/pet.v1.PetService/GetPet").
	SkipProcedures map[string]bool

	// Validator is an optional custom token/JWT validator (e.g. for verifying Google IAP JWT assertion).
	Validator func(ctx context.Context, token string) (*Claims, error)
}

// NewInterceptor returns a Connect unary interceptor that authenticates incoming requests
// via upstream OAuth proxy / Google IAP headers or bearer tokens.
func NewInterceptor(cfg Config) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !cfg.Enabled {
				return next(ctx, req)
			}

			// Check if this procedure skips authentication
			procedure := req.Spec().Procedure
			if cfg.SkipProcedures != nil && cfg.SkipProcedures[procedure] {
				return next(ctx, req)
			}

			header := req.Header()

			// 1. Check Google Cloud IAP headers
			iapEmail := header.Get("X-Goog-Authenticated-User-Email")
			iapID := header.Get("X-Goog-Authenticated-User-Id")
			iapJWT := header.Get("X-Goog-IAP-JWT-Assertion")

			if iapEmail != "" || iapJWT != "" {
				cleanEmail := strings.TrimPrefix(iapEmail, "accounts.google.com:")
				cleanID := strings.TrimPrefix(iapID, "accounts.google.com:")

				claims := &Claims{
					Subject:  cleanID,
					Email:    cleanEmail,
					Provider: "iap",
					Roles:    []string{"user"},
					RawToken: iapJWT,
				}

				if cfg.Validator != nil && iapJWT != "" {
					validatedClaims, err := cfg.Validator(ctx, iapJWT)
					if err != nil {
						return nil, connect.NewError(connect.CodeUnauthenticated, err)
					}
					claims = validatedClaims
				}

				ctx = WithClaims(ctx, claims)
				return next(ctx, req)
			}

			// 2. Check Generic OAuth2 Proxy / Ingress headers (e.g. oauth2-proxy, Envoy, Cloudflare Access)
			forwardedEmail := header.Get("X-Forwarded-Email")
			forwardedUser := header.Get("X-Forwarded-User")
			if forwardedEmail != "" || forwardedUser != "" {
				var roles []string
				if groups := header.Get("X-Forwarded-Groups"); groups != "" {
					for g := range strings.SplitSeq(groups, ",") {
						roles = append(roles, strings.TrimSpace(g))
					}
				}
				if len(roles) == 0 {
					roles = []string{"user"}
				}

				claims := &Claims{
					Subject:  forwardedUser,
					Email:    forwardedEmail,
					Provider: "oauth2-proxy",
					Roles:    roles,
				}
				ctx = WithClaims(ctx, claims)
				return next(ctx, req)
			}

			// 3. Check Authorization: Bearer <token> (for service-to-service calls or API clients)
			authHeader := header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
					token := parts[1]

					if cfg.Validator != nil {
						claims, err := cfg.Validator(ctx, token)
						if err != nil {
							return nil, connect.NewError(connect.CodeUnauthenticated, err)
						}
						ctx = WithClaims(ctx, claims)
						return next(ctx, req)
					}

					if slices.Contains(cfg.StaticTokens, token) {
						claims := &Claims{
							Subject:  "service-account",
							Email:    "service@internal",
							Provider: "bearer",
							Roles:    []string{"service"},
							RawToken: token,
						}
						ctx = WithClaims(ctx, claims)
						return next(ctx, req)
					}

					return nil, connect.NewError(
						connect.CodeUnauthenticated,
						errors.New("invalid or expired bearer token"),
					)
				}
			}

			// 4. Dev Mode fallback: if running locally without an OAuth proxy, supply a default dev identity
			if cfg.DevMode {
				claims := &Claims{
					Subject:  "dev-user",
					Email:    "developer@local.test",
					Provider: "dev",
					Roles:    []string{"user", "admin"},
				}
				ctx = WithClaims(ctx, claims)
				return next(ctx, req)
			}

			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				errors.New("missing authentication credentials: no upstream proxy identity (IAP/OAuth2-Proxy) or authorization header found"),
			)
		}
	}
}
