package web

import (
	"net/http"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func (app *Application) Routes() http.Handler {
	router := httprouter.New()

	//router.NotFound = http.HandlerFunc(app.notFoundResponse)
	//router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	fileServer := http.FileServer(neuteredFileSystem{http.Dir(app.Config.StaticDir)})
	storageServer := http.FileServer(neuteredFileSystem{http.Dir(app.Config.StorageDir)})

	router.Handler(http.MethodGet, "/static/*filepath", http.StripPrefix("/static", fileServer))
	router.HandlerFunc(http.MethodGet, "/healthcheck", app.healthcheck)

	dynamic := alice.New(app.SessionManager.LoadAndSave, app.noSurf, app.authenticate)

	router.Handler(http.MethodGet, "/", dynamic.ThenFunc(app.homePage))
	router.Handler(http.MethodGet, "/user/login", dynamic.ThenFunc(app.userLoginPage))
	router.Handler(http.MethodPost, "/user/login", dynamic.ThenFunc(app.userLoginPost))
	//router.Handler(http.MethodGet, "/runs/:id", dynamic.ThenFunc(app.runView))

	protected := dynamic.Append(app.requireAuthentication)

	router.Handler(http.MethodGet, "/storage/*filepath", protected.Then(http.StripPrefix("/storage", storageServer)))

	router.Handler(http.MethodPost, "/user/logout", protected.ThenFunc(app.userLogout))
	//TODO: Questi possono essere usati solo se utente e' loggato e con livello abbastanza alto, modificare middleware di conseguenza
	router.Handler(http.MethodGet, "/user/create", dynamic.ThenFunc(app.userCreatePage))
	router.Handler(http.MethodPost, "/user/create", dynamic.ThenFunc(app.userCreatePost))

	standard := alice.New(app.recoverPanic, app.logRequest, app.secureHeaders)

	return standard.Then(router)
}

type neuteredFileSystem struct {
	fs http.FileSystem
}

func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		// http.FileSystem.Open thinks that paths with \ are unsafe on windows,
		// while filepath.Join uses \ because that's what windows uses
		index = filepath.ToSlash(index)
		if _, err := nfs.fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}
