package web

import (
	"net/http"
)

func (app *Application) homePage(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	app.render(w, r, http.StatusOK, "home.tmpl", data)
}
