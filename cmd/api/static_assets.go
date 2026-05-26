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
	switch clean {
	case "styles.css":
		matches, err := filepath.Glob(filepath.Join(root, "assets", "styles-*.css"))
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			return "/assets/" + filepath.Base(matches[0])
		}
	case "vendor/tippy/tippy.css":
		matches, err := filepath.Glob(filepath.Join(root, "assets", "tippy-*.css"))
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			return "/assets/" + filepath.Base(matches[0])
		}
	}
	return "/" + clean
}
