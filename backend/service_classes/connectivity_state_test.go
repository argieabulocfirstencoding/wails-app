package service_classes

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSyncUnsyncedRecords_OnlineOfflineTransition(t *testing.T) {
	connectivity := NewConnectivityStateService()
	syncService := NewSyncService(connectivity)

	connectivity.SetOnline(false)
	err := syncService.SyncUnsyncedRecords()
	if err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("expected offline error, got: %v", err)
	}

	connectivity.SetOnline(true)
	err = syncService.SyncUnsyncedRecords()
	if err == nil || !strings.Contains(err.Error(), "authenticated user") {
		t.Fatalf("expected auth user error after online transition, got: %v", err)
	}
}

func TestSyncQueue_RespectsConnectivityTransition(t *testing.T) {
	connectivity := NewConnectivityStateService()
	syncService := NewSyncService(connectivity)
	_ = syncService.Connect(context.Background())
	defer syncService.Shutdown()

	connectivity.SetOnline(false)
	attempts := 0

	job := &syncJob{
		key:  "test-connectivity-transition",
		name: "test_connectivity_transition",
		fn: func(ctx context.Context) error {
			attempts++
			if !connectivity.IsOnline() {
				return fmt.Errorf("temporary connectivity failure")
			}
			return nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- syncService.enqueueAndWait(job)
	}()

	time.Sleep(150 * time.Millisecond)
	connectivity.SetOnline(true)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected successful retry after online transition, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for queue job")
	}

	if attempts < 2 {
		t.Fatalf("expected at least 2 attempts, got: %d", attempts)
	}
}
