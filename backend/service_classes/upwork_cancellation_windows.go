package service_classes

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

// The listener is shared by all login runs on a service. Its native callback
// only queues Esc; cancellation and cleanup happen outside the hook thread.
type upworkEscapeListener struct {
	escape <-chan struct{}
	done   <-chan struct{}
	stop   func()
}

type upworkCancellation struct {
	mu       sync.Mutex
	runs     map[context.Context]context.CancelFunc
	listener *upworkEscapeListener
	start    func() (*upworkEscapeListener, error)
	closed   bool
}

func (m *upworkCancellation) begin(parent context.Context) (context.Context, func(), error) {
	if parent == nil {
		parent = context.Background()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, nil, fmt.Errorf("Upwork automation is shutting down")
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	if m.listener == nil {
		start := m.start
		if start == nil {
			start = startUpworkEscapeListener
		}
		listener, err := start()
		if err != nil {
			return nil, nil, fmt.Errorf("start Upwork Esc shortcut: %w", err)
		}
		m.listener = listener
		go m.watch(listener)
	}
	ctx, cancel := context.WithCancel(parent)
	if m.runs == nil {
		m.runs = make(map[context.Context]context.CancelFunc)
	}
	m.runs[ctx] = cancel
	var once sync.Once
	finish := func() {
		once.Do(func() {
			cancel()
			m.mu.Lock()
			defer m.mu.Unlock()
			delete(m.runs, ctx)
			if len(m.runs) == 0 && m.listener != nil {
				m.listener.stop()
				m.listener = nil
			}
		})
	}
	// Also release the hook if a native UI Automation call is still returning.
	context.AfterFunc(ctx, finish)
	return ctx, finish, nil
}

func (m *upworkCancellation) watch(listener *upworkEscapeListener) {
	for {
		select {
		case <-listener.escape:
			m.cancelListenerRuns(listener)
		case <-listener.done:
			// An unexpected message-loop exit must stop automation too.
			m.cancelListenerRuns(listener)
			return
		}
	}
}

func (m *upworkCancellation) cancelListenerRuns(listener *upworkEscapeListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listener == listener {
		for _, cancel := range m.runs {
			cancel()
		}
		listener.stop()
		m.listener = nil
	}
}

func performUpworkAction(ctx context.Context, action func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return err
	}
	return ctx.Err()
}

func (m *upworkCancellation) shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, cancel := range m.runs {
		cancel()
	}
	if m.listener != nil {
		m.listener.stop()
		m.listener = nil
	}
}

func (f *FesClientService) StopUpworkAutomations() {
	f.upworkCancellation.shutdown()
}

var (
	upworkSetWindowsHookEx    = user32DLL.NewProc("SetWindowsHookExW")
	upworkUnhookWindowsHookEx = user32DLL.NewProc("UnhookWindowsHookEx")
	upworkCallNextHookEx      = user32DLL.NewProc("CallNextHookEx")
	upworkPostThreadMessage   = user32DLL.NewProc("PostThreadMessageW")
	upworkHookThreads         sync.Map // thread ID -> chan struct{}
	// Windows callback allocations cannot be freed. Reuse one for every launch.
	upworkKeyboardCallback = windows.NewCallback(upworkKeyboardHook)
)

func upworkKeyboardHook(code int32, message uintptr, data uintptr) uintptr {
	if code >= 0 && (message == win.WM_KEYDOWN || message == win.WM_SYSKEYDOWN) && data != 0 {
		// vkCode is the first DWORD of KBDLLHOOKSTRUCT.
		if *(*uint32)(unsafe.Pointer(data)) == win.VK_ESCAPE {
			if value, ok := upworkHookThreads.Load(windows.GetCurrentThreadId()); ok {
				select {
				case value.(chan struct{}) <- struct{}{}:
				default:
				}
			}
		}
	}
	result, _, _ := upworkCallNextHookEx.Call(0, uintptr(code), message, data)
	return result
}

func startUpworkEscapeListener() (*upworkEscapeListener, error) {
	escape := make(chan struct{}, 1)
	done := make(chan struct{})
	ready := make(chan error, 1)
	var threadID uint32
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		threadID = windows.GetCurrentThreadId()
		var msg win.MSG
		// Create the thread's message queue before allowing stop to post WM_QUIT.
		win.PeekMessage(&msg, 0, 0, 0, win.PM_NOREMOVE)
		upworkHookThreads.Store(threadID, escape)
		defer upworkHookThreads.Delete(threadID)
		hook, _, err := upworkSetWindowsHookEx.Call(13 /* WH_KEYBOARD_LL */, upworkKeyboardCallback, uintptr(win.GetModuleHandle(nil)), 0)
		if hook == 0 {
			ready <- fmt.Errorf("install keyboard hook: %w", err)
			return
		}
		defer upworkUnhookWindowsHookEx.Call(hook)
		ready <- nil
		for win.GetMessage(&msg, 0, 0, 0) > 0 {
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}()
	if err := <-ready; err != nil {
		<-done
		return nil, err
	}
	var once sync.Once
	return &upworkEscapeListener{escape: escape, done: done, stop: func() {
		once.Do(func() {
			upworkPostThreadMessage.Call(uintptr(threadID), win.WM_QUIT, 0, 0)
			<-done
		})
	}}, nil
}
