package service_classes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func awaitUpworkSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for automation cancellation/cleanup")
	}
}

func awaitUpworkCancellation(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("automation did not stop promptly")
	}
}

func fakeUpworkListener() (*upworkEscapeListener, chan struct{}) {
	escape := make(chan struct{}, 2)
	done := make(chan struct{})
	var once sync.Once
	return &upworkEscapeListener{escape: escape, done: done, stop: func() { once.Do(func() { close(done) }) }}, escape
}

func TestUpworkEscapeCancelsAllRunsAndAllowsRestart(t *testing.T) {
	var listeners []*upworkEscapeListener
	var signals []chan struct{}
	m := &upworkCancellation{start: func() (*upworkEscapeListener, error) {
		listener, signal := fakeUpworkListener()
		listeners = append(listeners, listener)
		signals = append(signals, signal)
		return listener, nil
	}}
	t.Cleanup(m.shutdown)
	first, finishFirst, err := m.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, finishSecond, err := m.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listeners) != 1 {
		t.Fatal("concurrent runs must share a listener")
	}
	signals[0] <- struct{}{}
	signals[0] <- struct{}{} // Repeated Esc must be harmless.
	awaitUpworkSignal(t, first.Done())
	awaitUpworkSignal(t, second.Done())
	awaitUpworkSignal(t, listeners[0].done)
	third, finishThird, err := m.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	finishFirst()
	finishSecond()
	if third.Err() != nil {
		t.Fatal("old-run cleanup cancelled the new run")
	}
	if len(listeners) != 2 {
		t.Fatal("expected a fresh listener after cancellation")
	}
	finishThird()
	finishThird()
	awaitUpworkSignal(t, listeners[1].done)
}

func TestUpworkListenerLifecycle(t *testing.T) {
	for _, outcome := range []string{"success", "error", "parent cancellation", "listener exit", "shutdown"} {
		t.Run(outcome, func(t *testing.T) {
			listener, _ := fakeUpworkListener()
			m := &upworkCancellation{start: func() (*upworkEscapeListener, error) { return listener, nil }}
			t.Cleanup(m.shutdown)
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx, finish, err := m.begin(parent)
			if err != nil {
				t.Fatal(err)
			}
			switch outcome {
			case "parent cancellation":
				cancel()
			case "listener exit":
				listener.stop()
			case "shutdown":
				m.shutdown()
			default:
				finish() // Worker defer executes for success and error alike.
			}
			awaitUpworkSignal(t, ctx.Done())
			awaitUpworkSignal(t, listener.done)
			finish()
			m.mu.Lock()
			remaining := len(m.runs)
			m.mu.Unlock()
			if remaining != 0 {
				t.Fatal("finished run remained registered")
			}
			if outcome == "shutdown" {
				if _, _, err := m.begin(context.Background()); err == nil {
					t.Fatal("shutdown accepted another run")
				}
			}
		})
	}
}

func TestUpworkListenerStaysActiveUntilLastRunFinishes(t *testing.T) {
	listener, _ := fakeUpworkListener()
	m := &upworkCancellation{start: func() (*upworkEscapeListener, error) { return listener, nil }}
	t.Cleanup(m.shutdown)
	_, finishFirst, err := m.begin(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, finishSecond, err := m.begin(nil)
	if err != nil {
		t.Fatal(err)
	}
	finishFirst()
	select {
	case <-listener.done:
		t.Fatal("listener stopped before last run finished")
	default:
	}
	if second.Err() != nil {
		t.Fatal("finishing first run cancelled second run")
	}
	finishSecond()
	awaitUpworkSignal(t, listener.done)
}

func TestUpworkLaunchFailsBeforeInputWhenListenerCannotStart(t *testing.T) {
	want := errors.New("hook unavailable")
	f := &FesClientService{upworkCancellation: upworkCancellation{start: func() (*upworkEscapeListener, error) { return nil, want }}}
	creds := &upworkCredentials{Username: "test-user", Password: "test-password"}
	// No logger/browser is needed: neither worker may start after hook failure.
	if err := f.launchClientAccountUpworkNativeAutomation(context.Background(), "test", creds); !errors.Is(err, want) {
		t.Fatalf("native launch: %v", err)
	}
	if err := f.launchUpworkLoginAutomation(context.Background(), "test", creds, false); !errors.Is(err, want) {
		t.Fatalf("DevTools launch: %v", err)
	}
}

func TestUpworkCancellationStopsNativeDelayAndFurtherActions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- performLegacyClientAccountUpworkLogin(ctx, "test-user", "test-password", nil) }()
	cancel()
	awaitUpworkCancellation(t, result)
	if err := performUpworkAction(ctx, func() error { t.Fatal("action executed after cancellation"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := actOnForegroundUpworkClientControl(ctx, 0, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	attempts := 0
	err := retryUpworkClientControl(ctx, "submit", time.Hour, func() error {
		attempts++
		cancel()
		return errors.New("control not ready")
	}, func(string, error) { t.Fatal("cancellation must not report a retry") })
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

type upworkTestTransport func(*http.Request) (*http.Response, error)

func (f upworkTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUpworkCancellationInterruptsTargetDiscovery(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	started := make(chan struct{})
	http.DefaultTransport = upworkTestTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := (&FesClientService{}).waitForUpworkLoginTarget(ctx, time.Hour); result <- err }()
	awaitUpworkSignal(t, started)
	cancel()
	awaitUpworkCancellation(t, result)
}

// A fake DevTools endpoint lets us cancel a command that is already waiting
// for its response, without typing into or submitting a real login page.
func TestUpworkCancellationInterruptsDevToolsAndPreventsFollowupCommands(t *testing.T) {
	for _, operation := range []string{"field discovery", "credential entry", "submit"} {
		t.Run(operation, func(t *testing.T) {
			started := make(chan struct{})
			closed := make(chan struct{})
			var commands atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				defer close(closed)
				for {
					var message chromeDevToolsMessage
					if err := conn.ReadJSON(&message); err != nil {
						return
					}
					if commands.Add(1) == 1 {
						close(started)
					}
					// Deliberately never respond. Cancellation must close the socket.
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			url := "ws" + strings.TrimPrefix(server.URL, "http")
			result := make(chan error, 1)
			if operation == "field discovery" {
				original := http.DefaultTransport
				defer func() { http.DefaultTransport = original }()
				body, _ := json.Marshal([]chromeDevToolsTarget{{ID: "test", Type: "page", URL: upworkChromeLoginURL, WebSocketDebuggerURL: url}})
				http.DefaultTransport = upworkTestTransport(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
				})
				go func() {
					_, _, err := (&FesClientService{}).waitForUpworkFieldEntry(ctx, "username", upworkUsernameFieldSelectors, "test-user", time.Hour)
					result <- err
				}()
			} else {
				client, err := newChromeDevToolsClient(ctx, url)
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				go func() {
					if operation == "submit" {
						result <- client.pressEnter()
						return
					}
					_, err := client.setFocusedFieldValue("input", "test-value")
					result <- err
				}()
			}
			awaitUpworkSignal(t, started)
			cancel()
			awaitUpworkCancellation(t, result)
			awaitUpworkSignal(t, closed)
			if commands.Load() != 1 {
				t.Fatalf("sent %d commands; expected no fallback, retry, or followup", commands.Load())
			}
		})
	}
}

func TestUpworkNativeEscapeListenerCanStartStopAndRestart(t *testing.T) {
	// Installs the real Windows hook but never injects keyboard input.
	for i := 0; i < 2; i++ {
		listener, err := startUpworkEscapeListener()
		if err != nil {
			t.Fatal(err)
		}
		stopped := make(chan struct{})
		go func() { listener.stop(); listener.stop(); close(stopped) }()
		awaitUpworkSignal(t, stopped)
		awaitUpworkSignal(t, listener.done)
	}
}

func TestUpworkCancellationInterruptsWebSocketHandshake(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // Never send the upgrade response until the test is done.
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		client, err := newChromeDevToolsClient(ctx, "ws"+strings.TrimPrefix(server.URL, "http"))
		if client != nil {
			client.Close()
		}
		result <- err
	}()
	awaitUpworkSignal(t, started)
	cancel()
	awaitUpworkCancellation(t, result)
}
