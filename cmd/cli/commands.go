package main

import (
	"database/sql"
	"fmt"
)

type Command interface {
	Init([]string) error
	Run(*sql.DB) error
	Name() string
}

// Returns command index, and the index of the argument that specifies the command
func findCommand(args []string, cmds []Command) (int, int, bool) {
	if len(args) < 1 {
		return 0, 0, false
	}

	for i, cmd := range cmds {
		for j, arg := range args {
			if cmd.Name() == arg {
				return i, j, true
			}
		}
	}

	fmt.Println("You must provide a valid command.\n\nValid commands:")
	for _, cmd := range cmds {
		fmt.Println("\t", cmd.Name())
	}
	fmt.Println()
	return 0, 0, false
}
