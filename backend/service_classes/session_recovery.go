package service_classes

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const bubbleRecoveryPointerName = "active"

type bubbleRecoveryPointer struct {
	UserID      string `json:"user_id"`
	Environment string `json:"environment"`
}

func (f *FesClientService) storeRecoveryPointer(userID string) error {
	if f.sessionStore == nil {
		return fmt.Errorf("Bubble session store is unavailable")
	}
	pointer, err := json.Marshal(bubbleRecoveryPointer{UserID: userID, Environment: f.sessionScope})
	if err != nil {
		return fmt.Errorf("encode Bubble recovery pointer: %w", err)
	}
	return f.sessionStore.Set(bubbleSessionKey(f.sessionScope, bubbleRecoveryPointerName), string(pointer))
}

func (f *FesClientService) clearRecoveryPointer() {
	if f.sessionStore != nil {
		_ = f.sessionStore.Delete(bubbleSessionKey(f.sessionScope, bubbleRecoveryPointerName))
	}
}

// RestoreBubbleSession loads the non-expired session selected by the secure
// recovery pointer. It performs no network request so local recovery also
// works while offline; a later Bubble 401 clears the session as usual.
func (f *FesClientService) RestoreBubbleSession() (string, error) {
	f.sessionMu.Lock()
	store := f.sessionStore
	scope := f.sessionScope
	f.sessionMu.Unlock()
	if store == nil {
		return "", newSessionReauthenticationError()
	}

	rawPointer, err := store.Get(bubbleSessionKey(scope, bubbleRecoveryPointerName))
	if err != nil {
		return "", newSessionReauthenticationError()
	}
	var pointer bubbleRecoveryPointer
	if err := json.Unmarshal([]byte(rawPointer), &pointer); err != nil ||
		strings.TrimSpace(pointer.UserID) == "" || pointer.Environment != scope {
		f.clearRecoveryPointer()
		return "", newSessionReauthenticationError()
	}

	rawSession, err := store.Get(bubbleSessionKey(scope, pointer.UserID))
	if err != nil {
		f.clearRecoveryPointer()
		return "", newSessionReauthenticationError()
	}
	var session bubbleSession
	if err := json.Unmarshal([]byte(rawSession), &session); err != nil ||
		session.UserID != pointer.UserID || session.Environment != scope ||
		strings.TrimSpace(session.Token) == "" || time.Now().Add(bubbleSessionExpiryBuffer).After(session.ExpiresAt) {
		_ = store.Delete(bubbleSessionKey(scope, pointer.UserID))
		f.clearRecoveryPointer()
		return "", newSessionReauthenticationError()
	}

	f.sessionMu.Lock()
	f.sessionUserID = session.UserID
	f.session = &session
	f.sessionMu.Unlock()
	return session.UserID, nil
}
