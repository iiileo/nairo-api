package service

import (
	"context"
	"strings"

	"nairo/api_service/ent"
	"nairo/api_service/ent/admin"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secrethash"
)

type AdminAuthService struct {
	db *mysql.Client
}

func NewAdminAuthService(db *mysql.Client) *AdminAuthService {
	return &AdminAuthService{db: db}
}

func (s *AdminAuthService) Login(ctx context.Context, username, password string) (*ent.Admin, error) {
	username = strings.TrimSpace(username)
	a, err := s.db.Ent().Admin.Query().Where(admin.UsernameEQ(username)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if a.Status != "active" || !secrethash.CheckPassword(a.PasswordHash, password) {
		return nil, ErrUnauthorized
	}
	return a, nil
}
