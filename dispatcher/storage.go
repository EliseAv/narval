package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/natefinch/atomic"
)

type Store struct {
	Users    map[Snowflake]*UserStore
	Channels map[Snowflake]*ChannelStore
	Guilds   map[Snowflake]*GuildStore
}

type UserStore struct {
	id           Snowflake
	IsAdmin      bool
	confirmation string
}

type ChannelStore struct {
	id            Snowflake
	SetupComplete bool
	Game          string
	Prefix        string
	dispatcher    dispatcher
	session       string
}

type GuildStore struct {
	id     Snowflake
	Bucket string
	Region string
}

var allDispatchers = map[string]dispatcher{}

var store Store
var storeUrl url.URL
var storeThrottle = make(chan struct{}, 200)

func loadSettings() error {
	envUrl := os.Getenv("STORAGE_URL")
	storeUrlPointer, err := url.Parse(envUrl)
	if err != nil {
		return err
	}
	storeUrl = *storeUrlPointer
	log.Printf("Store URL is %v", storeUrl)
	var buffer []byte
	switch storeUrl.Scheme {
	case "file":
		buffer, err = os.ReadFile(storeUrl.Opaque)
		if errors.Is(err, os.ErrNotExist) {
			initializeStore()
			return nil
		}
		if err != nil {
			return err
		}
	default:
		log.Panicf("Scheme not implemented: %v", storeUrl)
	}
	err = json.Unmarshal(buffer, &store)
	if err != nil {
		return err
	}
	go keepStoring()
	return nil
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
			_ = atomic.WriteFile(storeUrl.Opaque, reader)
		}
	}
}

func (store Store) user(id string) *UserStore {
	flake := sf(id)
	user, found := store.Users[flake]
	if !found {
		user = &UserStore{id: flake}
		store.Users[flake] = user
	}
	return user
}

func (store Store) channel(id string) *ChannelStore {
	flake := sf(id)
	channel, found := store.Channels[flake]
	if !found {
		channel = &ChannelStore{id: flake, Prefix: id + "/"}
		store.Channels[flake] = channel
	}
	if channel.dispatcher == nil {
		channel.dispatcher = allDispatchers[channel.Game]
	}
	return channel
}

func (store Store) guild(id string) *GuildStore {
	flake := sf(id)
	guild, found := store.Guilds[flake]
	if !found {
		guild = &GuildStore{id: flake}
		store.Guilds[flake] = guild
	}
	return guild
}
