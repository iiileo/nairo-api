package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"nairo/api_service/ent"
	"nairo/api_service/internal/config"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/snowflake"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/fx"
)

type Client struct {
	ent *ent.Client
	db  *sql.DB
}

func NewEntClient(lc fx.Lifecycle, cfg *config.Config, log *logger.Logger) (*Client, error) {
	if err := snowflake.Configure(cfg.Snowflake.NodeID); err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", cfg.MySQL.DSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	drv := entsql.OpenDB(dialect.MySQL, db)
	entClient := ent.NewClient(ent.Driver(drv))
	c := &Client{ent: entClient, db: db}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info(ctx, "running ent schema migrate",
				"host", cfg.MySQL.Host,
				"dbname", cfg.MySQL.DBName,
			)
			if err := c.ent.Schema.Create(ctx); err != nil {
				return fmt.Errorf("ent migrate: %w", err)
			}
			if err := applyPatches(ctx, c.db); err != nil {
				return fmt.Errorf("mysql patches: %w", err)
			}
			if err := syncComments(ctx, c.db, cfg.MySQL.DBName); err != nil {
				return fmt.Errorf("sync mysql comments: %w", err)
			}
			log.Info(ctx, "mysql connected and migrated")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info(ctx, "closing mysql")
			return c.Close()
		},
	})
	return c, nil
}

func (c *Client) Ent() *ent.Client {
	if c == nil {
		return nil
	}
	return c.ent
}

func (c *Client) DB() *sql.DB {
	if c == nil {
		return nil
	}
	return c.db
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.ent != nil {
		err = c.ent.Close()
	}
	if c.db != nil {
		if e := c.db.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (c *Client) WithTx(ctx context.Context, fn func(tx *ent.Tx) error) error {
	tx, err := c.ent.Tx(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("%w: rollback: %v", err, rerr)
		}
		return err
	}
	return tx.Commit()
}

// applyPatches：ent Schema.Create 不会扩大已有列，这里补齐兼容性变更。
func applyPatches(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE payment_accounts MODIFY COLUMN app_conn_type VARCHAR(16) NULL COMMENT '连接类型：http/ws'`,
		`ALTER TABLE platform_settings MODIFY COLUMN value LONGTEXT NOT NULL COMMENT '配置值（可存 JSON）'`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			// 表尚未创建等场景忽略；重复执行也应幂等成功
			if strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
				continue
			}
			return err
		}
	}
	return nil
}
