package v1

import (
	"errors"
	"net/http"
	"strconv"

	"nairo/api_service/internal/api/middleware"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminUserHandler struct {
	users *service.UserAdminService
	accts *service.AccountService
}

func NewAdminUserHandler(users *service.UserAdminService, accts *service.AccountService) *AdminUserHandler {
	return &AdminUserHandler{users: users, accts: accts}
}

type createUserReq struct {
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Password   string `json:"password" binding:"required"`
	FeeRateBps int    `json:"fee_rate_bps"`
}

func (h *AdminUserHandler) Create(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	res, err := h.users.CreateUser(c.Request.Context(), service.CreateUserInput{
		Email: req.Email, Phone: req.Phone, Password: req.Password, FeeRateBps: req.FeeRateBps,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            formatInt64(res.User.ID),
			"email":         res.User.Email,
			"phone":         res.User.Phone,
			"fee_rate_bps":  res.User.FeeRateBps,
			"status":        res.User.Status,
			"adjust_enabled": res.User.AdjustEnabled,
			"adjust_direction": res.User.AdjustDirection,
		},
		"access_key": res.AccessKey,
		"api_secret": res.APISecret,
	})
}

func (h *AdminUserHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	list, err := h.users.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, u := range list {
		items = append(items, gin.H{
			"id": formatInt64(u.ID), "email": u.Email, "phone": u.Phone,
			"fee_rate_bps": u.FeeRateBps, "status": u.Status,
			"adjust_direction": u.AdjustDirection,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type setFeeReq struct {
	FeeRateBps int `json:"fee_rate_bps"`
}

func (h *AdminUserHandler) SetFee(c *gin.Context) {
	uid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req setFeeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	u, err := h.users.SetFeeRate(c.Request.Context(), uid, req.FeeRateBps)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": formatInt64(u.ID), "fee_rate_bps": u.FeeRateBps})
}

type updateUserReq struct {
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Password *string `json:"password"`
	Status   *string `json:"status"`
}

func (h *AdminUserHandler) Update(c *gin.Context) {
	uid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	u, err := h.users.UpdateUser(c.Request.Context(), uid, service.UpdateUserInput{
		Email: req.Email, Phone: req.Phone, Password: req.Password, Status: req.Status,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, service.ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "email_taken"})
		case errors.Is(err, service.ErrInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": formatInt64(u.ID), "email": u.Email, "phone": u.Phone,
		"status": u.Status, "fee_rate_bps": u.FeeRateBps,
	})
}

type setPasswordReq struct {
	Password string `json:"password" binding:"required"`
}

func (h *AdminUserHandler) SetPassword(c *gin.Context) {
	uid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req setPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	u, err := h.users.SetPassword(c.Request.Context(), uid, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, service.ErrInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": formatInt64(u.ID), "ok": true})
}

type rechargeReq struct {
	AmountCents int64  `json:"amount_cents" binding:"required"`
	Remark      string `json:"remark"`
}

func (h *AdminUserHandler) Recharge(c *gin.Context) {
	uid, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	var req rechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	adminID, _ := c.Get(middleware.CtxAdminID)
	aid, _ := adminID.(int64)
	w, err := h.users.Recharge(c.Request.Context(), uid, aid, req.AmountCents, req.Remark)
	if err != nil {
		if errors.Is(err, service.ErrInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": formatInt64(uid),
		"balance_cents": w.BalanceCents,
	})
}

type createAccountReq struct {
	UserID   string   `json:"user_id" binding:"required"`
	Channels []string `json:"channels"`
	Channel  string   `json:"channel"`
	Name     string   `json:"name"`
}

func (h *AdminUserHandler) CreateAccount(c *gin.Context) {
	var req createAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	uid, err := parseID(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_user_id"})
		return
	}
	res, err := h.accts.CreateAccount(c.Request.Context(), service.CreateAccountInput{
		UserID: uid, Channels: req.Channels, Channel: req.Channel, Name: req.Name,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"account": gin.H{
			"id": formatInt64(res.Account.ID),
			"user_id": formatInt64(res.Account.UserID),
			"channel": res.Account.Channel,
			"channels": res.Channels,
			"name": res.Account.Name,
			"callback_id": res.Account.CallbackID,
		},
		"app_secret": res.AppSecret,
		"callback_url": res.CallbackURL,
		"bind": res.Bind,
	})
}

type createPayCodeReq struct {
	AccountID   string `json:"account_id" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
	CodeType    string `json:"code_type"`
	AmountCents *int   `json:"amount_cents"`
	ImageURL    string `json:"image_url"`
	Payload     string `json:"payload"`
}

func (h *AdminUserHandler) CreatePayCode(c *gin.Context) {
	var req createPayCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	aid, err := parseID(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_account_id"})
		return
	}
	code, err := h.accts.CreatePayCode(c.Request.Context(), service.CreatePayCodeInput{
		AccountID: aid, Channel: req.Channel, CodeType: req.CodeType, AmountCents: req.AmountCents,
		ImageURL: req.ImageURL, Payload: req.Payload,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": formatInt64(code.ID),
		"account_id": formatInt64(code.AccountID),
		"channel": code.Channel,
		"code_type": code.CodeType,
		"amount_cents": code.AmountCents,
		"image_url": code.ImageURL,
		"status": code.Status,
	})
}
