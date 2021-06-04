package dispatcher

import (
	"bytes"
	"encoding/json"
	"strings"
)

func init() {
	allDispatchers["factorio"] = factorioDispatcher{}
}

type jsobj map[string]interface{}

type factorioDispatcher struct{}

func (factorioDispatcher) setup(event messageEvent) error {
	initFile, err := json.Marshal(jsobj{"launch": "factorio"})
	if err != nil {
		return err
	}
	err = event.putS3file("init.json", bytes.NewReader(initFile))
	if err != nil {
		return err
	}
	message := []string{
		"All right, let's build an awesome factory!",
		"If you want an initial save game, send your save zip file.",
		"If you want mods, zip your `%appdata%\\Factorio\\mods` folder and send it over.",
		"Some server json files are accepted too, including world settings with world seed.",
		"When you are ready, say `>start`",
	}
	event.reply(strings.Join(message, "\n"))
	return nil
}

func (factorioDispatcher) play(event messageEvent) error {
	//guild := store.guild(event.message.GuildID)
	return notImplemented{}
}
