package dispatcher

import (
	crypto_rand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

type messageEvent struct {
	session *discordgo.Session
	message *discordgo.MessageCreate
	command []string
}

func RunDispatcher() {
	loadSettings()

	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	discord.AddHandler(messageCreate)
	discord.Identify.Intents = discordgo.IntentsDirectMessages | discordgo.IntentsGuildMessages

	err = discord.Open()
	if err != nil {
		log.Panic(err)
	}
	defer discord.Close()

	// We done; just wait for exit
	fmt.Println("Bot is running. Ctrl-C to exit.")
	signalsChannel := make(chan os.Signal, 1)
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-signalsChannel
}

func init() {
	// Try to seed the RNG with a cryptographically good 64-bit number
	// https://stackoverflow.com/a/54491783/98029
	buffer := make([]byte, 8)
	_, err := crypto_rand.Read(buffer)
	var seed int64
	if err != nil {
		seed = time.Now().UnixNano() ^ -0xbeef1e57b00b1e5
	} else {
		seed = int64(binary.LittleEndian.Uint64(buffer))
	}
	rand.Seed(seed)
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID { // self messages
		return
	}

	if message.Content[:1] == ">" {
		// commands
		command := strings.Split(message.Content[1:], " ")
		event := messageEvent{session, message, command}
		switch event.command[0] {
		case "opme":
			event.commandOpme()
		case "setup":
			event.commandSetup()
		}
	}
}

func (event messageEvent) commandOpme() {
	user := store.user(event.message.Author.ID)
	if len(event.command) == 1 {
		if user.IsAdmin {
			event.reply("No need.")
		} else {
			author := event.message.Author
			whatever := make([]byte, 48)
			rand.Read(whatever)
			encoded := base64.StdEncoding.EncodeToString(whatever)
			user.confirmation = encoded
			log.Printf("Tell %s#%s >opme %s", author.Username, author.Discriminator, encoded)
			event.reply("I'll think about it.")
		}
	} else {
		password := event.command[1]
		if password == user.confirmation {
			user.IsAdmin = true
			store.store()
			event.reply("Okay.")
		} else {
			event.reply("Still thinking about it.")
		}
	}
}

func (event messageEvent) commandSetup() {
	user := store.user(event.message.Author.ID)
	if !user.IsAdmin {
		event.reply("Nope.")
		return
	}
	channel := store.channel(event.message.ChannelID)
	if channel.SetupComplete {
		event.reply("Already set up.")
		return
	}
	var game string
	if len(event.command) > 1 {
		game = event.command[1]
	}
	switch game {
	case "factorio":
		message := strings.Join(factorioSetupMessage(), "\n")
		message = strings.TrimSpace(message)
		message = strings.ReplaceAll(message, "\t", "")
	}
}

func (event messageEvent) reply(message string) {
	event.session.ChannelMessageSendReply(event.message.ChannelID, message, event.message.Reference())
}
