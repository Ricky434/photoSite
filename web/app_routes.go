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

	// PUBLIC
	dynamic := alice.New(app.SessionManager.LoadAndSave, app.noSurf, app.authenticate)

	router.Handler(http.MethodGet, "/user/login", dynamic.ThenFunc(app.userLoginPage))
	router.Handler(http.MethodPost, "/user/login", dynamic.ThenFunc(app.userLoginPost))

	// LOGIN REQUIRED
	protected := dynamic.Append(app.requireAuthentication)

	// Added headers that allow files to be cached only by local browser, after protected in order to overwrite the header authenticate sets
	router.Handler(http.MethodGet, "/storage/*filepath", protected.Then(app.staticCacheHeaders(http.StripPrefix("/storage", storageServer))))

	router.Handler(http.MethodGet, "/", protected.ThenFunc(app.homePage))
	router.Handler(http.MethodGet, "/events/view/:id", protected.ThenFunc(app.eventPage))
	router.Handler(http.MethodGet, "/photos/view/:file", protected.ThenFunc(app.photoPage))
	router.Handler(http.MethodPost, "/user/logout", protected.ThenFunc(app.userLogout))
	router.Handler(http.MethodPost, "/photos/download", protected.ThenFunc(app.photoDownload))
	router.Handler(http.MethodGet, "/events/download/:id", protected.ThenFunc(app.eventDownload))

	// ADMIN
	admin := protected.Append(app.requireAdmin)
	router.Handler(http.MethodGet, "/user/create", admin.ThenFunc(app.userCreatePage))
	router.Handler(http.MethodPost, "/user/create", admin.ThenFunc(app.userCreatePost))
	router.Handler(http.MethodGet, "/photos/upload", admin.ThenFunc(app.photoUploadPage))
	router.Handler(http.MethodPost, "/photos/upload", admin.ThenFunc(app.photoUploadPost))
	router.Handler(http.MethodPost, "/photos/delete", admin.ThenFunc(app.photoDelete))
	router.Handler(http.MethodGet, "/events/create", admin.ThenFunc(app.eventsCreatePage))
	router.Handler(http.MethodPost, "/events/create", admin.ThenFunc(app.eventsCreatePost))
	router.Handler(http.MethodGet, "/events/update/:id", admin.ThenFunc(app.eventsUpdatePage))
	router.Handler(http.MethodPost, "/events/update/:id", admin.ThenFunc(app.eventsUpdatePost))
	router.Handler(http.MethodGet, "/events/delete", admin.ThenFunc(app.eventsDeletePage))
	router.Handler(http.MethodPost, "/events/delete", admin.ThenFunc(app.eventsDeletePost))

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
