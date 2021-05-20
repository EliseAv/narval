package dispatcher

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func RunDispatcher() {
	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
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

	// Wait for interrupt
	fmt.Println("Bot is running. Ctrl-C to exit.")
	signalsChannel := make(chan os.Signal, 1)
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-signalsChannel
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == session.State.User.ID { // self messages
		return
	}
	if message.Content != "ping" { // bye
		return
	}

	channel, err := session.UserChannelCreate(message.Author.ID)
	if err != nil {
		fmt.Println(err)
		session.ChannelMessageSend(message.ChannelID, "Something went wrong!")
		return
	}

	_, err = session.ChannelMessageSend(channel.ID, "Pong!")
	if err != nil {
		fmt.Println(err)
		session.ChannelMessageSend(message.ChannelID, "DM Failed.")
	}
}
