package service_classes

import (
	"errors"
	"fesapp/backend/types"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFesAPIBaseURLUsesTheDevelopmentBubbleAppForDevelopmentBuilds(t *testing.T) {
	if got := fesAPIBaseURL("development", false); got != developmentFESAPIBaseURL {
		t.Fatalf("expected development build to use %q, got %q", developmentFESAPIBaseURL, got)
	}
	if got := fesAPIBaseURL("production", true); got != developmentFESAPIBaseURL {
		t.Fatalf("expected local development override to use %q, got %q", developmentFESAPIBaseURL, got)
	}
}

func TestFesAPIBaseURLUsesTheProductionBubbleAppForProductionBuilds(t *testing.T) {
	if got := fesAPIBaseURL("production", false); got != productionFESAPIBaseURL {
		t.Fatalf("expected production build to use %q, got %q", productionFESAPIBaseURL, got)
	}
}

func TestRequestPreservesUnauthorizedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Invalid or expired token"}`))
	}))
	defer server.Close()

	client := NewFesClientService(NewConnectivityStateService())
	client.baseUrl = server.URL + "/"
	client.client = server.Client()
	client.apiToken = "test-token"

	_, err := client.Request(types.Request{Url: "wf/task_out", Method: http.MethodPost, Body: map[string]any{}})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}

	var remoteErr *types.RemoteAPIError
	if !errors.As(err, &remoteErr) {
		t.Fatalf("expected RemoteAPIError, got %T", err)
	}
	if remoteErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", remoteErr.StatusCode)
	}
}
