package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sitoWow/internal/data"
	"sitoWow/internal/data/models"
	"sitoWow/internal/validator"
	"slices"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

func (app *Application) photoList(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Event int
		data.Filters
	}

	v := &validator.Validator{}

	qs := r.URL.Query()

	input.Event = app.readInt(qs, "event", 1, v)
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 10, v)
	input.Filters.Sort = app.readString(qs, "sort", "taken_at")

	input.Filters.SortSafelist = []string{"id", "-id", "taken_at", "-taken_at", "latitude", "-latitude", "longitude", "-longitude"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		data := app.newTemplateData(r)
		// trovare un modo per mostrare errori
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
		if errors.Is(err, models.ErrRecordNotFound) {
			app.clientErrorHTMX(w, http.StatusNotFound)
			return
		}

		app.serverErrorHTMX(w, r, err)
		return
	}

	photos, metadata, err := app.Models.Photos.GetAll(&input.Event, input.Filters)
	if err != nil {
		app.serverErrorHTMX(w, r, err)
		return
	}

	// Set thubnail names, replace video extensions with jpg extension (for thumbnail path)
	for i := range photos {
		if slices.Contains(models.VideoExtensions, strings.ToLower(path.Ext(photos[i].FileName))) {
			// Thumbnail for video is video filename(with extension)+".jpg"
			photos[i].ThumbName = fmt.Sprintf("%s%s", path.Base(photos[i].FileName), ".jpg")
		} else {
			photos[i].ThumbName = photos[i].FileName
		}
	}

	tdata := app.newTemplateData(r)
	tdata.Photos = photos
	tdata.Event = event
	tdata.Metadata = &metadata

	app.renderRaw(w, r, http.StatusOK, "photoList.tmpl", tdata)
}

func (app *Application) photoPage(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())

	file := params.ByName("file")

	photo, err := app.Models.Photos.GetByFile(file)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			app.clientError(w, http.StatusNotFound)
			return
		}

		app.serverError(w, r, err)
	}

	event, err := app.Models.Events.GetByID(photo.Event)
	if err != nil {
		// This should never happen thanks to db foreign key
		if errors.Is(err, models.ErrRecordNotFound) {
			app.clientError(w, http.StatusNotFound)
			return
		}

		app.serverError(w, r, err)
	}

	tdata := app.newTemplateData(r)
	tdata.Photo = photo
	tdata.Event = event

	app.render(w, r, http.StatusOK, "photo.tmpl", tdata)
}

type photoUploadForm struct {
	Event               string `form:"event"`
	validator.Validator `form:"-"`
}

func (app *Application) photoUploadPage(w http.ResponseWriter, r *http.Request) {
	tdata := app.newTemplateData(r)
	tdata.Form = photoUploadForm{}

	events, err := app.Models.Events.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tdata.Events = events
	app.render(w, r, http.StatusOK, "photoUpload.tmpl", tdata)
}

func (app *Application) photoUploadPost(w http.ResponseWriter, r *http.Request) {
	var form photoUploadForm

	err := r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.Event = r.MultipartForm.Value["event"][0]

	form.CheckField(form.Event != "", "event", "This field must not be empty")
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
		app.render(w, r, http.StatusUnprocessableEntity, "photoUpload.tmpl", tdata)
		return
	}

	// Retrieve event id
	event, err := app.Models.Events.GetByName(form.Event)
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
			app.render(w, r, http.StatusUnprocessableEntity, "photoUpload.tmpl", tdata)
			return
		}

		app.serverError(w, r, err)
		return
	}

	// Create photos and event directories if not present
	photosDir := path.Join(app.Config.StorageDir, "photos", event.Name)
	if _, err := os.Stat(photosDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(photosDir, os.ModePerm)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	// Create thumbnails directory if not present
	thumbsDir := path.Join(app.Config.StorageDir, "thumbnails", event.Name)
	if _, err := os.Stat(thumbsDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(thumbsDir, os.ModePerm)
		if err != nil {
			app.serverError(w, r, err)
			return

		}
	}

	// TODO server error message should tell which files have been uploaded
	files := r.MultipartForm.File["files"]
	for _, file := range files {
		var isVideo bool

		if slices.Contains(models.VideoExtensions, strings.ToLower(path.Ext(file.Filename))) {
			isVideo = true
		}

		if !slices.Contains(models.ImageExtensions, strings.ToLower(path.Ext(file.Filename))) && !isVideo {
			// This is a non fatal error, add it to the errors and keep going
			form.AddNonFieldError(fmt.Sprintf("This file is neither a supported image nor video: %s", file.Filename))
			continue
		}

		// Save file
		f, err := file.Open()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		newFilePath := path.Join(app.Config.StorageDir, "photos", event.Name, file.Filename)

		destination, err := os.Create(newFilePath)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		defer destination.Close()

		_, err = io.Copy(destination, f)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		destination.Close() // Close file since we need to access it

		// Extract metadata from photo
		var ExiftoolOut []struct {
			Latitude        *float32   `json:"GPSLatitude"`
			Longitude       *float32   `json:"GPSLongitude"`
			TakenAt         *time.Time `json:"DateTimeOriginal"`
			TakenAtFallback *time.Time `json:"TrackCreateDate"`
		}

		cmd := exec.Command(
			"exiftool",
			"-TAG", "-GPSLatitude#", "-GPSLongitude#", "-DateTimeOriginal", "-TrackCreateDate",
			"-j",
			"-d", "%Y-%m-%dT%H:%M:%SZ",
			newFilePath,
		)

		stdout, err := cmd.Output()
		if err != nil {
			os.Remove(newFilePath)
			app.serverError(w, r, err)
			return
		}

		err = json.Unmarshal(stdout, &ExiftoolOut)
		if err != nil {
			os.Remove(newFilePath)
			app.serverError(w, r, err)
			return
		}

		// Insert file data in db
		photo := &models.Photo{
			FileName:  path.Base(newFilePath),
			TakenAt:   ExiftoolOut[0].TakenAt,
			Latitude:  ExiftoolOut[0].Latitude,
			Longitude: ExiftoolOut[0].Longitude,
			Event:     event.ID,
		}

		if photo.TakenAt == nil && ExiftoolOut[0].TakenAtFallback != nil {
			photo.TakenAt = ExiftoolOut[0].TakenAtFallback
		}

		err = app.Models.Photos.Insert(photo)
		if err != nil {
			os.Remove(newFilePath)
			// If there is a non fatal error, add it to the errors and keep going
			if errors.Is(err, models.ErrDuplicateName) {
				form.AddNonFieldError(fmt.Sprintf("This file was already uploaded: %s", file.Filename))
			} else if errors.Is(err, models.ErrInvalidLatLon) {
				form.AddNonFieldError(fmt.Sprintf("This file has invalid latitude or longitude: %s", file.Filename))
			} else {
				app.serverError(w, r, err)
				return
			}

			continue
		}

		// Make thumbnail
		var magickCmd *exec.Cmd
		if !isVideo {
			magickCmd = exec.Command(
				"mogrify",
				"-auto-orient",
				"-path", path.Join(app.Config.StorageDir, "thumbnails", event.Name),
				"-thumbnail", "500x500",
				newFilePath,
			)
		} else {
			magickCmd = exec.Command(
				"magick", "convert",
				"-resize", "500x500>",
				fmt.Sprintf("%s[1]", newFilePath),
				// Thumbnail for video is video filename(with extension)+".jpg"
				path.Join(app.Config.StorageDir, "thumbnails", event.Name, fmt.Sprintf("%s%s", path.Base(newFilePath), ".jpg")),
			)
		}

		_, err = magickCmd.Output()
		if err != nil {
			// Rollback
			app.Models.Photos.Delete(photo.ID)
			os.Remove(destination.Name())
			app.serverError(w, r, err)
			return
		}
	}

	// If non fatal errors happened, inform client
	if !form.Valid() {
		form.AddNonFieldError("All other files have been uploaded")
		tdata := app.newTemplateData(r)
		tdata.Form = form

		events, err := app.Models.Events.GetAll()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		tdata.Events = events
		app.render(w, r, http.StatusUnprocessableEntity, "photoUpload.tmpl", tdata)
		return
	}

	app.SessionManager.Put(r.Context(), "flash", "Files uploaded successfully")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app Application) photoDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Event  string   `json:"event"`
		Photos []string `json:"photos"`
	}

	err := app.readJSON(w, r, input)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		// TODO: maybe should return html
		return
	}

	for _, photo := range input.Photos {
		photoPath := path.Join(app.Config.StorageDir, "photos", input.Event, photo)
		thumbPath := path.Join(app.Config.StorageDir, "thumbnails", input.Event, photo)

		err := app.Models.Photos.DeleteByFile(photo)
		if err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				app.clientError(w, http.StatusNotFound)
				// TODO: maybe should return html, say which have succeded
				return
			}

			app.serverError(w, r, err)
			return
		}

		err = os.Remove(thumbPath)
		if err != nil {
			app.serverError(w, r, err)
			//same
			return
		}

		err = os.Remove(photoPath)
		if err != nil {
			app.serverError(w, r, err)
			//same
			return
		}
	}

	// TODO: return ok
}