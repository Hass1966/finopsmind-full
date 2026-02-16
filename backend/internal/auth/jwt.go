// Package auth provides authentication and authorization for the FinOpsMind API.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role represents a user role within an organization.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// ValidRoles contains all valid role values.
var ValidRoles = map[Role]bool{
	RoleAdmin:  true,
	RoleEditor: true,
	RoleViewer: true,
}

// IsValidRole checks whether a role string is a recognized role.
func IsValidRole(r string) bool {
	return ValidRoles[Role(r)]
}

// Claims represents the JWT claims for a FinOpsMind token.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	OrgID  uuid.UUID `json:"org_id"`
	Email  string    `json:"email"`
	Role   Role      `json:"role"`
	Exp    int64     `json:"exp"`
	Iat    int64     `json:"iat"`
}

// IsExpired returns true if the token has expired.
func (c *Claims) IsExpired() bool {
	return time.Now().Unix() > c.Exp
}

// jwtHeader is the fixed header for HS256 tokens.
var jwtHeader = base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

// Errors returned by JWT operations.
var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("token has expired")
	ErrInvalidSecret  = errors.New("jwt secret must not be empty")
)

// JWTManager handles token generation and validation.
type JWTManager struct {
	secret []byte
	expiry time.Duration
}

// NewJWTManager creates a new JWTManager with the given secret and token lifetime.
func NewJWTManager(secret string, expiry time.Duration) (*JWTManager, error) {
	if secret == "" {
		return nil, ErrInvalidSecret
	}
	return &JWTManager{
		secret: []byte(secret),
		expiry: expiry,
	}, nil
}

// GenerateToken creates a signed JWT for the given claims.
func (m *JWTManager) GenerateToken(userID, orgID uuid.UUID, email string, role Role) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		OrgID:  orgID,
		Email:  email,
		Role:   role,
		Iat:    now.Unix(),
		Exp:    now.Add(m.expiry).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	encodedPayload := base64URLEncode(payload)
	signingInput := jwtHeader + "." + encodedPayload
	signature := m.sign([]byte(signingInput))

	return signingInput + "." + base64URLEncode(signature), nil
}

// ValidateToken parses and validates a JWT string, returning its claims.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	signatureBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	expectedSig := m.sign([]byte(signingInput))
	if !hmac.Equal(signatureBytes, expectedSig) {
		return nil, ErrInvalidToken
	}

	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if claims.IsExpired() {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

// sign computes the HMAC-SHA256 signature for the given data.
func (m *JWTManager) sign(data []byte) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write(data)
	return h.Sum(nil)
}

// GenerateAPIKey generates a random 32-byte API key and returns the hex-encoded
// key (to give to the user) and its SHA-256 hash (to store in the database).
func GenerateAPIKey() (plainKey string, hashedKey string, err error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", "", fmt.Errorf("generate random key: %w", err)
	}

	plainKey = hex.EncodeToString(key)
	hashedKey = HashAPIKey(plainKey)
	return plainKey, hashedKey, nil
}

// HashAPIKey returns the SHA-256 hex digest of a plaintext API key.
func HashAPIKey(plainKey string) string {
	h := sha256.Sum256([]byte(plainKey))
	return hex.EncodeToString(h[:])
}

// base64URLEncode encodes bytes to base64url without padding.
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// base64URLDecode decodes a base64url string without padding.
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
