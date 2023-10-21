package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"sitoWow/internal/data/models"
)

type createAdminCommand struct {
	name     string
	password string
	fs       *flag.FlagSet
}

func (c *createAdminCommand) Init(args []string) error {
	err := c.fs.Parse(args)
	if err != nil {
		return err
	}

	if c.name == "" {
		c.fs.Usage()
		fmt.Println()

		return errors.New("No name provided")
	}

	if c.password == "" {
		c.fs.Usage()
		fmt.Println()

		return errors.New("No password provided")
	}

	return nil
}

func (c *createAdminCommand) Run(db *sql.DB) error {
	fmt.Println(c.name)
	fmt.Println(c.password)

	m := models.New(db)

	user := &models.User{
		Name:  c.name,
		Level: models.ADMIN_LEVEL,
	}

	user.Password.Set(c.password)

	err := m.Users.Insert(user)
	if err != nil {
		return err
	}

	return nil
}

func (c *createAdminCommand) Name() string {
	return "createAdmin"
}

func newCreateAdminCommand() *createAdminCommand {
	c := &createAdminCommand{
		fs: flag.NewFlagSet("createAdmin", flag.ContinueOnError),
	}
	c.fs.StringVar(&c.name, "name", "", "Name")
	c.fs.StringVar(&c.password, "day", "", "Password")

	return c
}
