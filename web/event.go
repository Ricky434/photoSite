package web

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"sitoWow/internal/data"
	"sitoWow/internal/data/models"
	"sitoWow/internal/validator"
	"strings"
	"time"

	"github.com/google/uuid"
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
	// Try to prevent path traversal attacks
	form.CheckField(!strings.Contains(event.Name, ".."), "name", "This field must not contain the string '..'")

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

func (app *Application) renderEventDeleteErrors(w http.ResponseWriter, r *http.Request, form eventDeleteForm) {
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
		app.renderEventDeleteErrors(w, r, form)
		return
	}

	// Delete event
	err = app.Models.Events.Delete(form.Event)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			// Render page again, with errors
			form.AddFieldError("event", "Event not found")
			app.renderEventDeleteErrors(w, r, form)
			return
		}

		app.serverError(w, r, err)
		return
	}

	photoPath := path.Join(app.Config.StorageDir, "photos", form.Event)
	// Prevent path traversal
	if !app.InAllowedPath(photoPath, path.Join(app.Config.StorageDir, "photos")) {
		form.AddFieldError("event", "Event is not valid")
		app.renderEventDeleteErrors(w, r, form)
		return
	}

	thumbPath := path.Join(app.Config.StorageDir, "thumbnails", form.Event)
	// Prevent path traversal
	if !app.InAllowedPath(thumbPath, path.Join(app.Config.StorageDir, "thumbnails")) {
		form.AddFieldError("event", "Event is not valid")
		app.renderEventDeleteErrors(w, r, form)
		return
	}

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

func (app *Application) eventDownload(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())

	name := params.ByName("name")

	event, err := app.Models.Events.GetByName(name)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			app.clientError(w, http.StatusNotFound)
			return
		}

		app.serverError(w, r, err)
		return
	}

	filters := data.Filters{
		Page:     1,
		PageSize: math.MaxInt32,
		SortSafelist: []string{"taken_at"},
		Sort: "taken_at",
	}
	photos, _, err := app.Models.Photos.GetAll(&event.ID, filters)

	// ---- Zip files
	tmpDir := path.Join(app.Config.StorageDir, "tmp")
	tmpPath := path.Join(tmpDir, uuid.NewString())

	// Create temp directory if not present
	if _, err := os.Stat(tmpDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(tmpDir, os.ModePerm)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	// Create temp zip file
	tmpZip, err := os.Create(tmpPath)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	defer func() {
		// TODO: Error handling?
		tmpZip.Close()
		os.Remove(tmpPath)
	}()

	zipWriter := zip.NewWriter(tmpZip)

	// Add photos to zip
	for _, photo := range photos {
		photoPath := path.Join(app.Config.StorageDir, "photos", name, photo.FileName)
		// Prevent path traversal
		if !app.InAllowedPath(photoPath, path.Join(app.Config.StorageDir, "photos")) {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		f, err := os.Open(photoPath)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		defer f.Close()

		zw, err := zipWriter.Create(photo.FileName)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		_, err = io.Copy(zw, f)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	err = zipWriter.Close()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", name))

	// There is probably a better way
	file, err := os.ReadFile(tmpPath)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	_, err = w.Write(file)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
}
