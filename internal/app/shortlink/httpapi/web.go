package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"day.local/gee"
)

//go:embed static/*
var staticFS embed.FS

// RegisterWebRoutes mounts the built-in UI for the shortlink service.
// Static files are embedded from the static/ directory (built by Astro).
func RegisterWebRoutes(r *gee.Engine) {
	// Get the static subdirectory
	staticRoot, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("failed to get static subdirectory: " + err.Error())
	}

	// Serve index.html for root path
	r.GET("/", func(ctx *gee.Context) {
		data, err := fs.ReadFile(staticRoot, "index.html")
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, "index.html not found")
			return
		}
		ctx.SetHeader("Content-Type", "text/html; charset=utf-8")
		ctx.Data(http.StatusOK, data)
	})

	// Serve favicon
	r.GET("/favicon.svg", func(ctx *gee.Context) {
		data, err := fs.ReadFile(staticRoot, "favicon.svg")
		if err != nil {
			ctx.Status(http.StatusNoContent)
			return
		}
		ctx.SetHeader("Content-Type", "image/svg+xml")
		ctx.Data(http.StatusOK, data)
	})

	// Serve _astro/* static assets (CSS, JS)
	r.GET("/_astro/*filepath", func(ctx *gee.Context) {
		filepath := ctx.Param("filepath")
		data, err := fs.ReadFile(staticRoot, "_astro/"+filepath)
		if err != nil {
			ctx.AbortWithError(http.StatusNotFound, "file not found")
			return
		}

		// Set content type based on extension
		contentType := "application/octet-stream"
		if strings.HasSuffix(filepath, ".js") {
			contentType = "application/javascript; charset=utf-8"
		} else if strings.HasSuffix(filepath, ".css") {
			contentType = "text/css; charset=utf-8"
		}

		// Cache static assets (they have hashed filenames)
		ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
		ctx.SetHeader("Content-Type", contentType)
		ctx.Data(http.StatusOK, data)
	})

	// Avoid noisy 404s for favicon.ico
	r.GET("/favicon.ico", func(ctx *gee.Context) {
		ctx.Status(http.StatusNoContent)
	})
}
