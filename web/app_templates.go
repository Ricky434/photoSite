package web

import (
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"sitoWow/internal/data"
	"sitoWow/internal/data/models"
	"sitoWow/internal/validator"
	"sitoWow/ui"
	"slices"
	"strings"

	"github.com/justinas/nosurf"
)

type TemplateData struct {
	Form            any
	Validator       *validator.Validator // use when you only need errors without a form (htmx)
	Flash           string
	IsAuthenticated bool
	CSRFToken       string
	Event           *models.Event
	Events          []*models.Event
	Photo           *models.Photo
	Photos          []*models.Photo
	PhotosByEvent   map[int][]*models.Photo
	Metadata        *data.Metadata
}

var functions = template.FuncMap{
	"Add":     func(a, b int) int { return a + b },
	"Modulo":  func(a, b, c int) bool { return a%b == c },
	"isVideo": func(f string) bool { return slices.Contains(models.VideoExtensions, strings.ToLower(path.Ext(f))) },
}

func NewTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	pages, err := fs.Glob(ui.Files, "html/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		patterns := []string{
			"html/base.tmpl",
			"html/partials/*.tmpl",
			page,
		}

		ts, err := template.New(name).Funcs(functions).ParseFS(ui.Files, patterns...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	return cache, nil
}

func (app *Application) newTemplateData(r *http.Request) *TemplateData {
	return &TemplateData{
		Flash:           app.SessionManager.PopString(r.Context(), "flash"),
		IsAuthenticated: app.IsAuthenticated(r),
		CSRFToken:       nosurf.Token(r),
	}
}
