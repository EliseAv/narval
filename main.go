package main

import (
	"math/rand"
	"narval/dispatcher"
	"narval/launchers"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	rand.Seed(time.Now().UnixNano() ^ -0xbeef1e57b00b1e5)

	launch := os.Getenv("LAUNCH")
	if launch != "" {
		launchers.Launch(launch)
		return
	}

	dispatcher.RunDispatcher()
}
