package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/justinas/nosurf"
)

func (app *Application) secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//w.Header().Set("Content-Security-Policy",
		//	"default-src 'self'; style-src 'self' fonts.googleapis.com; font-src fonts.gstatic.com; script-src unpkg.com") //maybe insecure unpkg

		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-XSS-Protection", "0")

		next.ServeHTTP(w, r)
	})
}

func (app *Application) staticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")

		next.ServeHTTP(w, r)
	})
}

func (app *Application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.Logger.Info("request",
			"remoteAddr", r.RemoteAddr,
			"realIP", r.Header.Get("X-Real-IP"),
			"protocol", r.Proto,
			"method", r.Method,
			"URI", r.URL.RequestURI(),
		)

		next.ServeHTTP(w, r)
	})
}

func (app *Application) recoverPanic(nexth http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverError(w, r, fmt.Errorf("%s", err))
			}
		}()

		nexth.ServeHTTP(w, r)
	})
}

func (app *Application) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.IsAuthenticated(r) {
			http.Redirect(w, r, "/user/login", http.StatusSeeOther)
			return
		}

		// so that pages that require authentication are not
		// stored in the users browser cache
		w.Header().Add("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

func (app *Application) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.IsAuthenticated(r) || !app.IsAdmin(r) {
			http.Redirect(w, r, "/user/login", http.StatusSeeOther)
			return
		}

		// so that pages that require authentication are not
		// stored in the users browser cache
		w.Header().Add("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

func (app *Application) noSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
	})

	// Manually check json requests
	csrfHandler.ExemptFunc(func(r *http.Request) bool {
		if r.Header.Get("Content-Type") != "application/json" {
			return false
		}

		var inputToken struct {
			Token string `json:"csrf_token"`
		}

		// Read body (max 1MB)
		maxBytes := 1_048_576 // 1MB
		r.Body = http.MaxBytesReader(nil, r.Body, int64(maxBytes))
		var buf []byte
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			return false
		}

		// json decoder wants a reader, so restore body
		r.Body = io.NopCloser(bytes.NewBuffer(buf))

		dec := json.NewDecoder(r.Body)
		err = dec.Decode(&inputToken)
		if err != nil {
			return false
		}

		// handler will also expect a reader, so restore body again
		r.Body = io.NopCloser(bytes.NewBuffer(buf))

		return nosurf.VerifyToken(nosurf.Token(r), inputToken.Token)
	})

	return csrfHandler
}

func (app *Application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := app.SessionManager.GetInt(r.Context(), "authenticatedUserID")
		if id == 0 {
			next.ServeHTTP(w, r)
			return
		}

		exists, level, err := app.Models.Users.Exists(id)
		if err != nil {
			app.serverError(w, r, err)
		}

		if exists {
			ctx := context.WithValue(r.Context(), isAuthenticatedContextKey, true)
			ctx = context.WithValue(ctx, userLevelContextKey, level)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}
