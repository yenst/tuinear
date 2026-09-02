package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "Tuinear"
	keyringUser    = "oauth-session"
)

var ErrTokenNotFound = errors.New("OAuth token not found")

type KeyringStore struct {
	service string
	user    string
}

func NewKeyringStore(clientID string) *KeyringStore {
	user := keyringUser
	if trimmed := strings.TrimSpace(clientID); trimmed != "" {
		user += ":" + trimmed
	}
	return &KeyringStore{service: keyringService, user: user}
}

func (s *KeyringStore) Load() (Token, error) {
	encoded, err := keyring.Get(s.service, s.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return Token{}, ErrTokenNotFound
	}
	if err != nil {
		return Token{}, fmt.Errorf("read operating-system credential store: %w", err)
	}
	var token Token
	if err := json.Unmarshal([]byte(encoded), &token); err != nil {
		return Token{}, fmt.Errorf("decode stored OAuth token: %w", err)
	}
	return token, nil
}

func (s *KeyringStore) Save(token Token) error {
	encoded, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("encode OAuth token: %w", err)
	}
	if err := keyring.Set(s.service, s.user, string(encoded)); err != nil {
		return fmt.Errorf("write operating-system credential store: %w", err)
	}
	return nil
}

func (s *KeyringStore) Delete() error {
	err := keyring.Delete(s.service, s.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrTokenNotFound
	}
	if err != nil {
		return fmt.Errorf("delete operating-system credential: %w", err)
	}
	return nil
}
