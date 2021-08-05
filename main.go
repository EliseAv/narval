package main

import (
	"github.com/joho/godotenv"
	"log"
	"narval/dispatcher"
	"narval/launchers"
	"os"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Print(err)
	}

	launch := os.Getenv("LAUNCH")
	if launch != "" {
		launchers.Launch(launch)
		return
	}

	dispatcher.RunDispatcher()
}
