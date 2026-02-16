package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/finopsmind/backend/internal/apierrors"
	"github.com/finopsmind/backend/internal/model"
	"github.com/finopsmind/backend/internal/repository"
)

// Handler exposes HTTP endpoints for authentication.
type Handler struct {
	jwtMgr   *JWTManager
	userRepo repository.UserRepository
	orgRepo  repository.OrganizationRepository
}

// NewHandler creates a new auth Handler.
func NewHandler(jwtMgr *JWTManager, userRepo repository.UserRepository, orgRepo repository.OrganizationRepository) *Handler {
	return &Handler{
		jwtMgr:   jwtMgr,
		userRepo: userRepo,
		orgRepo:  orgRepo,
	}
}

// --- Request / Response types ------------------------------------------------

// LoginRequest is the payload for POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SignupRequest is the payload for POST /api/v1/auth/signup.
type SignupRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	OrgName   string `json:"org_name"`
}

// TokenResponse is the standard response containing a JWT.
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo is a safe subset of user data returned in API responses.
type UserInfo struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Email          string  `json:"email"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	Role           string  `json:"role"`
	LastLoginAt    *string `json:"last_login_at,omitempty"`
}

// APIKeyResponse is the response for POST /api/v1/auth/api-keys.
type APIKeyResponse struct {
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
}

// --- Handlers ----------------------------------------------------------------

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.NewBadRequestError("invalid request body").Write(w, r)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		apierrors.NewValidationError("email and password are required", nil).Write(w, r)
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		apierrors.NewUnauthorizedError("invalid email or password").Write(w, r)
		return
	}

	if !user.Active {
		apierrors.NewForbiddenError("account is deactivated").Write(w, r)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		apierrors.NewUnauthorizedError("invalid email or password").Write(w, r)
		return
	}

	token, err := h.jwtMgr.GenerateToken(user.ID, user.OrganizationID, user.Email, Role(user.Role))
	if err != nil {
		apierrors.NewInternalError("failed to generate token").Write(w, r)
		return
	}

	// Update last login timestamp (best effort).
	now := time.Now().UTC()
	_ = h.userRepo.UpdateLastLogin(r.Context(), user.ID, now)

	writeJSON(w, http.StatusOK, TokenResponse{
		Token:     token,
		ExpiresAt: now.Add(h.jwtMgr.expiry),
		User:      toUserInfo(user),
	})
}

// Signup handles POST /api/v1/auth/signup.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierrors.NewBadRequestError("invalid request body").Write(w, r)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" || req.OrgName == "" {
		apierrors.NewValidationError("all fields are required: email, password, first_name, last_name, org_name", nil).Write(w, r)
		return
	}

	if len(req.Password) < 8 {
		apierrors.NewValidationError("password must be at least 8 characters", nil).Write(w, r)
		return
	}

	// Check for existing user.
	existing, _ := h.userRepo.GetByEmail(r.Context(), req.Email)
	if existing != nil {
		apierrors.NewConflictError("a user with this email already exists").Write(w, r)
		return
	}

	// Create organization.
	org := &model.Organization{
		BaseEntity: model.NewBaseEntity(),
		Name:       req.OrgName,
		Settings: model.OrganizationSettings{
			DefaultCurrency: model.CurrencyUSD,
			Timezone:        "UTC",
			FiscalYearStart: 1,
			AlertsEnabled:   true,
		},
	}
	if err := h.orgRepo.Create(r.Context(), org); err != nil {
		apierrors.NewInternalError("failed to create organization").Write(w, r)
		return
	}

	// Hash password.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		apierrors.NewInternalError("failed to hash password").Write(w, r)
		return
	}

	// Create user with admin role (first user in org).
	user := &model.User{
		BaseEntity:     model.NewBaseEntity(),
		OrganizationID: org.ID,
		Email:          req.Email,
		PasswordHash:   string(passwordHash),
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Role:           string(RoleAdmin),
		Active:         true,
	}
	if err := h.userRepo.Create(r.Context(), user); err != nil {
		apierrors.NewInternalError("failed to create user").Write(w, r)
		return
	}

	// Generate token.
	token, err := h.jwtMgr.GenerateToken(user.ID, user.OrganizationID, user.Email, RoleAdmin)
	if err != nil {
		apierrors.NewInternalError("failed to generate token").Write(w, r)
		return
	}

	writeJSON(w, http.StatusCreated, TokenResponse{
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(h.jwtMgr.expiry),
		User:      toUserInfo(user),
	})
}

// CreateAPIKey handles POST /api/v1/auth/api-keys.
// Requires an authenticated user (JWT).
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		apierrors.NewUnauthorizedError("authentication required").Write(w, r)
		return
	}

	plainKey, hashedKey, err := GenerateAPIKey()
	if err != nil {
		apierrors.NewInternalError("failed to generate API key").Write(w, r)
		return
	}

	if err := h.userRepo.SetAPIKeyHash(r.Context(), claims.UserID, hashedKey); err != nil {
		apierrors.NewInternalError("failed to store API key").Write(w, r)
		return
	}

	writeJSON(w, http.StatusCreated, APIKeyResponse{
		Key:       plainKey,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Message:   "Store this key securely. It will not be shown again.",
	})
}

// Me handles GET /api/v1/auth/me.
// Returns the currently authenticated user's profile.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		apierrors.NewUnauthorizedError("authentication required").Write(w, r)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		apierrors.NewNotFoundError("user", claims.UserID.String()).Write(w, r)
		return
	}

	writeJSON(w, http.StatusOK, toUserInfo(user))
}

// --- Helpers -----------------------------------------------------------------

func toUserInfo(u *model.User) UserInfo {
	info := UserInfo{
		ID:             u.ID.String(),
		OrganizationID: u.OrganizationID.String(),
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Role:           u.Role,
	}
	if u.LastLoginAt != nil {
		s := u.LastLoginAt.Format(time.RFC3339)
		info.LastLoginAt = &s
	}
	return info
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
