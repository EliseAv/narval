package dispatcher

import (
	"bytes"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/natefinch/atomic"
)

type Snowflake uint64

type Store struct {
	Users    map[Snowflake]*UserStore
	Channels map[Snowflake]*ChannelStore
	Guilds   map[Snowflake]*GuildStore
}

type UserStore struct {
	IsAdmin      bool
	confirmation string
}

type ChannelStore struct {
	SetupComplete bool
	Game          string
	Flags         map[string]bool
	Numbers       map[string]float64
	Settings      map[string]string
	dispatcher    dispatcher
}

type GuildStore struct {
	AssumeRole string
}

type dispatcher interface {
	setupMessage() []string
	configFromEvent(messageEvent)
}

var stringToDispatcher = map[string]dispatcher{}

var store Store
var storeUrl url.URL
var storeThrottle = make(chan struct{})

func loadSettings() {
	envUrl := os.Getenv("STORAGE_URL")
	storeUrlPointer, err := url.Parse(envUrl)
	if err != nil {
		log.Printf("Unable to parse store URL (%s): %v", envUrl, err)
		return
	}
	storeUrl = *storeUrlPointer
	log.Printf("Store URL is %v", storeUrl)
	var buffer []byte
	switch storeUrl.Scheme {
	case "file":
		buffer, err = os.ReadFile(storeUrl.Opaque)
		if os.IsNotExist(err) {
			initializeStore()
			return
		}
		if err != nil {
			log.Panic(err)
		}
	default:
		log.Panicf("Scheme not implemented: %v", storeUrl)
	}
	json.Unmarshal(buffer, &store)
	go keepStoring()
}

func initializeStore() {
	store.Users = map[Snowflake]*UserStore{}
	store.Channels = map[Snowflake]*ChannelStore{}
	store.Guilds = map[Snowflake]*GuildStore{}
}

func (store Store) store() {
	storeThrottle <- struct{}{}
}

func keepStoring() {
	for {
		time.Sleep(1 * time.Second)
		<-storeThrottle

		buffer, err := json.Marshal(store)
		if err != nil {
			log.Printf("Unable to marshal (%s): %v", err, store)
			continue
		}
		switch storeUrl.Scheme {
		case "file":
			reader := bytes.NewReader(buffer)
			atomic.WriteFile(storeUrl.Opaque, reader)
		}
	}
}

func (store Store) user(id string) *UserStore {
	flake := sf(id)
	user, found := store.Users[flake]
	if !found {
		user = &UserStore{}
		store.Users[flake] = user
	}
	return user
}

func (store Store) channel(id string) *ChannelStore {
	flake := sf(id)
	channel, found := store.Channels[flake]
	if !found {
		channel = &ChannelStore{}
		store.Channels[flake] = channel
	}
	if channel.dispatcher == nil {
		channel.dispatcher = stringToDispatcher[channel.Game]
	}
	return channel
}

func (store Store) guild(id string) *GuildStore {
	flake := sf(id)
	guild, found := store.Guilds[flake]
	if !found {
		guild = &GuildStore{}
		store.Guilds[flake] = guild
	}
	return guild
}

func sf(id string) Snowflake {
	result, _ := strconv.ParseUint(id, 10, 64)
	return Snowflake(result)
}

func (flake Snowflake) String() string {
	return strconv.FormatUint(uint64(flake), 10)
}
