package web

import (
	"net/http"
	"sitoWow/internal/data"
	"sitoWow/internal/validator"
)

func (app *Application) photoList(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Event int32
		data.Filters
	}

	v := &validator.Validator{}

	qs := r.URL.Query()

	input.Event = int32(app.readInt(qs, "event", 0, v))
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 10, v)
	input.Filters.Sort = app.readString(qs, "sort", "taken_at")

	input.Filters.SortSafelist = []string{"id", "-id", "taken_at", "-taken_at", "latitude", "-latitude", "longitude", "-longitude"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		data := app.newTemplateData(r)
		// TODO hack, trovare un modo per mostrare errori
		// penso che nei casi come questo, in cui viene restituito solo un pezzo di html e non una pagina intera,
		// non bisogna redirectare alla pagina che ha fatto la richiesta (perche' non dovrebbe esistere, solo htmx dovrebbe farla)
		// quindi bisogna restituire html che contiene tutti gli errori, che htmx puo far visualizzare al posto della risposta.
		// L'html degli errori per essere visualizzato da htmx deve avere codice 200
		// Forse non dovrei proprio restituire errori
		data.Validator = v
		app.renderRaw(w, r, http.StatusOK, "errors.tmpl", data)
		return
	}

	event, err := app.Models.Events.GetByID(input.Event)
	if err != nil {
		//TODO check other errors
		app.serverErrorHTMX(w, r, err)
		return
	}

	photos, metadata, err := app.Models.Photos.GetAll(&input.Event, input.Filters)
	if err != nil {
		//TODO check other errors
		app.serverErrorHTMX(w, r, err)
		return
	}

	tdata := app.newTemplateData(r)
	tdata.Photos = photos
	tdata.Event = event
	tdata.Metadata = &metadata

	app.renderRaw(w, r, http.StatusOK, "photoList.tmpl", tdata)
}

func (app *Application) photoPage(w http.ResponseWriter, r *http.Request) {
}
