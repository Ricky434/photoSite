package web

import (
	"net/http"
	"sitoWow/internal/data"
	"sitoWow/internal/data/models"
)

func (app *Application) homePage(w http.ResponseWriter, r *http.Request) {
	tdata := app.newTemplateData(r)

	filters := data.Filters{
		Page:         1,
		PageSize:     100,
		Sort:         "taken_at",
		SortSafelist: []string{"taken_at"},
		//TODO: aggiungere groupby
	}

	photos, metadata, err := app.Models.Photos.GetAll(nil, filters)
	if err != nil {
		app.serverError(w, r, err)
	}

	tdata.PhotosByEvent = make(map[string][]*models.Photo)

	for _, p := range photos {
		if p.EventName != nil {
			tdata.PhotosByEvent[*p.EventName] = append(tdata.PhotosByEvent[*p.EventName], p)
		} else {
			tdata.PhotosByEvent[""] = append(tdata.PhotosByEvent[""], p)
		}
	}

	tdata.Metadata = &metadata

	//controlla todo, eventi getall/id/name
	app.render(w, r, http.StatusOK, "home.tmpl", tdata)
}
