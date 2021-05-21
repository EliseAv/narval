package dispatcher

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID { // self messages
		return
	}

	if message.Content[:1] != ">" { // bye
		return
	}

	command := strings.Split(message.Content[1:], " ")
	event := messageEvent{session, message, command}
	event.choose()
}

func (event messageEvent) choose() {
	if event.command[0] == "opme" {
		event.commandOpme()
	}
}

func (event messageEvent) commandOpme() {
	if len(event.command) == 1 {
		author := event.message.Author
		whatever := make([]byte, 32)
		encoded := base64.StdEncoding.EncodeToString(whatever)
		store.user(author.ID).confirmation = encoded
		log.Printf("Tell %s#%s it's %s", author.Username, author.Discriminator, encoded)
		event.reply("I'll think about it.")
	} else {
		// decode and check
	}
}

func (event messageEvent) reply(message string) {
	event.session.ChannelMessageSendReply(event.message.ChannelID, message, event.message.MessageReference)
}
