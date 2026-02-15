package frontend

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
)

const indexPage = "index.html"

//go:embed all:dist
var dist embed.FS

// Dist returns a read-only file system of the UI static files.
func Dist() (fs.FS, error) {
	basePath := "dist"

	f, err := dist.Open(basePath + "/" + indexPage)
	if err != nil {
		return nil, fmt.Errorf("ui build not found: missing %s (did you run `make ui-build-go`?): %w", indexPage, err)
	}
	if err = f.Close(); err != nil {
		return nil, fmt.Errorf("close %s: %w", indexPage, err)
	}

	subFS, err := fs.Sub(dist, basePath)
	if err != nil {
		return nil, fmt.Errorf("create sub filesystem: %w", err)
	}
	return subFS, nil
}

// DistHandler returns an Echo handler for serving static UI files for the
// specified build.
func DistHandler() (echo.HandlerFunc, error) {
	distFS, err := Dist()
	if err != nil {
		return nil, err
	}

	handler := echo.StaticDirectoryHandler(distFS, false)

	return func(c echo.Context) error {
		if err := handler(c); err != nil {
			// SPA fallback: serve index page for client-side routing
			index, err := distFS.Open(indexPage)
			if err != nil {
				return echo.ErrNotFound
			}
			defer index.Close()
			return c.Stream(http.StatusOK, echo.MIMETextHTMLCharsetUTF8, index)
		}
		return nil
	}, nil
}
