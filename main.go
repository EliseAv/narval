package main

import (
	"narval/dispatcher"
	"narval/launchers"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	launch := os.Getenv("LAUNCH")
	if launch != "" {
		launchers.Launch(launch)
		return
	}

	dispatcher.RunDispatcher()
}
