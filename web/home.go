package web

import (
	"net/http"
	"sitoWow/internal/data"
	"sitoWow/internal/data/models"
)

func (app *Application) homePage(w http.ResponseWriter, r *http.Request) {
	tdata := app.newTemplateData(r)

	filtersEvents := data.Filters{
		Page:         1,
		PageSize:     1000,
		Sort:         "day",
		SortSafelist: []string{"day"},
	}

	events, err := app.Models.Events.GetAll(filtersEvents)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	tdata.Events = events

	photos, err := app.Models.Photos.Summary(10)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tdata.PhotosByEvent = make(map[int][]*models.Photo)

	// Since they are already ordered, they will remain ordered
	for _, p := range photos {
		tdata.PhotosByEvent[p.Event] = append(tdata.PhotosByEvent[p.Event], p)
	}

	app.render(w, r, http.StatusOK, "home.tmpl", tdata)
}
