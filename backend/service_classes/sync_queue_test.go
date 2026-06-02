package service_classes

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSyncQueueWaiterUnblocksOnShutdown(t *testing.T) {
	syncService := NewSyncService()
	if err := syncService.Connect(context.Background()); err != nil {
		t.Fatalf("connect sync service: %v", err)
	}

	started := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- syncService.enqueueAndWait(&syncJob{
			key:  "blocking-job",
			name: "blocking_job",
			fn: func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			},
		})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("sync job did not start")
	}

	syncService.Shutdown()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected shutdown error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sync queue waiter did not unblock after shutdown")
	}
}
