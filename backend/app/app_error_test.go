package app

import (
	"errors"
	"fesapp/backend/types"
	"testing"
)

func TestStableClientErrorMapsExpiredBubbleSession(t *testing.T) {
	a := NewApp()
	err := a.stableClientError("sync_unsynced_records", types.NewRemoteAPIError("FES API", 401, "Invalid or expired token"))

	var stable *types.StableError
	if !errors.As(err, &stable) {
		t.Fatalf("expected StableError, got %T", err)
	}
	if stable.Code != "SESSION_REAUTH_REQUIRED" {
		t.Fatalf("expected SESSION_REAUTH_REQUIRED, got %q", stable.Code)
	}
}
