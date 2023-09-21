package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"sitoWow/internal/data/models"
	"time"
)

type createEventCommand struct {
	name      string
	dayString string
	day       *time.Time
	fs        *flag.FlagSet
}

func (c *createEventCommand) Init(args []string) error {
	err := c.fs.Parse(args)
	if err != nil {
		return err
	}

	if c.name == "" {
		c.fs.Usage()
		fmt.Println()

		return errors.New("No event name provided")
	}

	if c.dayString != "" {
		var day time.Time

		day, err = time.Parse(time.DateOnly, c.dayString)
		if err != nil {
			return err
		}

		c.day = &day
	}

	return nil
}

func (c *createEventCommand) Run(db *sql.DB) error {
	fmt.Println(c.name)
	fmt.Println(c.day)

	m := models.New(db)

	event := &models.Event{
		Name: c.name,
		Date: c.day,
	}

	err := m.Events.Insert(event)
	if err != nil {
		return err
	}

	return nil
}

func (c *createEventCommand) Name() string {
	return "createEvent"
}

func newCreateEventCommand() *createEventCommand {
	c := &createEventCommand{
		fs: flag.NewFlagSet("createEvent", flag.ContinueOnError),
	}
	c.fs.StringVar(&c.name, "event", "", "Event name")
	c.fs.StringVar(&c.dayString, "day", "", "Event date YY-MM-DD")

	return c
}
