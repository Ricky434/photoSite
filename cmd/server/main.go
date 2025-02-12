package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sitoWow/internal/data/models"
	"sitoWow/web"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func main() {
	var cfg web.Config

	// Get configuration
	flag.IntVar(&cfg.Port, "port", 4000, "Server port")
	flag.StringVar(&cfg.Env, "env", "development", "Environment (development|production)") // non usata al momento
	flag.StringVar(&cfg.StaticDir, "static-dir", "./ui/static", "Path to static assets")
	flag.StringVar(&cfg.StorageDir, "storage-dir", "./storage", "Path to storage assets")

	flag.StringVar(&cfg.DB.Dsn, "db-dsn", "postgres://utentedb:password@localhost/sitoWow?sslmode=disable", "PostgreSQL DSN")
	flag.StringVar(&cfg.DB.MigrationsDir, "db-migrations", "./migrations", "DB migrations directory")
	flag.IntVar(&cfg.DB.MaxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.DB.MaxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.DB.MaxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max idle time")
	flag.Parse()

	// Setup logger
	handler := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(handler)

	// Open db connection
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("database connection pool established")

	// Migrate database
	migrations, err := migrate.New(fmt.Sprintf("file://%s", cfg.DB.MigrationsDir), cfg.DB.Dsn)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	err = migrations.Up()
	if err != nil && err != migrate.ErrNoChange {
		logger.Error(err.Error())
		os.Exit(1)
	}
	migrations.Close()

	// Initialize template cache
	templateCache, err := web.NewTemplateCache()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	formDecoder := form.NewDecoder()

	// Configure session manager
	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	// This means that cookies only work in https
	sessionManager.Cookie.Secure = true

	app := &web.Application{
		Config:         cfg,
		DB:             db,
		Logger:         logger,
		Models:         models.New(db),
		TemplateCache:  templateCache,
		FormDecoder:    formDecoder,
		SessionManager: sessionManager,
	}

	err = app.Serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func openDB(cfg web.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DB.Dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	db.SetMaxIdleConns(cfg.DB.MaxIdleConns)

	duration, err := time.ParseDuration(cfg.DB.MaxIdleTime)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
