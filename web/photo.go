package web

import (
	"archive/zip"
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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

// Not used
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

	photos, metadata, err := app.Models.Photos.GetFiltered(&input.Event, input.Filters)
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
	Event               int `form:"event"`
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

func (app *Application) renderPhotosUploadErrors(w http.ResponseWriter, r *http.Request, form photoUploadForm) {
	// Render page again, with errors
	tdata := app.newTemplateData(r)
	tdata.Form = form

	events, err := app.Models.Events.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tdata.Events = events
	// Only render form body for htmx to substitute to current one
	app.renderRaw(w, r, http.StatusOK, "photoUploadHTMX.tmpl", tdata)
}

func (app *Application) photoUploadPost(w http.ResponseWriter, r *http.Request) {
	var form photoUploadForm

	err := r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.Event, err = strconv.Atoi(r.MultipartForm.Value["event"][0])
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(form.Event != 0, "event", "This field must not be zero")
	if !form.Valid() {
		app.renderPhotosUploadErrors(w, r, form)
		return
	}

	// Retrieve event
	event, err := app.Models.Events.GetByID(form.Event)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			// Render page again, with errors
			form.AddFieldError("event", "Event not found")
			app.renderPhotosUploadErrors(w, r, form)
			return
		}

		app.serverError(w, r, err)
		return
	}

	// Check if photos event directory is allowed (prevent path traversal)
	photosDir := path.Join(app.Config.StorageDir, "photos", strconv.Itoa(event.ID))
	if !app.InAllowedPath(photosDir, path.Join(app.Config.StorageDir, "photos")) {
		form.AddFieldError("event", "Event is not valid")
		app.renderPhotosUploadErrors(w, r, form)
		return
	}

	// Create photos and event directories if not present
	if _, err := os.Stat(photosDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(photosDir, os.ModePerm)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	// Check if photos event directory is allowed (prevent path traversal)
	thumbsDir := path.Join(app.Config.StorageDir, "thumbnails", strconv.Itoa(event.ID))
	if !app.InAllowedPath(thumbsDir, path.Join(app.Config.StorageDir, "thumbnails")) {
		form.AddFieldError("event", "Event is not valid")
		app.renderPhotosUploadErrors(w, r, form)
		return
	}

	// Create thumbnails directory if not present
	if _, err := os.Stat(thumbsDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(thumbsDir, os.ModePerm)
		if err != nil {
			app.serverError(w, r, err)
			return

		}
	}

	// TODO server error message should tell which files have been uploaded
	// TODO is file name at risk for path traversal?
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		form.AddFieldError("files", "You must select at least one file")
		app.renderPhotosUploadErrors(w, r, form)
		return
	}
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

		newFilePath := path.Join(photosDir, file.Filename)

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
			Latitude  *float32   `json:"GPSLatitude"`
			Longitude *float32   `json:"GPSLongitude"`
			TakenAt   *time.Time `json:"DateTimeOriginal"`
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
				"-path", thumbsDir,
				"-thumbnail", "500x500",
				newFilePath,
			)
		} else {
			magickCmd = exec.Command(
				"magick", "convert",
				"-resize", "500x500>",
				fmt.Sprintf("%s[1]", newFilePath),
				// Thumbnail for video is video filename(with extension)+".jpg"
				path.Join(thumbsDir, fmt.Sprintf("%s%s", path.Base(newFilePath), ".jpg")),
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
		app.renderPhotosUploadErrors(w, r, form)
		return
	}

	app.SessionManager.Put(r.Context(), "flash", "Files uploaded successfully")
	w.Header()["HX-Redirect"] = []string{fmt.Sprintf("/events/view/%d", event.ID)}
	//http.Redirect(w, r, fmt.Sprintf("/events/view/%d", event.ID), http.StatusOK)
}

func (app Application) photoDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token  string   `json:"csrf_token"` // only needed by readJSON since it checks for unknown keys
		Event  int      `json:"event"`
		Photos []string `json:"photos"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	missingFiles := []string{}

	for _, photo := range input.Photos {
		thumbFile := photo
		// If it is a video, "photo" will be the video file name + ".jpg", so we have to remove that
		if slices.Contains(models.VideoExtensions, strings.ToLower(path.Ext(strings.TrimSuffix(photo, ".jpg")))) {
			photo = strings.TrimSuffix(thumbFile, ".jpg")
		}

		photoPath := path.Join(app.Config.StorageDir, "photos", strconv.Itoa(input.Event), photo)
		// Prevent path traversal
		if !app.InAllowedPath(photoPath, path.Join(app.Config.StorageDir, "photos")) {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		thumbPath := path.Join(app.Config.StorageDir, "thumbnails", strconv.Itoa(input.Event), thumbFile)
		// Prevent path traversal
		if !app.InAllowedPath(thumbPath, path.Join(app.Config.StorageDir, "thumbnails")) {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		err := app.Models.Photos.DeleteByFile(photo)
		if err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				missingFiles = append(missingFiles, photo)
				continue
			}

			app.serverError(w, r, err)
			//return
		}

		err = os.Remove(thumbPath)
		if err != nil {
			app.serverError(w, r, err)
			//return
		}

		err = os.Remove(photoPath)
		if err != nil {
			app.serverError(w, r, err)
			//return
		}
	}

	if len(missingFiles) > 0 {
		app.SessionManager.Put(r.Context(), "flash",
			fmt.Sprintf("These files do not exist: \n\t%s.\nOther files have been deleted", strings.Join(missingFiles, "\n\t")))
	} else {
		app.SessionManager.Put(r.Context(), "flash", "Files deleted successfully")
	}
}

func (app Application) photoDownload(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token  string   `json:"csrf_token"` // only needed by readJSON since it checks for unknown keys
		Event  int      `json:"event"`
		Photos []string `json:"photos"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}
	// TODO: check len(photos) > 0

	// Check for files existance
	for _, photo := range input.Photos {
		_, err := app.Models.Photos.GetByFile(photo)
		if err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				app.clientError(w, http.StatusNotFound)
				return
			}

			app.serverError(w, r, err)
			return
		}
	}

	if len(input.Photos) == 1 {
		photoPath := path.Join(app.Config.StorageDir, "photos", strconv.Itoa(input.Event), input.Photos[0])
		// Prevent path traversal
		if !app.InAllowedPath(photoPath, path.Join(app.Config.StorageDir, "photos")) {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "image")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", input.Photos[0]))

		// There is probably a better way
		file, err := os.ReadFile(photoPath)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		_, err = w.Write(file)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		return
	}

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
	for _, photo := range input.Photos {
		photoPath := path.Join(app.Config.StorageDir, "photos", strconv.Itoa(input.Event), photo)
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

		zw, err := zipWriter.Create(photo)
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"foto.zip\""))

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
