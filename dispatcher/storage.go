package dispatcher

import (
	"bytes"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strconv"

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
}

type GuildStore struct{}

var store Store
var storeUrl url.URL

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
}

func initializeStore() {
	store.Users = map[Snowflake]*UserStore{}
	store.Channels = map[Snowflake]*ChannelStore{}
	store.Guilds = map[Snowflake]*GuildStore{}
}

func (store Store) store() {
	buffer, err := json.Marshal(store)
	if err != nil {
		log.Printf("Unable to marshal")
	}
	switch storeUrl.Scheme {
	case "file":
		reader := bytes.NewReader(buffer)
		atomic.WriteFile(storeUrl.Opaque, reader)
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

func sf(id string) Snowflake {
	result, _ := strconv.ParseUint(id, 10, 64)
	return Snowflake(result)
}

func (flake Snowflake) String() string {
	return strconv.FormatUint(uint64(flake), 10)
}
