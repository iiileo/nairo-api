package admin

import (
	"fmt"
	"time"

	"nairo/api_service/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

const claimTypAdmin = "ad"

type AccessClaims struct {
	AdminID  int64  `json:"aid"`
	Username string `json:"username"`
	Typ      string `json:"typ"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenManager(cfg *config.Config) *TokenManager {
	ttl := cfg.Admin.AccessTTLSeconds
	if ttl <= 0 {
		ttl = 86400
	}
	return &TokenManager{
		secret:    []byte(cfg.Admin.JWTSecret),
		accessTTL: time.Duration(ttl) * time.Second,
	}
}

func (m *TokenManager) IssueAccess(adminID int64, username string) (string, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		AdminID:  adminID,
		Username: username,
		Typ:      claimTypAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("admin:%d", adminID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(m.secret)
}

func (m *TokenManager) ParseAccess(token string) (*AccessClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &AccessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.Typ != claimTypAdmin {
		return nil, fmt.Errorf("not admin token")
	}
	return claims, nil
}
