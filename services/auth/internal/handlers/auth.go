package handlers

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"github.com/tenzoshare/tenzoshare/services/auth/internal/domain"
	"github.com/tenzoshare/tenzoshare/services/auth/internal/service"
	apperrors "github.com/tenzoshare/tenzoshare/shared/pkg/errors"
	"github.com/tenzoshare/tenzoshare/shared/pkg/middleware"
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

	user, err := h.svc.Register(c.Context(), req.Email, req.Password, c.IP())
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

	pair, user, mfaSetupRequired, err := h.svc.Login(c.Context(), req.Email, req.Password, c.IP())
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

	resp := tokenResponse(pair)
	if mfaSetupRequired {
		// The token issued here is setup-only (MFASetupRequired=true claim, no refresh).
		// Remove the empty refresh_token from the response so clients don't try to store it.
		delete(resp, "refresh_token")
		resp["mfa_setup_required"] = true
	}
	return c.JSON(resp)
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

	// Blacklist the current access token so it cannot be reused within its remaining TTL.
	if claims, ok := c.Locals("claims").(*middleware.Claims); ok && claims != nil {
		_ = h.svc.RevokeAccessToken(c.Context(), claims.JTI)
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

	// Prevent the TOTP secret from being cached by browsers or intermediaries.
	c.Set("Cache-Control", "no-store")
	c.Set("Pragma", "no-cache")
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

	pair, err := h.svc.VerifyMFA(c.Context(), userID, req.OTPCode)
	if err != nil {
		return err
	}

	// Return full tokens so clients coming from the mfa_setup_required flow
	// can immediately start a real session without re-authenticating.
	resp := tokenResponse(pair)
	resp["mfa_enabled"] = true
	return c.JSON(resp)
}

// ── MFA disable ───────────────────────────────────────────────────────────────

type mfaDisableRequest struct {
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}

func (h *Handler) MFADisable(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	var req mfaDisableRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	if err := h.svc.DisableMFA(c.Context(), userID, req.OTPCode); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"mfa_enabled": false})
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
	_, _, _ = h.svc.RequestPasswordReset(c.Context(), req.Email, c.IP())
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "if that email is registered, a reset link has been sent",
	})
}

// ── Email verification ────────────────────────────────────────────────────────

// VerifyEmail GET /auth/verify-email?token=... (public)
func (h *Handler) VerifyEmail(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return apperrors.BadRequest("token is required")
	}
	if err := h.svc.VerifyEmail(c.Context(), token); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "email verified successfully"})
}

type resendVerificationBody struct {
	Email string `json:"email" validate:"required,email"`
}

// ResendVerification POST /auth/resend-verification (public — always 202)
func (h *Handler) ResendVerification(c fiber.Ctx) error {
	var req resendVerificationBody
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}
	// Always 202 regardless of whether the email exists or is already verified.
	_ = h.svc.ResendVerificationEmail(c.Context(), req.Email)
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "if that email is registered and unverified, a verification link has been sent",
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

	// Revoke the current access token so the client must log in again with the new password.
	if claims, ok := c.Locals("claims").(*middleware.Claims); ok && claims != nil {
		_ = h.svc.RevokeAccessToken(c.Context(), claims.JTI)
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
		"date_format":    u.DateFormat,
		"time_format":    u.TimeFormat,
		"timezone":       u.Timezone,
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		m["locked_until"] = u.LockedUntil
	}
	return m
}

// UpdatePreferences PATCH /me/preferences — stores per-user date/time prefs.
func (h *Handler) UpdatePreferences(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	var req struct {
		DateFormat *string `json:"date_format"`
		TimeFormat *string `json:"time_format"`
		Timezone   *string `json:"timezone"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	if err := h.svc.UpdatePreferences(c.Context(), userID, req.DateFormat, req.TimeFormat, req.Timezone); err != nil {
		return err
	}
	user, err := h.svc.GetMe(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(profileResponse(user))
}

// ── API key management ────────────────────────────────────────────────────────

type createAPIKeyRequest struct {
	Name      string  `json:"name"       validate:"required,min=1,max=100"`
	ExpiresAt *string `json:"expires_at"` // optional RFC3339; nil = no expiry
}

func (h *Handler) CreateAPIKey(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	var req createAPIKeyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if err := validate.Struct(req); err != nil {
		return apperrors.Validation(err.Error())
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return apperrors.BadRequest("expires_at must be RFC3339 (e.g. 2027-01-01T00:00:00Z)")
		}
		if t.Before(time.Now()) {
			return apperrors.BadRequest("expires_at must be in the future")
		}
		expiresAt = &t
	}

	result, err := h.svc.CreateAPIKey(c.Context(), userID, req.Name, expiresAt)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         result.ID,
		"name":       result.Name,
		"key":        result.RawKey, // shown once — client must save it
		"key_prefix": result.KeyPrefix,
		"expires_at": result.ExpiresAt,
		"created_at": result.CreatedAt,
	})
}

func (h *Handler) ListAPIKeys(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}

	keys, err := h.svc.ListAPIKeys(c.Context(), userID)
	if err != nil {
		return err
	}

	out := make([]fiber.Map, 0, len(keys))
	for _, k := range keys {
		out = append(out, fiber.Map{
			"id":         k.ID,
			"name":       k.Name,
			"key_prefix": k.KeyPrefix,
			"last_used":  k.LastUsed,
			"expires_at": k.ExpiresAt,
			"created_at": k.CreatedAt,
		})
	}
	return c.JSON(fiber.Map{"api_keys": out})
}

func (h *Handler) DeleteAPIKey(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	id := c.Params("id")
	if id == "" {
		return apperrors.BadRequest("missing api key id")
	}
	if err := h.svc.DeleteAPIKey(c.Context(), id, userID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
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

// ── Notification preferences & unsubscribe ────────────────────────────────────

// Unsubscribe GET /auth/unsubscribe?token=:token (public — no JWT required)
// Validates the HMAC-signed token, sets notifications_opt_out=true for the user.
func (h *Handler) Unsubscribe(c fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return apperrors.BadRequest("missing token")
	}
	email, ok := h.svc.ValidateUnsubscribeToken(token)
	if !ok {
		return apperrors.BadRequest("invalid or tampered unsubscribe token")
	}
	if err := h.svc.Unsubscribe(c.Context(), email); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "You have been unsubscribed from email notifications."})
}

// GetNotificationPrefs GET /auth/me/notification-prefs (authenticated)
func (h *Handler) GetNotificationPrefs(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	user, err := h.svc.GetMe(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"notifications_opt_out": user.NotificationsOptOut,
		"transfer_received":     user.NotificationPrefs.TransferReceived,
		"download_notification": user.NotificationPrefs.DownloadNotification,
		"expiry_reminders":      user.NotificationPrefs.ExpiryReminders,
	})
}

// UpdateNotificationPrefs PATCH /auth/me/notification-prefs (authenticated)
func (h *Handler) UpdateNotificationPrefs(c fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	if userID == "" {
		return apperrors.Unauthorized("unauthenticated")
	}
	var req struct {
		TransferReceived     *bool `json:"transfer_received"`
		DownloadNotification *bool `json:"download_notification"`
		ExpiryReminders      *bool `json:"expiry_reminders"`
		NotificationsOptOut  *bool `json:"notifications_opt_out"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid JSON")
	}
	// Handle the global opt-out toggle first (requires email lookup).
	if req.NotificationsOptOut != nil {
		user, err := h.svc.GetMe(c.Context(), userID)
		if err != nil {
			return err
		}
		if *req.NotificationsOptOut {
			if err := h.svc.Unsubscribe(c.Context(), user.Email); err != nil {
				return err
			}
		} else {
			if err := h.svc.Resubscribe(c.Context(), user.Email); err != nil {
				return err
			}
		}
	}
	// Build prefs from current state + request overrides.
	user, err := h.svc.GetMe(c.Context(), userID)
	if err != nil {
		return err
	}
	prefs := user.NotificationPrefs
	if req.TransferReceived != nil {
		prefs.TransferReceived = *req.TransferReceived
	}
	if req.DownloadNotification != nil {
		prefs.DownloadNotification = *req.DownloadNotification
	}
	if req.ExpiryReminders != nil {
		prefs.ExpiryReminders = *req.ExpiryReminders
	}
	if err := h.svc.UpdateNotificationPrefs(c.Context(), userID, prefs); err != nil {
		return err
	}
	user2, err := h.svc.GetMe(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"notifications_opt_out": user2.NotificationsOptOut,
		"transfer_received":     user2.NotificationPrefs.TransferReceived,
		"download_notification": user2.NotificationPrefs.DownloadNotification,
		"expiry_reminders":      user2.NotificationPrefs.ExpiryReminders,
	})
}

// InternalNotificationPrefs GET /api/v1/auth/internal/notification-prefs?email=:email
// Internal-only endpoint (no JWT) for the notification service to check prefs before sending.
func (h *Handler) InternalNotificationPrefs(c fiber.Ctx) error {
	email := c.Query("email")
	if email == "" {
		return apperrors.BadRequest("missing email")
	}
	optOut, prefs, err := h.svc.GetNotificationStatus(c.Context(), email)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"opt_out":               optOut,
		"transfer_received":     prefs.TransferReceived,
		"download_notification": prefs.DownloadNotification,
		"expiry_reminders":      prefs.ExpiryReminders,
	})
}
