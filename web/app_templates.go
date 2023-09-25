package web

import (
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"sitoWow/internal/data"
	"sitoWow/internal/data/models"
	"sitoWow/internal/validator"
	"sitoWow/ui"

	"github.com/justinas/nosurf"
)

type TemplateData struct {
	Form            any
	Validator       *validator.Validator // use when you only need errors without a form (htmx)
	Flash           string
	IsAuthenticated bool
	CSRFToken       string
	Event           *models.Event
	Photo           *models.Photo
	Photos          []*models.Photo
	PhotosByEvent   map[int32][]*models.Photo
	Events          []*models.Event
	Metadata        *data.Metadata
}

var functions = template.FuncMap{
	"Deref": func(i *int32) int32 { return *i },
	"Add":   func(a, b int) int { return a + b },
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
