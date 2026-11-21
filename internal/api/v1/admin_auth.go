package v1

import (
	"errors"
	"net/http"

	"nairo/api_service/internal/admin"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type AdminAuthHandler struct {
	auth *service.AdminAuthService
	tm   *admin.TokenManager
}

func NewAdminAuthHandler(auth *service.AdminAuthService, tm *admin.TokenManager) *AdminAuthHandler {
	return &AdminAuthHandler{auth: auth, tm: tm}
}

type adminLoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req adminLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	a, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	token, err := h.tm.IssueAccess(a.ID, a.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"admin": gin.H{
			"id":       strconvID(a.ID),
			"username": a.Username,
			"name":     a.Name,
		},
	})
}

func strconvID(id int64) string {
	return formatInt64(id)
}
