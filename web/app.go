package web

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sitoWow/internal/data/models"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
)

type Config struct {
	Port       int
	Env        string
	StaticDir  string
	StorageDir string
	DB         struct {
		Dsn           string
		MigrationsDir string
		MaxOpenConns  int
		MaxIdleConns  int
		MaxIdleTime   string
	}
}

type Application struct {
	Config         Config
	DB             *sql.DB
	Logger         *slog.Logger
	Models         models.Models
	TemplateCache  map[string]*template.Template
	FormDecoder    *form.Decoder
	SessionManager *scs.SessionManager
}

func (app *Application) Serve() error {
	tlsConfig := &tls.Config{
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.Config.Port),
		ErrorLog:     slog.NewLogLogger(app.Logger.Handler(), slog.LevelError),
		Handler:      app.Routes(),
		TLSConfig:    tlsConfig,
		IdleTimeout:  time.Minute,
		ReadTimeout:  0,
		WriteTimeout: 0,
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		app.Logger.Info("shutting down server",
			"signal", s.String(),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		shutdownError <- srv.Shutdown(ctx)
	}()

	app.Logger.Info("starting server",
		"addr", srv.Addr,
		"env", app.Config.Env,
	)

	err := srv.ListenAndServeTLS("./tls/cert.pem", "./tls/key.pem")
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	app.Logger.Info("stopped server",
		"addr", srv.Addr,
	)

	return nil
}
