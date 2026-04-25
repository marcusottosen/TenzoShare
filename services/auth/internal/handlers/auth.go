package handlers

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/auth/internal/service"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
)

var validate = validator.New()

type Handler struct {
	svc *service.AuthService
}

func New(svc *service.AuthService) *Handler {
	return &Handler{svc: svc}
}

// ── Register ──────────────────────────────────────────────────────────────────

type registerRequest struct {
	Email    string `json:"email"    validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

func (h *Handler) Register(c fiber.Ctx) error {
	var req registerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	user, err := h.svc.Register(c.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
}

// ── Login ─────────────────────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (h *Handler) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	pair, user, err := h.svc.Login(c.Context(), req.Email, req.Password, c.IP())
	if err != nil {
		// surface mfa_required as a specific response, not a generic 401
		if apperrors.IsUnauthorized(err) && err.Error() == "mfa_required" {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"mfa_required": true,
				"user_id":      user.ID,
			})
		}
		return err
	}

	return c.JSON(tokenResponse(pair))
}

// ── Login with MFA ────────────────────────────────────────────────────────────

type loginMFARequest struct {
	UserID  string `json:"user_id"  validate:"required"`
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}

func (h *Handler) LoginWithMFA(c fiber.Ctx) error {
	var req loginMFARequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	pair, err := h.svc.LoginWithMFA(c.Context(), req.UserID, req.OTPCode)
	if err != nil {
		return err
	}

	return c.JSON(tokenResponse(pair))
}

// ── Refresh ───────────────────────────────────────────────────────────────────

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (h *Handler) Refresh(c fiber.Ctx) error {
	var req refreshRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	pair, err := h.svc.RefreshTokens(c.Context(), req.RefreshToken)
	if err != nil {
		return err
	}

	return c.JSON(tokenResponse(pair))
}

// ── Logout ────────────────────────────────────────────────────────────────────

func (h *Handler) Logout(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	if err := h.svc.Logout(c.Context(), userID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ── MFA setup ─────────────────────────────────────────────────────────────────

func (h *Handler) MFASetup(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	result, err := h.svc.SetupMFA(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"secret":           result.Secret,
		"provisioning_uri": result.ProvisioningURI,
	})
}

// ── MFA verify ────────────────────────────────────────────────────────────────

type mfaVerifyRequest struct {
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}

func (h *Handler) MFAVerify(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	var req mfaVerifyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	if err := h.svc.VerifyMFA(c.Context(), userID, req.OTPCode); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"mfa_enabled": true})
}

// ── Password reset ────────────────────────────────────────────────────────────

type passwordResetRequestBody struct {
	Email string `json:"email" validate:"required,email"`
}

func (h *Handler) PasswordResetRequest(c fiber.Ctx) error {
	var req passwordResetRequestBody
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	// Intentionally always returns 202 — don't leak whether the email exists.
	_, _, _ = h.svc.RequestPasswordReset(c.Context(), req.Email)
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "if that email is registered, a reset link has been sent",
	})
}

type passwordResetConfirmBody struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

func (h *Handler) PasswordResetConfirm(c fiber.Ctx) error {
	var req passwordResetConfirmBody
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	if err := h.svc.ConfirmPasswordReset(c.Context(), req.Token, req.NewPassword); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"message": "password updated"})
}

// ── Me ────────────────────────────────────────────────────────────────────────

func (h *Handler) Me(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	user, err := h.svc.GetMe(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(profileResponse(user))
}

// UpdateMe PATCH /me — allows password change.
func (h *Handler) UpdateMe(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return apperrors.Validation("current_password and new_password are required")
	}

	if err := h.svc.ChangePassword(c.Context(), service.ChangePasswordParams{
		UserID:          userID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "password updated"})
}

func profileResponse(u *domain.User) fiber.Map {
	m := fiber.Map{
		"id":             u.ID,
		"email":          u.Email,
		"role":           string(u.Role),
		"is_active":      u.IsActive,
		"email_verified": u.EmailVerified,
		"mfa_enabled":    u.MFAEnabled,
		"created_at":     u.CreatedAt,
		"updated_at":     u.UpdatedAt,
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		m["locked_until"] = u.LockedUntil
	}
	return m
}

// ── helpers ───────────────────────────────────────────────────────────────────

func tokenResponse(p *service.TokenPair) fiber.Map {
	return fiber.Map{
		"access_token":  p.AccessToken,
		"refresh_token": p.RefreshToken,
		"expires_in":    p.ExpiresIn,
		"token_type":    "Bearer",
	}
}
