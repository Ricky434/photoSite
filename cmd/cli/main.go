package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type config struct {
	DB struct {
		Dsn          string
		MaxOpenConns int
		MaxIdleConns int
		MaxIdleTime  string
	}
}

func main() {
	var cfg config

	// Get configuration
	optionsFs := flag.NewFlagSet("Options", flag.ExitOnError)
	optionsFs.StringVar(&cfg.DB.Dsn, "db-dsn", "postgres://utentedb:password@localhost/sitoWow?sslmode=disable", "PostgreSQL DSN")
	// Non so se sono utili
	optionsFs.IntVar(&cfg.DB.MaxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	optionsFs.IntVar(&cfg.DB.MaxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	optionsFs.StringVar(&cfg.DB.MaxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max idle time")

	cmds := []Command{
		newCreateEventCommand(),
		newInsertPhotosCommand(),
		newCreateAdminCommand(),
		newEventFoldersIDCommand(),
	}

	// Find command, and its index in arguments list
	command, command_id, ok := findCommand(os.Args, cmds) // Possible bug: exe name same as one of the commands name
	if !ok {
		fmt.Printf("Usage: %s OPTIONS COMMAND [arg...]\n\n", os.Args[0])
		optionsFs.Usage()
		return
	}

	// Parse generic flags
	err := optionsFs.Parse(os.Args[1:command_id])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Init command (parse its flags)
	err = cmds[command].Init(os.Args[command_id+1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Open db connection
	db, err := openDB(cfg)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	fmt.Println("database connection pool established")

	// Run command
	err = cmds[command].Run(db)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func openDB(cfg config) (*sql.DB, error) {
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
