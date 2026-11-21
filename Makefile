.PHONY: run sync-web build-web generate-ent tidy build test fmt

export GOTOOLCHAIN ?= go1.25.0

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILDTIME:= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X nairo/api_service/internal/version.Version=$(VERSION) \
	-X nairo/api_service/internal/version.Commit=$(COMMIT) \
	-X nairo/api_service/internal/version.BuildTime=$(BUILDTIME)

# 需与 WEB_ADMIN_BASE_PATH 一致（末尾斜杠可选）
ADMIN_BASE_PATH ?= /admin_nairo/
CONSOLE_BASE_PATH ?= /console/

run: ## 本地开发（WEB_DEV，不强制 embed 同步）
	go run ./cmd/server

build-web: ## 按二级目录 base 构建 console / admin
	cd console_web && BASE_PATH=$(CONSOLE_BASE_PATH) pnpm build
	cd admin_web && BASE_PATH=$(ADMIN_BASE_PATH) pnpm build

sync-web: build-web ## 同步前端到 embed 目录
	rm -rf internal/webembed/data/index internal/webembed/data/console internal/webembed/data/admin
	mkdir -p internal/webembed/data/index internal/webembed/data/console internal/webembed/data/admin
	cp -R index_web/. internal/webembed/data/index/
	cp -R console_web/dist/. internal/webembed/data/console/
	cp -R admin_web/dist/. internal/webembed/data/admin/

generate-ent: ## 生成 Ent 代码
	go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

tidy:
	go mod tidy

build: sync-web ## 发布：同步前端 + 单一二进制（embed）
	WEB_DEV=false go build -ldflags "$(LDFLAGS)" -o bin/api_service ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w .
