package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/finopsmind/backend/internal/apierrors"
	"github.com/finopsmind/backend/internal/repository"
)

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey int

const (
	claimsContextKey contextKey = iota
)

// Middleware returns an HTTP middleware that validates JWT tokens or API keys
// from the Authorization header and injects the authenticated claims into the
// request context.
func Middleware(jwtMgr *JWTManager, userRepo repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				apierrors.NewUnauthorizedError("missing authorization header").Write(w, r)
				return
			}

			// Bearer token (JWT)
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := jwtMgr.ValidateToken(tokenStr)
				if err != nil {
					apierrors.NewUnauthorizedError("invalid or expired token").Write(w, r)
					return
				}
				ctx := context.WithValue(r.Context(), claimsContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// API key authentication: "ApiKey <hex-key>"
			if strings.HasPrefix(authHeader, "ApiKey ") {
				apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
				hashedKey := HashAPIKey(apiKey)

				user, err := userRepo.GetByAPIKeyHash(r.Context(), hashedKey)
				if err != nil || user == nil {
					apierrors.NewUnauthorizedError("invalid API key").Write(w, r)
					return
				}

				if !user.Active {
					apierrors.NewForbiddenError("account is deactivated").Write(w, r)
					return
				}

				claims := &Claims{
					UserID: user.ID,
					OrgID:  user.OrganizationID,
					Email:  user.Email,
					Role:   Role(user.Role),
				}
				ctx := context.WithValue(r.Context(), claimsContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			apierrors.NewUnauthorizedError("unsupported authorization scheme").Write(w, r)
		})
	}
}

// RequireRole returns a middleware that restricts access to users whose role
// is in the provided set of allowed roles.
func RequireRole(allowed ...Role) func(http.Handler) http.Handler {
	allowedSet := make(map[Role]bool, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaimsFromContext(r.Context())
			if claims == nil {
				apierrors.NewUnauthorizedError("authentication required").Write(w, r)
				return
			}
			if !allowedSet[claims.Role] {
				apierrors.NewForbiddenError("insufficient permissions").Write(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetClaimsFromContext extracts the Claims stored in the context by the auth
// middleware.
func GetClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsContextKey).(*Claims)
	return claims
}

// GetUserFromContext returns the authenticated user's ID and email from the
// request context.
func GetUserFromContext(ctx context.Context) (userID uuid.UUID, email string, ok bool) {
	claims := GetClaimsFromContext(ctx)
	if claims == nil {
		return uuid.Nil, "", false
	}
	return claims.UserID, claims.Email, true
}

// GetOrgIDFromContext returns the authenticated user's organization ID from
// the request context.
func GetOrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	claims := GetClaimsFromContext(ctx)
	if claims == nil {
		return uuid.Nil, false
	}
	return claims.OrgID, true
}
