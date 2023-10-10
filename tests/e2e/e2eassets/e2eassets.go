package e2eassets

import (
	"embed"
)

//go:embed assets/*
var AssetsFS embed.FS
