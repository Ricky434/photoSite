package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"sitoWow/internal/data/models"
	"strconv"
)

type eventFoldersIDCommand struct {
	storageDir string
	fs         *flag.FlagSet
}

func (c *eventFoldersIDCommand) Init(args []string) error {
	err := c.fs.Parse(args)
	if err != nil {
		return err
	}

	if c.storageDir == "" {
		c.fs.Usage()
		fmt.Println()

		return errors.New("Not enough arguments provided")
	}

	return nil
}

func (c *eventFoldersIDCommand) Run(db *sql.DB) error {
	fmt.Println("flag:", c.storageDir)

	m := models.New(db)

	photosDir := path.Join(c.storageDir, "photos")
	if _, err := os.Stat(photosDir); errors.Is(err, os.ErrNotExist) {
		return err
	}

	thumbsDir := path.Join(c.storageDir, "thumbnails")
	if _, err := os.Stat(thumbsDir); errors.Is(err, os.ErrNotExist) {
		return err
	}

	events, err := m.Events.GetAll()
	if err != nil {
		return err
	}

	for _, e := range events {
		err := os.Rename(path.Join(photosDir, e.Name), path.Join(photosDir, strconv.Itoa(e.ID)))
		if err != nil {
			fmt.Printf("%s. Path: %s\n", err.Error(), path.Join(photosDir, e.Name))
		}

		err = os.Rename(path.Join(thumbsDir, e.Name), path.Join(thumbsDir, strconv.Itoa(e.ID)))
		if err != nil {
			fmt.Printf("%s. Path: %s\n", err.Error(), path.Join(thumbsDir, e.Name))
		}
	}

	return nil
}

func (c *eventFoldersIDCommand) Name() string {
	return "eventFoldersID"
}

func newEventFoldersIDCommand() *eventFoldersIDCommand {
	c := &eventFoldersIDCommand{
		fs: flag.NewFlagSet("eventFoldersID", flag.ContinueOnError),
	}
	c.fs.StringVar(&c.storageDir, "storage-dir", "./storage", "Photos storage directory")

	return c
}
