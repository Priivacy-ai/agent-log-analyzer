package main

import (
	"path/filepath"
	"sort"
	"strings"
)

func staticWebRoot() string {
	return existingDir("web-dist", "web", "../../web-dist", "../../web")
}

func docsWebRoot() string {
	return existingDir("docs", "web/docs", "web-dist/docs", "../../docs")
}

func staticAssetPath(asset string) string {
	return staticAssetPathFromRoot(staticWebRoot(), asset)
}

func staticAssetPathFromRoot(root, asset string) string {
	clean := strings.TrimPrefix(asset, "/")
	if hashed, ok := hashedStaticAssetPath(root, clean); ok {
		return hashed
	}
	return "/" + clean
}

func hashedStaticAssetPath(root, clean string) (string, bool) {
	base := filepath.Base(clean)
	ext := filepath.Ext(base)
	if ext == "" {
		return "", false
	}
	stem := strings.TrimSuffix(base, ext)
	matches, err := filepath.Glob(filepath.Join(root, "assets", stem+"-*"+ext))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	return "/assets/" + filepath.Base(matches[0]), true
}
