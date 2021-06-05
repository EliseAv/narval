package talkers

import "github.com/google/uuid"

type Session struct {
	Id uuid.UUID
}

var sessions = map[uuid.UUID]*Session{}

func NewSession() (*Session, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	new := &Session{uuid}
	sessions[uuid] = new
	return new, nil
}
