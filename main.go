package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"narval/gameLaunchers"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type jsobj map[string](interface{})

const getIpUrl = "http://169.254.169.254/latest/meta-data/public-ipv4"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Panic(err)
	}
	factorioServer := gameLaunchers.FactorioServer{}
	factorioServer.Game.Version = "1.1.33"
	factorioServer.Game.Save = "aoeu.zip"
	factorioServer.Settings.MaxSession = 24 * time.Hour
	factorioServer.Settings.StartupGrace = 5 * time.Minute
	factorioServer.Settings.ShutdownGrace = 1 * time.Minute
	var server gameLaunchers.Server = &factorioServer
	// server.Prepare()
	server.Start()
	for line := range server.GetLinesChannel() {
		message := toMessage(server, line)
		if message != "" {
			sayInDiscord(message)
		}
	}
	sayInDiscord("Server shut down.")
}

func toMessage(server gameLaunchers.Server, line gameLaunchers.ParsedLine) string {
	switch line.Event {
	case gameLaunchers.EventReady:
		return "Server is ready!"
	case gameLaunchers.EventJoin:
		return fmt.Sprintf("`[%2d]` :star2: %s", server.NumPlayers(), line.Author)
	case gameLaunchers.EventLeave:
		return fmt.Sprintf("`[%2d]` :comet: %s", server.NumPlayers(), line.Author)
	case gameLaunchers.EventTalk:
		return fmt.Sprintf("`<%s>` %s", line.Author, line.Message)
	}
	return ""
}

func sayInDiscord(message string) {
	body := jsobj{
		"content":          message,
		"allowed_mentions": jsobj{"parse": []string{}},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		log.Panic(err)
	}
	reader := bytes.NewReader(payload)
	hookUrl := os.Getenv("WEBHOOK_URL")
	_, err = http.Post(hookUrl, "application/json", reader)
	if err != nil {
		log.Print(err)
	}
}
