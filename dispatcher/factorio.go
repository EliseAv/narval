package dispatcher

func init() {
	stringToDispatcher["factorio"] = factorioDispatcher{}
}

type factorioDispatcher struct{}

func (factorioDispatcher) setupMessage() []string {
	return []string{
		"All right, let's build an awesome factory!",
		"If you want an initial save game, send your save zip file.",
		"If you want mods, zip your `%appdata%\\Factorio\\mods` folder and send it over.",
		"Some server json files are accepted too, including world settings with world seed.",
		"When you are ready, say `>start`",
	}
}

func (factorioDispatcher) configFromEvent(event messageEvent) {}
