package dispatcher

import (
	"log"
	"strconv"
)

type Snowflake uint64

func sf(id string) Snowflake {
	result, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		log.Panic(err)
	}
	return Snowflake(result)
}

func (flake Snowflake) String() string {
	return strconv.FormatUint(uint64(flake), 10)
}
