package auth

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

type memoryKeyring struct {
	values map[string]string
}

func newMemoryKeyring() *memoryKeyring {
	return &memoryKeyring{values: make(map[string]string)}
}

func (k *memoryKeyring) key(service, user string) string {
	return service + "\x00" + user
}

func (k *memoryKeyring) Get(service, user string) (string, error) {
	value, ok := k.values[k.key(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (k *memoryKeyring) Set(service, user, password string) error {
	k.values[k.key(service, user)] = password
	return nil
}

func (k *memoryKeyring) Delete(service, user string) error {
	key := k.key(service, user)
	if _, ok := k.values[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(k.values, key)
	return nil
}

func TestKeyringStoreKeepsMultipleProfilesIsolated(t *testing.T) {
	backend := newMemoryKeyring()
	store := NewKeyringStore("client-123")
	store.backend = backend
	work := Session{Profile: Profile{ID: "work", WorkspaceName: "Work", UserEmail: "work@example.com"}, Token: Token{AccessToken: "work-token"}}
	personal := Session{Profile: Profile{ID: "personal", WorkspaceName: "Personal", UserEmail: "me@example.com"}, Token: Token{AccessToken: "personal-token"}}

	if err := store.Save(work, true); err != nil {
		t.Fatalf("save work: %v", err)
	}
	if err := store.Save(personal, true); err != nil {
		t.Fatalf("save personal: %v", err)
	}
	profiles, err := store.List()
	if err != nil || len(profiles) != 2 {
		t.Fatalf("profiles = %#v, error = %v", profiles, err)
	}
	active, err := store.Active()
	if err != nil || active != "personal" {
		t.Fatalf("active = %q, error = %v", active, err)
	}
	loaded, err := store.Load("work")
	if err != nil || loaded.Token.AccessToken != "work-token" {
		t.Fatalf("loaded = %#v, error = %v", loaded, err)
	}
	if err := store.SetActive("work"); err != nil {
		t.Fatalf("set active: %v", err)
	}
	loaded, err = store.Load("")
	if err != nil || loaded.Profile.ID != "work" {
		t.Fatalf("active session = %#v, error = %v", loaded, err)
	}
}

func TestKeyringStoreDeleteSelectsRemainingProfile(t *testing.T) {
	store := NewKeyringStore("client-123")
	store.backend = newMemoryKeyring()
	_ = store.Save(Session{Profile: Profile{ID: "work"}, Token: Token{AccessToken: "work"}}, true)
	_ = store.Save(Session{Profile: Profile{ID: "personal"}, Token: Token{AccessToken: "personal"}}, true)

	if err := store.Delete("personal"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	active, err := store.Active()
	if err != nil || active != "work" {
		t.Fatalf("active = %q, error = %v", active, err)
	}
	if _, err := store.Load("personal"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("deleted profile error = %v", err)
	}
}

func TestProfileIDIncludesWorkspaceAndUser(t *testing.T) {
	workUser := profileID("work", "same-user")
	personalUser := profileID("personal", "same-user")
	secondWorkUser := profileID("work", "second-user")
	if workUser == personalUser || workUser == secondWorkUser || personalUser == secondWorkUser {
		t.Fatal("workspace-user combinations must produce distinct profile IDs")
	}
}
