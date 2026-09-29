package service_classes

import (
	"encoding/json"
	"fesapp/backend/types"
	"fmt"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	bubbleSessionKeyringService = "fesapp.bubble.session"
	bubbleSessionExpiryBuffer   = 5 * time.Minute
)

type bubbleSessionStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

type systemBubbleSessionStore struct{}

func (systemBubbleSessionStore) Get(key string) (string, error) {
	return keyring.Get(bubbleSessionKeyringService, key)
}

func (systemBubbleSessionStore) Set(key, value string) error {
	return keyring.Set(bubbleSessionKeyringService, key, value)
}

func (systemBubbleSessionStore) Delete(key string) error {
	return keyring.Delete(bubbleSessionKeyringService, key)
}

type bubbleSession struct {
	UserID      string    `json:"user_id"`
	Environment string    `json:"environment"`
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func bubbleSessionKey(environment, userID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(environment), strings.TrimSpace(userID))
}

func newSessionReauthenticationError() error {
	return types.NewStableError("SESSION_REAUTH_REQUIRED", "Your Bubble session needs renewal. Sign in to resume sync; your tracked time is saved locally.")
}

func (f *FesClientService) activeBubbleSession() (*bubbleSession, error) {
	f.sessionMu.Lock()
	if f.session != nil {
		session := *f.session
		f.sessionMu.Unlock()
		return &session, nil
	}
	userID := f.sessionUserID
	scope := f.sessionScope
	store := f.sessionStore
	f.sessionMu.Unlock()

	if userID == "" || store == nil {
		return nil, newSessionReauthenticationError()
	}

	raw, err := store.Get(bubbleSessionKey(scope, userID))
	if err != nil {
		return nil, newSessionReauthenticationError()
	}

	var session bubbleSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil ||
		strings.TrimSpace(session.UserID) != userID ||
		strings.TrimSpace(session.Environment) != scope ||
		strings.TrimSpace(session.Token) == "" ||
		session.ExpiresAt.IsZero() {
		return nil, newSessionReauthenticationError()
	}

	f.sessionMu.Lock()
	if f.sessionUserID == userID && f.session == nil {
		f.session = &session
	}
	f.sessionMu.Unlock()
	return &session, nil
}

func (f *FesClientService) storeBubbleSession(session *bubbleSession) error {
	if f.sessionStore == nil {
		return fmt.Errorf("Bubble session store is unavailable")
	}

	raw, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode Bubble session: %w", err)
	}
	if err := f.sessionStore.Set(bubbleSessionKey(session.Environment, session.UserID), string(raw)); err != nil {
		return fmt.Errorf("store Bubble session: %w", err)
	}
	return nil
}
