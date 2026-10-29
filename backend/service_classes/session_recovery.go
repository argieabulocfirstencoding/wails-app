package service_classes

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	bubbleRecoveryPointerName = "active"
	bubbleCleanExitMarkerName = "clean-exit"
)

type bubbleRecoveryPointer struct {
	UserID      string `json:"user_id"`
	Environment string `json:"environment"`
}

// bubbleCleanExitMarker records that the last process holding an active
// session reached the normal Wails shutdown hook. It contains no credentials;
// it only lets the next process distinguish a deliberate quit from a crash or
// forced termination, where the hook could not run.
type bubbleCleanExitMarker struct {
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

func (f *FesClientService) clearCleanExitMarker() {
	if f.sessionStore != nil {
		_ = f.sessionStore.Delete(bubbleSessionKey(f.sessionScope, bubbleCleanExitMarkerName))
	}
}

// MarkCleanExit records a controlled application shutdown for userID. A
// marker is deliberately consumed during the following startup, so it cannot
// hide a later abnormal termination.
func (f *FesClientService) MarkCleanExit(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	f.sessionMu.Lock()
	store := f.sessionStore
	scope := f.sessionScope
	f.sessionMu.Unlock()
	if store == nil {
		return fmt.Errorf("Bubble session store is unavailable")
	}

	raw, err := json.Marshal(bubbleCleanExitMarker{UserID: userID, Environment: scope})
	if err != nil {
		return fmt.Errorf("encode Bubble clean-exit marker: %w", err)
	}
	if err := store.Set(bubbleSessionKey(scope, bubbleCleanExitMarkerName), string(raw)); err != nil {
		return fmt.Errorf("store Bubble clean-exit marker: %w", err)
	}
	return nil
}

// ConsumeCleanExit reports whether userID was the user that deliberately
// closed the previous app process. Missing or malformed markers are treated as
// an unexpected termination. The marker is removed regardless of its value.
func (f *FesClientService) ConsumeCleanExit(userID string) bool {
	userID = strings.TrimSpace(userID)
	f.sessionMu.Lock()
	store := f.sessionStore
	scope := f.sessionScope
	f.sessionMu.Unlock()
	if userID == "" || store == nil {
		return false
	}

	key := bubbleSessionKey(scope, bubbleCleanExitMarkerName)
	raw, err := store.Get(key)
	if err != nil {
		return false
	}
	_ = store.Delete(key)

	var marker bubbleCleanExitMarker
	if err := json.Unmarshal([]byte(raw), &marker); err != nil {
		return false
	}
	return marker.UserID == userID && marker.Environment == scope
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
