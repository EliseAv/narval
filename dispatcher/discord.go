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

type dispatcher interface {
	setup(messageEvent)
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

	if len(message.Attachments) > 0 {
		for _, v := range message.Attachments {
			log.Printf("%v", v)
		}
	} else if len(message.Content) > 1 && message.Content[:1] == ">" {
		command := strings.Split(message.Content[1:], " ")
		messageEvent{session, message, command}.commands()
	}
}

func (event messageEvent) commands() {
	// commands
	switch event.command[0] {
	case "opme":
		event.commandOpme()
	case "aws":
		event.commandAws()
	case "setup":
		event.commandSetup()
	}
}

func (event messageEvent) commandOpme() {
	user := store.user(event.message.Author.ID)
	if len(event.command) == 1 {
		if user.IsAdmin {
			event.reply(":white_check_mark:")
		} else {
			author := event.message.Author
			whatever := make([]byte, 48)
			rand.Read(whatever)
			encoded := base64.StdEncoding.EncodeToString(whatever)
			user.confirmation = encoded
			log.Printf("Tell %s#%s >opme %s", author.Username, author.Discriminator, encoded)
			event.react(":thinking:")
		}
	} else {
		password := event.command[1]
		if password == user.confirmation {
			user.IsAdmin = true
			store.store()
			event.react(":white_check_mark:")
		} else {
			event.react(":unamused:")
		}
	}
}

func (event messageEvent) commandAws() {
	user := store.user(event.message.Author.ID)
	if !user.IsAdmin {
		event.react(":unamused:")
		return
	}
	if len(event.command) != 3 {
		event.reply("Expected: `>aws region-name bucket-name`")
	}
	guild := store.guild(event.message.GuildID)
	guild.Region = event.command[1]
	guild.Bucket = event.command[2]
	store.store()
	event.react(":white_check_mark:")
}

func (event messageEvent) commandSetup() {
	user := store.user(event.message.Author.ID)
	if !user.IsAdmin {
		event.react(":unamused:")
		return
	}
	channel := store.channel(event.message.ChannelID)
	if channel.SetupComplete {
		event.reply(fmt.Sprintf("Already set up %s.", channel.Game))
		return
	}
	if len(event.command) > 1 {
		channel.Game = event.command[1]
		channel.dispatcher = allDispatchers[channel.Game]
	}
	if channel.dispatcher == nil {
		event.reply("Try `>setup factorio`")
	} else {
		channel.dispatcher.setup(event)
	}
}

func (event messageEvent) reply(message string) error {
	_, err := event.session.ChannelMessageSendReply(event.message.ChannelID, message, event.message.Reference())
	return err
}

func (event messageEvent) react(emoji string) error {
	return event.session.MessageReactionAdd(event.message.ChannelID, event.message.ID, emoji)
}
