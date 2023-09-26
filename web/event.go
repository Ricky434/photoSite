package web

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *Application) eventPage(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())

	name := params.ByName("name")

	tdata := app.newTemplateData(r)

	event, err := app.Models.Events.GetByName(name)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tdata.Event = event

	app.render(w, r, http.StatusOK, "event.tmpl", tdata)
}
