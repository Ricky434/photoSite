package web

import (
	"fmt"
	"net/http"
	"path"
	"sitoWow/internal/data/models"
	"slices"
	"strings"
)

func (app *Application) homePage(w http.ResponseWriter, r *http.Request) {
	tdata := app.newTemplateData(r)

	events, err := app.Models.Events.GetAll()
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
	for i, p := range photos {
		if slices.Contains(models.VideoExtensions, strings.ToLower(path.Ext(photos[i].FileName))) {
			// Thumbnail for video is video filename(with extension)+".jpg"
			photos[i].ThumbName = fmt.Sprintf("%s%s", path.Base(photos[i].FileName), ".jpg")
		} else {
			photos[i].ThumbName = photos[i].FileName
		}

		tdata.PhotosByEvent[p.Event] = append(tdata.PhotosByEvent[p.Event], p)
	}

	app.render(w, r, http.StatusOK, "home.tmpl", tdata)
}
