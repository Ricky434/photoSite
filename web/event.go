package web

import (
	"errors"
	"net/http"
	"os"
	"path"
	"sitoWow/internal/data/models"
	"sitoWow/internal/validator"
	"time"

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

//type MyDate time.Time
//
//func (d *MyDate) UnmarshalJSON(data []byte) error {
//	s := strings.Trim(string(data), `"`)
//	if s == "" {
//		return nil
//	}
//
//	dt, err := time.Parse(time.DateOnly, s)
//	if err != nil {
//		return err
//	}
//
//	*d = MyDate(dt)
//
//	return nil
//}

type eventCreateForm struct {
	Name                string     `form:"name"`
	Date                *time.Time `form:"-"`
	validator.Validator `form:"-"`
}

func (app *Application) eventsCreatePage(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = eventCreateForm{}
	app.render(w, r, http.StatusOK, "eventCreate.tmpl", data)
}

func (app *Application) eventsCreatePost(w http.ResponseWriter, r *http.Request) {
	var form eventCreateForm

	// TODO perche non va in errore se la data non e' giusta???
	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// shouldnt be manual, should use UnmarshalJSON but it doesnt work
	if r.FormValue("date") != "" {
		timeDate, err := time.Parse(time.DateOnly, r.FormValue("date"))
		if err != nil {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		form.Date = &timeDate
	}

	event := &models.Event{
		Name: form.Name,
		Date: form.Date,
	}

	form.CheckField(event.Name != "", "name", "This field must not be empty")
	form.CheckField(r.FormValue("date") == "" || !form.Date.IsZero(), "date", "Date must be non zero")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, r, http.StatusUnprocessableEntity, "eventCreate.tmpl", data)
		return
	}

	err = app.Models.Events.Insert(event)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateName) {
			form.AddFieldError("name", "This name already exists")

			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, r, http.StatusUnprocessableEntity, "eventCreate.tmpl", data)
			return
		}

		app.serverError(w, r, err)
		return
	}

	app.SessionManager.Put(r.Context(), "flash", "Event created successfully")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type eventDeleteForm struct {
	Event               string `form:"event"`
	validator.Validator `form:"-"`
}

func (app *Application) eventsDeletePage(w http.ResponseWriter, r *http.Request) {
	tdata := app.newTemplateData(r)
	tdata.Form = eventDeleteForm{}

	events, err := app.Models.Events.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tdata.Events = events
	app.render(w, r, http.StatusOK, "eventDelete.tmpl", tdata)
}

func (app *Application) eventsDeletePost(w http.ResponseWriter, r *http.Request) {
	var form eventDeleteForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(form.Event != "", "event", "You must select an event")
	if !form.Valid() {
		// Render page again, with errors
		tdata := app.newTemplateData(r)
		tdata.Form = form

		events, err := app.Models.Events.GetAll()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		tdata.Events = events
		app.render(w, r, http.StatusUnprocessableEntity, "eventDelete.tmpl", tdata)
		return
	}

	// Delete event
	err = app.Models.Events.Delete(form.Event)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			// Render page again, with errors
			form.AddFieldError("event", "Event not found")

			tdata := app.newTemplateData(r)
			tdata.Form = form

			events, err := app.Models.Events.GetAll()
			if err != nil {
				app.serverError(w, r, err)
				return
			}

			tdata.Events = events
			app.render(w, r, http.StatusUnprocessableEntity, "eventDelete.tmpl", tdata)
			return
		}

		app.serverError(w, r, err)
		return
	}

	photoPath := path.Join(app.Config.StorageDir, "photos", form.Event)
	thumbPath := path.Join(app.Config.StorageDir, "thumbnails", form.Event)

	err = os.RemoveAll(thumbPath)
	if err != nil {
		app.serverError(w, r, err)
		//return
	}

	err = os.RemoveAll(photoPath)
	if err != nil {
		app.serverError(w, r, err)
		//return
	}

	app.SessionManager.Put(r.Context(), "flash", "Event deleted successfully")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
