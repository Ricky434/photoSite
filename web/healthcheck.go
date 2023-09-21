package web

import "net/http"

func (app *Application) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}
