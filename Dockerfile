# syntax=docker/dockerfile:1.7

# ---------- 前端：console / admin ----------
FROM node:22-bookworm-slim AS web
WORKDIR /src
RUN corepack enable
COPY console_web/package.json console_web/pnpm-lock.yaml ./console_web/
COPY admin_web/package.json admin_web/pnpm-lock.yaml ./admin_web/
RUN cd console_web && pnpm install --frozen-lockfile \
 && cd ../admin_web && pnpm install --frozen-lockfile
COPY console_web ./console_web
COPY admin_web ./admin_web
COPY index_web ./index_web
ARG CONSOLE_BASE=/console/
ARG ADMIN_BASE=/admin_nairo/
RUN cd console_web && BASE_PATH="${CONSOLE_BASE}" pnpm build \
 && cd ../admin_web && BASE_PATH="${ADMIN_BASE}" pnpm build

# ---------- Go 编译（embed 静态资源）----------
FROM golang:1.25-bookworm AS builder
WORKDIR /src
ARG VERSION=dev
ARG COMMIT=none
ARG BUILDTIME=unknown
ENV CGO_ENABLED=0 GOTOOLCHAIN=local \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/index_web ./index_web
COPY --from=web /src/console_web/dist ./console_web/dist
COPY --from=web /src/admin_web/dist ./admin_web/dist
RUN rm -rf internal/webembed/data/index internal/webembed/data/console internal/webembed/data/admin \
 && mkdir -p internal/webembed/data/index internal/webembed/data/console internal/webembed/data/admin \
 && cp -R index_web/. internal/webembed/data/index/ \
 && cp -R console_web/dist/. internal/webembed/data/console/ \
 && cp -R admin_web/dist/. internal/webembed/data/admin/ \
 && go build -trimpath \
      -ldflags "-s -w -X nairo/api_service/internal/version.Version=${VERSION} -X nairo/api_service/internal/version.Commit=${COMMIT} -X nairo/api_service/internal/version.BuildTime=${BUILDTIME}" \
      -o /out/api_service ./cmd/server

# ---------- 运行时 ----------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /
COPY --from=builder /out/api_service /api_service
ENV SERVER_ADDRESS=:8088 \
    SERVER_MODE=prod \
    WEB_DEV=false \
    TZ=Asia/Shanghai \
    APP_TIMEZONE=Asia/Shanghai
EXPOSE 8088
USER nonroot:nonroot
ENTRYPOINT ["/api_service"]
