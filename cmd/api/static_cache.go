package main

import (
	"net/http"
	"path"
	"strings"
)

func staticFileServer(root string) http.Handler {
	return withStaticCacheHeaders(http.FileServer(http.Dir(root)))
}

func withStaticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cacheControl := staticCacheControl(r.URL.Path); cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		next.ServeHTTP(w, r)
	})
}

func staticCacheControl(requestPath string) string {
	cleanPath := "/" + strings.TrimLeft(requestPath, "/")
	if strings.HasPrefix(cleanPath, "/assets/") {
		return "public, max-age=31536000, immutable"
	}

	switch strings.ToLower(path.Ext(cleanPath)) {
	case ".css", ".js", ".mjs", ".webp", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2":
		return "public, max-age=86400, stale-while-revalidate=604800"
	case ".html", "":
		return "no-cache"
	default:
		return ""
	}
}
