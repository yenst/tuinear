package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "Tuinear"

var (
	ErrTokenNotFound   = errors.New("OAuth token not found")
	ErrProfileNotFound = errors.New("account profile not found")
)

type Profile struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	UserEmail     string `json:"user_email"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceKey  string `json:"workspace_key"`
}

func (p Profile) Label() string {
	workspace := p.WorkspaceName
	if workspace == "" {
		workspace = p.WorkspaceKey
	}
	user := p.UserName
	if user == "" {
		user = p.UserEmail
	}
	if workspace == "" {
		return user
	}
	if user == "" {
		return workspace
	}
	return workspace + " — " + user
}

type Session struct {
	Profile Profile `json:"profile"`
	Token   Token   `json:"token"`
}

type Store interface {
	List() ([]Profile, error)
	Active() (string, error)
	Load(profileID string) (Session, error)
	Save(Session, bool) error
	SetActive(profileID string) error
	Delete(profileID string) error
}

type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (systemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (systemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type profileRegistry struct {
	Version  int       `json:"version"`
	Active   string    `json:"active"`
	Profiles []Profile `json:"profiles"`
}

type KeyringStore struct {
	service   string
	namespace string
	backend   keyringBackend
}

func NewKeyringStore(clientID string) *KeyringStore {
	return &KeyringStore{
		service:   keyringService,
		namespace: shortHash(clientID),
		backend:   systemKeyring{},
	}
}

func (s *KeyringStore) List() ([]Profile, error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	return append([]Profile(nil), registry.Profiles...), nil
}

func (s *KeyringStore) Active() (string, error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return "", err
	}
	if registry.Active == "" {
		return "", ErrProfileNotFound
	}
	return registry.Active, nil
}

func (s *KeyringStore) Load(profileID string) (Session, error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return Session{}, err
	}
	if profileID == "" {
		profileID = registry.Active
	}
	profile, ok := findProfileByID(registry.Profiles, profileID)
	if !ok {
		return Session{}, ErrProfileNotFound
	}
	encoded, err := s.backend.Get(s.service, s.tokenUser(profileID))
	if errors.Is(err, keyring.ErrNotFound) {
		return Session{}, ErrTokenNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("read operating-system credential store: %w", err)
	}
	var token Token
	if err := json.Unmarshal([]byte(encoded), &token); err != nil {
		return Session{}, fmt.Errorf("decode stored OAuth token: %w", err)
	}
	return Session{Profile: profile, Token: token}, nil
}

func (s *KeyringStore) Save(session Session, activate bool) error {
	if session.Profile.ID == "" {
		return errors.New("cannot save an account profile without an ID")
	}
	encoded, err := json.Marshal(session.Token)
	if err != nil {
		return fmt.Errorf("encode OAuth token: %w", err)
	}
	if err := s.backend.Set(s.service, s.tokenUser(session.Profile.ID), string(encoded)); err != nil {
		return fmt.Errorf("write operating-system credential store: %w", err)
	}

	registry, err := s.loadRegistry()
	if err != nil {
		return err
	}
	updated := false
	for index := range registry.Profiles {
		if registry.Profiles[index].ID == session.Profile.ID {
			registry.Profiles[index] = session.Profile
			updated = true
			break
		}
	}
	if !updated {
		registry.Profiles = append(registry.Profiles, session.Profile)
	}
	if activate || registry.Active == "" {
		registry.Active = session.Profile.ID
	}
	return s.saveRegistry(registry)
}

func (s *KeyringStore) SetActive(profileID string) error {
	registry, err := s.loadRegistry()
	if err != nil {
		return err
	}
	if _, ok := findProfileByID(registry.Profiles, profileID); !ok {
		return ErrProfileNotFound
	}
	registry.Active = profileID
	return s.saveRegistry(registry)
}

func (s *KeyringStore) Delete(profileID string) error {
	registry, err := s.loadRegistry()
	if err != nil {
		return err
	}
	if profileID == "" {
		profileID = registry.Active
	}
	index := -1
	for i := range registry.Profiles {
		if registry.Profiles[i].ID == profileID {
			index = i
			break
		}
	}
	if index < 0 {
		return ErrProfileNotFound
	}
	if err := s.backend.Delete(s.service, s.tokenUser(profileID)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete operating-system credential: %w", err)
	}
	registry.Profiles = append(registry.Profiles[:index], registry.Profiles[index+1:]...)
	if registry.Active == profileID {
		registry.Active = ""
		if len(registry.Profiles) > 0 {
			registry.Active = registry.Profiles[0].ID
		}
	}
	if len(registry.Profiles) == 0 {
		if err := s.backend.Delete(s.service, s.registryUser()); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("delete account registry: %w", err)
		}
		return nil
	}
	return s.saveRegistry(registry)
}

func (s *KeyringStore) loadRegistry() (profileRegistry, error) {
	encoded, err := s.backend.Get(s.service, s.registryUser())
	if errors.Is(err, keyring.ErrNotFound) {
		return profileRegistry{Version: 1}, nil
	}
	if err != nil {
		return profileRegistry{}, fmt.Errorf("read account registry from operating-system credential store: %w", err)
	}
	var registry profileRegistry
	if err := json.Unmarshal([]byte(encoded), &registry); err != nil {
		return profileRegistry{}, fmt.Errorf("decode account registry: %w", err)
	}
	if registry.Version == 0 {
		registry.Version = 1
	}
	return registry, nil
}

func (s *KeyringStore) saveRegistry(registry profileRegistry) error {
	registry.Version = 1
	encoded, err := json.Marshal(registry)
	if err != nil {
		return fmt.Errorf("encode account registry: %w", err)
	}
	if err := s.backend.Set(s.service, s.registryUser(), string(encoded)); err != nil {
		return fmt.Errorf("write account registry to operating-system credential store: %w", err)
	}
	return nil
}

func (s *KeyringStore) registryUser() string {
	return "profiles:" + s.namespace
}

func (s *KeyringStore) tokenUser(profileID string) string {
	return "token:" + s.namespace + ":" + profileID
}

func profileID(workspaceID, userID string) string {
	return shortHash(workspaceID + "\x00" + userID)
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}

func findProfileByID(profiles []Profile, id string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}
