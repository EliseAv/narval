package dispatcher

import (
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"strconv"

	"github.com/shamaton/msgpack"
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
var storeUrl *url.URL

func loadSettings() {
	envUrl := os.Getenv("STORAGE_URL")
	storeUrl, err := url.Parse(envUrl)
	if err != nil {
		log.Printf("Unable to parse store URL (%s): %v", envUrl, err)
		return
	}
	var buffer []byte
	if storeUrl.Scheme == "file" {
		buffer, err = ioutil.ReadFile(storeUrl.Opaque)
		if os.IsNotExist(err) {
			initializeStore()
			return
		}
		if err != nil {
			log.Panic(err)
		}
	} else {
		log.Panicf("Scheme not implemented: %v", storeUrl)
	}
	msgpack.Unmarshal(buffer, &store)
}

func initializeStore() {
	store.Users = map[Snowflake]*UserStore{}
	store.Channels = map[Snowflake]*ChannelStore{}
	store.Guilds = map[Snowflake]*GuildStore{}
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
