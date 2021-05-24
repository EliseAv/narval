package dispatcher

func factorioSetupMessage() []string {
	return []string{
		"All right, let's build an awesome factory!",
		"If you want an initial save game, send a your save zip file.",
		"If you want a new world, say `>config seed` or `>config seed SEEDNUMBER`",
		"If you want mods, zip your `%appdata%\\Factorio\\mods` folder and send it over.",
		"Some server json files are accepted too.",
		"When you are ready, say `>start`",
	}
}
