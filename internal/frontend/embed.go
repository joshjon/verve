package frontend

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	indexPage       = "index.html"
	placeholderPage = "placeholder.html"
)

//go:embed all:dist
var dist embed.FS

// Dist returns a read-only file system of the UI static files.
// If the full UI build is not present (index.html missing), it falls back to
// serving placeholder.html as the index page.
func Dist() (fs.FS, error) {
	basePath := "dist"

	subFS, err := fs.Sub(dist, basePath)
	if err != nil {
		return nil, fmt.Errorf("create sub filesystem: %w", err)
	}

	// Check if the full UI build is present.
	if f, err := subFS.Open(indexPage); err == nil {
		_ = f.Close()
		return subFS, nil
	}

	// Fall back to placeholder if no full build exists.
	f, err := subFS.Open(placeholderPage)
	if err != nil {
		return nil, fmt.Errorf("ui build not found: missing %s and %s (did you run `make ui-build-go`?): %w", indexPage, placeholderPage, err)
	}
	_ = f.Close()

	return &placeholderFS{sub: subFS}, nil
}

// placeholderFS wraps a filesystem and serves placeholder.html when
// index.html is requested.
type placeholderFS struct {
	sub fs.FS
}

func (p *placeholderFS) Open(name string) (fs.File, error) {
	if name == indexPage {
		return p.sub.Open(placeholderPage)
	}
	return p.sub.Open(name)
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
			defer func() { _ = index.Close() }()
			return c.Stream(http.StatusOK, echo.MIMETextHTMLCharsetUTF8, index)
		}
		return nil
	}, nil
}
