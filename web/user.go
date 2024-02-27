package web

import (
	"errors"
	"net/http"
	"sitoWow/internal/data/models"
	"sitoWow/internal/validator"

	"github.com/google/uuid"
)

type userCreateForm struct {
	Name                string `form:"name"`
	Password            string `form:"password"`
	Level               int    `form:"level"`
	validator.Validator `form:"-"`
}

func (app *Application) userCreatePage(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = userCreateForm{}
	app.render(w, r, http.StatusOK, "userCreate.tmpl", data)
}

func (app *Application) userCreatePost(w http.ResponseWriter, r *http.Request) {
	var form userCreateForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user := &models.User{
		Name:  form.Name,
		Level: form.Level,
	}

	err = user.Password.Set(form.Password)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	models.ValidateUser(&form.Validator, user)

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, r, http.StatusUnprocessableEntity, "userCreate.tmpl", data)
		return
	}

	err = app.Models.Users.Insert(user)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateName) {
			form.AddFieldError("name", "Name is already in use")

			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, r, http.StatusUnprocessableEntity, "userCreate.tmpl", data)
		} else {
			app.serverError(w, r, err)
		}

		return
	}

	err = app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.SessionManager.Put(r.Context(), "flash", "User creation was successful.")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type userLoginForm struct {
	Name                string `form:"name"`
	Password            string `form:"password"`
	validator.Validator `form:"-"`
}

func (app *Application) userLoginPage(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Form = userLoginForm{}
	app.render(w, r, http.StatusOK, "login.tmpl", data)
}

func (app *Application) userLoginPost(w http.ResponseWriter, r *http.Request) {
	// This panics if the request id is not present in the context
	requestId := r.Context().Value(requestIdContextKey).(uuid.UUID)

	var form userLoginForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.NotBlank(form.Password), "password", "This field cannot be blank")

	if !form.Valid() {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, r, http.StatusUnprocessableEntity, "login.tmpl", data)
		return
	}

	id, err := app.Models.Users.Authenticate(form.Name, form.Password)
	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) {
			app.Logger.Warn("login failed",
				"requestId", requestId,
				"name", form.Name,
			)

			form.AddNonFieldError("Name or password is incorrect")

			data := app.newTemplateData(r)
			data.Form = form
			app.render(w, r, http.StatusUnprocessableEntity, "login.tmpl", data)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	err = app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.SessionManager.Put(r.Context(), "authenticatedUserID", id)

	app.Logger.Info("login successful",
		"requestId", requestId,
		"userId", id,
	)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Application) userLogout(w http.ResponseWriter, r *http.Request) {
	// This panics if the request id is not present in the context
	requestId := r.Context().Value(requestIdContextKey).(uuid.UUID)

	err := app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	id := app.SessionManager.Get(r.Context(), "authenticatedUserID")
	app.SessionManager.Remove(r.Context(), "authenticatedUserID")

	app.SessionManager.Put(r.Context(), "flash", "You've been logged out successfully!")

	app.Logger.Info("logout successful",
		"requestId", requestId,
		"userId", id,
	)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
