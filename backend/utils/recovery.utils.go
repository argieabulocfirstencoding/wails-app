package utils

import (
	"fmt"
	"runtime/debug"
)

func RecoverPanic(scope string) {
	if r := recover(); r != nil {
		Logger.Printf("panic recovered scope=%s err=%v stack=%s", scope, r, string(debug.Stack()))
	}
}

func GoSafe(scope string, fn func()) {
	if fn == nil {
		return
	}

	go func() {
		defer RecoverPanic(scope)
		fn()
	}()
}

func SafeExecute(scope string, fn func() error) (err error) {
	if fn == nil {
		return fmt.Errorf("nil function for scope %s", scope)
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered scope=%s err=%v", scope, r)
			Logger.Printf("%v stack=%s", err, string(debug.Stack()))
		}
	}()

	return fn()
}
