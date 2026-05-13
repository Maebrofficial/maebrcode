package main

import (
	"github.com/mohammadtihame/maebrcode/cmd"
	"github.com/mohammadtihame/maebrcode/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	cmd.Execute()
}
