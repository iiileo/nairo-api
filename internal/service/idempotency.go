package service

import (
	"context"
	"strings"

	"nairo/api_service/ent"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/snowflake"
)

const (
	IdemScopeAppPush   = "app_push"
	IdemScopeBotNotify = "bot_notify"
)

// claimIdempotency 尝试写入幂等键；已存在返回 false。
func claimIdempotency(ctx context.Context, db *mysql.Client, scope, key, result string) (bool, error) {
	key = strings.TrimSpace(key)
	scope = strings.TrimSpace(scope)
	if key == "" || scope == "" || db == nil {
		return true, nil
	}
	key = truncateUTF8(key, 128)
	id, err := snowflake.NextID()
	if err != nil {
		return false, err
	}
	_, err = db.Ent().IdempotencyRecord.Create().
		SetID(id).
		SetScope(scope).
		SetIdemKey(key).
		SetResult(result).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func claimIdempotencyTx(ctx context.Context, tx *ent.Tx, scope, key, result string) (bool, error) {
	key = strings.TrimSpace(key)
	scope = strings.TrimSpace(scope)
	if key == "" || scope == "" || tx == nil {
		return true, nil
	}
	if len(key) > 128 {
		key = key[:128]
	}
	id, err := snowflake.NextID()
	if err != nil {
		return false, err
	}
	_, err = tx.IdempotencyRecord.Create().
		SetID(id).
		SetScope(scope).
		SetIdemKey(key).
		SetResult(result).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
