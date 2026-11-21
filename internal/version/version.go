package version

// 打包时用 -ldflags -X 覆盖；本地默认 dev。
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)
