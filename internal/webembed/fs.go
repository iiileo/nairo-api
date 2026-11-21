package webembed

import (
	"embed"
	"io/fs"
	"os"

	"nairo/api_service/internal/config"
)

// data/* 由 `make sync-web` 从 index_web / console_web/dist / admin_web/dist 同步而来。
// 发布构建前必须先 sync；本地 WEB_DEV=1 时直接读源目录，可不 sync。
//
//go:embed all:data/index
var indexEmbed embed.FS

//go:embed all:data/console
var consoleEmbed embed.FS

//go:embed all:data/admin
var adminEmbed embed.FS

type Bundles struct {
	Index   fs.FS
	Console fs.FS
	Admin   fs.FS
}

func NewBundles(cfg *config.Config) (*Bundles, error) {
	if cfg.Web.Dev {
		return &Bundles{
			Index:   os.DirFS(cfg.Web.IndexDir),
			Console: os.DirFS(cfg.Web.ConsoleDir),
			Admin:   os.DirFS(cfg.Web.AdminDir),
		}, nil
	}

	indexFS, err := fs.Sub(indexEmbed, "data/index")
	if err != nil {
		return nil, err
	}
	consoleFS, err := fs.Sub(consoleEmbed, "data/console")
	if err != nil {
		return nil, err
	}
	adminFS, err := fs.Sub(adminEmbed, "data/admin")
	if err != nil {
		return nil, err
	}
	return &Bundles{Index: indexFS, Console: consoleFS, Admin: adminFS}, nil
}
