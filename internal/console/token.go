package console

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"nairo/api_service/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

const (
	claimTypAccess  = "cs"
	claimTypRefresh = "cr"
)

type AccessClaims struct {
	UserID int64  `json:"uid"`
	Typ    string `json:"typ"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID int64  `json:"uid"`
	Typ    string `json:"typ"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  int64
	RefreshExpiresIn int64
	RefreshJTI       string
	RefreshExpiresAt time.Time
}

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(cfg *config.Config) *TokenManager {
	access := cfg.Console.AccessTTLSeconds
	if access <= 0 {
		access = 900 // 15m
	}
	refresh := cfg.Console.RefreshTTLSeconds
	if refresh <= 0 {
		refresh = 7 * 24 * 3600
	}
	secret := cfg.Console.JWTSecret
	if secret == "" {
		secret = cfg.Admin.JWTSecret
	}
	return &TokenManager{
		secret:     []byte(secret),
		accessTTL:  time.Duration(access) * time.Second,
		refreshTTL: time.Duration(refresh) * time.Second,
	}
}

func (m *TokenManager) AccessTTLSeconds() int64  { return int64(m.accessTTL.Seconds()) }
func (m *TokenManager) RefreshTTLSeconds() int64 { return int64(m.refreshTTL.Seconds()) }

func (m *TokenManager) IssuePair(userID int64) (*TokenPair, error) {
	now := time.Now().UTC()
	access, err := m.issueAccess(userID, now)
	if err != nil {
		return nil, err
	}
	jti, err := randomJTI()
	if err != nil {
		return nil, err
	}
	refreshExp := now.Add(m.refreshTTL)
	refreshClaims := RefreshClaims{
		UserID: userID,
		Typ:    claimTypRefresh,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   fmt.Sprintf("user:%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExp),
		},
	}
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refresh, err := rt.SignedString(m.secret)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresIn:  m.AccessTTLSeconds(),
		RefreshExpiresIn: m.RefreshTTLSeconds(),
		RefreshJTI:       jti,
		RefreshExpiresAt: refreshExp,
	}, nil
}

func (m *TokenManager) issueAccess(userID int64, now time.Time) (string, error) {
	claims := AccessClaims{
		UserID: userID,
		Typ:    claimTypAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("user:%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(m.secret)
}

func (m *TokenManager) ParseAccess(token string) (*AccessClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &AccessClaims{}, m.keyFunc)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*AccessClaims)
	if !ok || !parsed.Valid || claims.Typ != claimTypAccess {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func (m *TokenManager) ParseRefresh(token string) (*RefreshClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &RefreshClaims{}, m.keyFunc)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*RefreshClaims)
	if !ok || !parsed.Valid || claims.Typ != claimTypRefresh {
		return nil, fmt.Errorf("invalid refresh token")
	}
	if claims.JTI == "" {
		claims.JTI = claims.ID
	}
	return claims, nil
}

func (m *TokenManager) keyFunc(t *jwt.Token) (any, error) {
	if t.Method != jwt.SigningMethodHS256 {
		return nil, fmt.Errorf("unexpected signing method")
	}
	return m.secret, nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
