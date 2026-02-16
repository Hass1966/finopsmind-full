// Package correlation provides request correlation ID handling.
package correlation

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// HeaderName is the HTTP header for correlation IDs.
const HeaderName = "X-Correlation-ID"

// Generator generates correlation IDs.
type Generator struct{}

// NewGenerator creates a new correlation ID generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates a new correlation ID.
func (g *Generator) Generate() string {
	return uuid.New().String()
}

// Middleware adds correlation ID to requests.
func Middleware(gen *Generator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for existing correlation ID
			correlationID := r.Header.Get(HeaderName)
			if correlationID == "" {
				correlationID = gen.Generate()
			}

			// Add to context
			ctx := context.WithValue(r.Context(), correlationIDKey, correlationID)

			// Add to response header
			w.Header().Set(HeaderName, correlationID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetID retrieves the correlation ID from context.
func GetID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// WithID adds a correlation ID to the context.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}
