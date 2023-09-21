package web

import (
	"net/http"
	"runtime/debug"
)

func (app *Application) logError(r *http.Request, err error) {
	app.Logger.Error(err.Error(),
		"request_method", r.Method,
		"request_url", r.URL.String(),
		"trace", string(debug.Stack()),
	)
}

func (app *Application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	//send 500 page
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// This is for when we don't need to log the error, since it is caused by the client
func (app *Application) clientError(w http.ResponseWriter, r *http.Request, status int, err error) {
	http.Error(w, http.StatusText(status), status)
}
