package service_classes

import (
	"errors"
	"fesapp/backend/types"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryBubbleSessionStore struct {
	values map[string]string
}

func (s *memoryBubbleSessionStore) Get(key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (s *memoryBubbleSessionStore) Set(key, value string) error {
	s.values[key] = value
	return nil
}

func (s *memoryBubbleSessionStore) Delete(key string) error {
	delete(s.values, key)
	return nil
}

func newTestFesClient(t *testing.T, server *httptest.Server) (*FesClientService, *memoryBubbleSessionStore) {
	t.Helper()
	store := &memoryBubbleSessionStore{values: map[string]string{}}
	client := NewFesClientService(NewConnectivityStateService())
	client.baseUrl = server.URL + "/"
	client.client = server.Client()
	client.sessionScope = "test"
	client.sessionStore = store
	if err := client.SetBubbleSession("user-1", "test-token", time.Hour); err != nil {
		t.Fatalf("set session: %v", err)
	}
	return client, store
}

func TestRequestClearsUnauthorizedSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Invalid or expired token"}`))
	}))
	defer server.Close()

	client, store := newTestFesClient(t, server)

	_, err := client.Request(types.Request{Url: "wf/task_out", Method: http.MethodPost, Body: map[string]any{}})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}

	var stableErr *types.StableError
	if !errors.As(err, &stableErr) {
		t.Fatalf("expected StableError, got %T", err)
	}
	if stableErr.Code != "SESSION_REAUTH_REQUIRED" {
		t.Fatalf("expected SESSION_REAUTH_REQUIRED, got %q", stableErr.Code)
	}
	if _, ok := store.values[bubbleSessionKey("test", "user-1")]; ok {
		t.Fatal("expected unauthorized session to be removed from secure storage")
	}
}

func TestRequestBlocksExpiredSessionBeforeNetworkCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, store := newTestFesClient(t, server)
	store.values[bubbleSessionKey("test", "user-1")] = `{"user_id":"user-1","environment":"test","token":"expired-token","expires_at":"2020-01-01T00:00:00Z"}`
	client.SetActiveSessionUser("user-1")

	_, err := client.Request(types.Request{Url: "wf/task_out", Method: http.MethodPost, Body: map[string]any{}})
	if err == nil {
		t.Fatal("expected expired-session error")
	}
	if called {
		t.Fatal("expired session must not be sent to Bubble")
	}
	if _, ok := store.values[bubbleSessionKey("test", "user-1")]; ok {
		t.Fatal("expected expired session to be removed from secure storage")
	}
}

func TestRequestDoesNotUseSessionFromAnotherEnvironment(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := &memoryBubbleSessionStore{values: map[string]string{}}
	productionClient := NewFesClientService(NewConnectivityStateService())
	productionClient.sessionStore = store
	productionClient.sessionScope = "production"
	if err := productionClient.SetBubbleSession("user-1", "production-token", time.Hour); err != nil {
		t.Fatalf("set production session: %v", err)
	}

	developmentClient := NewFesClientService(NewConnectivityStateService())
	developmentClient.baseUrl = server.URL + "/"
	developmentClient.client = server.Client()
	developmentClient.sessionStore = store
	developmentClient.sessionScope = "development"
	developmentClient.SetActiveSessionUser("user-1")

	_, err := developmentClient.Request(types.Request{Url: "wf/task_out", Method: http.MethodPost, Body: map[string]any{}})
	if err == nil {
		t.Fatal("expected reauthentication error for an environment-scoped session miss")
	}
	if called {
		t.Fatal("a production session must not be sent to the development Bubble app")
	}
}
