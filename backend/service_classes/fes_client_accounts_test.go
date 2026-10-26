package service_classes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lxn/win"
	uia "github.com/sjuhan/w32uiautomation"
)

var errNoMatchingChromeProfile = errors.New("no matching Chrome profile")

func TestParseClientAccountRecords(t *testing.T) {
	response := map[string]any{
		"client_lists": []any{
			map[string]any{
				"_id":                    "1721878290336x545953098524262400",
				"Platform":               "Upwork",
				"Display":                "Upwork (Johnna-mae)",
				"Holder":                 "1721796252301x906548332130730000",
				"credentials - email":    "johnna-mae\n",
				"credentials - password": "in8(Lh;53$WG\n",
				"Google profile":         "johnnamaeyodico@gmail.com",
			},
			map[string]any{
				"_id":      "1722379406624x296992370222170100",
				"Platform": "Upwork",
				"Display":  "Upwork (Dhea)",
			},
		},
	}

	records, found, err := parseClientAccountRecords(response)
	if err != nil {
		t.Fatalf("parseClientAccountRecords returned error: %v", err)
	}
	if !found {
		t.Fatalf("expected client_lists to be detected")
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Username != "johnna-mae" {
		t.Fatalf("expected trimmed username, got %q", records[0].Username)
	}
	if records[0].Password != "in8(Lh;53$WG" {
		t.Fatalf("expected trimmed password, got %q", records[0].Password)
	}
	if records[1].Display != "Upwork (Dhea)" {
		t.Fatalf("expected second display to be preserved, got %q", records[1].Display)
	}
}

func TestParseClientAccountRecordsMissingField(t *testing.T) {
	records, found, err := parseClientAccountRecords(map[string]any{
		"results": []any{},
	})
	if err != nil {
		t.Fatalf("expected no error when client_lists is absent, got %v", err)
	}
	if found {
		t.Fatalf("expected client_lists absence to report found=false")
	}
	if records != nil {
		t.Fatalf("expected nil records when client_lists is absent")
	}
}

func TestParseUpworkCredentialsResponse(t *testing.T) {
	response := map[string]any{
		"username": "johnna-mae@example.com\n",
		"password": "super-secret-password\n",
	}

	creds := parseUpworkCredentialsResponse(response)
	if creds.Username != "johnna-mae@example.com" {
		t.Fatalf("expected username to map from response username, got %q", creds.Username)
	}
	if creds.Password != "super-secret-password" {
		t.Fatalf("expected password to map from response password, got %q", creds.Password)
	}
}

func TestDirectUpworkCredentialsFromRecord(t *testing.T) {
	record := &clientAccountRecord{
		Username: "johnna-mae@example.com\n",
		Password: "super-secret-password\n",
	}

	creds := directUpworkCredentialsFromRecord(record)
	if creds == nil {
		t.Fatalf("expected direct credentials to be returned")
	}
	if creds.Username != "johnna-mae@example.com" {
		t.Fatalf("expected trimmed direct username, got %q", creds.Username)
	}
	if creds.Password != "super-secret-password" {
		t.Fatalf("expected trimmed direct password, got %q", creds.Password)
	}
}

func TestDirectUpworkCredentialsFromRecordReturnsNilWhenIncomplete(t *testing.T) {
	record := &clientAccountRecord{
		Username: "johnna-mae@example.com",
	}

	creds := directUpworkCredentialsFromRecord(record)
	if creds != nil {
		t.Fatalf("expected nil direct credentials when password is missing")
	}
}

func TestResolveChromeProfileDirectoryFromLocalStateMatchesUserName(t *testing.T) {
	rawLocalState := []byte(`{
		"profile": {
			"info_cache": {
				"Profile 1": {
					"user_name": "other@example.com",
					"name": "Other"
				},
				"Profile 27": {
					"user_name": "johnnamaeyodico@gmail.com",
					"name": "Johnna"
				}
			}
		}
	}`)

	profileDirectory, err := resolveChromeProfileDirectoryFromLocalState(rawLocalState, " johnnamaeyodico@gmail.com ")
	if err != nil {
		t.Fatalf("expected profile directory match, got error: %v", err)
	}
	if profileDirectory != "Profile 27" {
		t.Fatalf("expected Profile 27, got %q", profileDirectory)
	}
}

func TestResolveChromeProfileDirectoryFromLocalStateMatchesProfileNameFallback(t *testing.T) {
	rawLocalState := []byte(`{
		"profile": {
			"info_cache": {
				"Profile 99": {
					"user_name": "",
					"name": "Johnna Mae"
				}
			}
		}
	}`)

	profileDirectory, err := resolveChromeProfileDirectoryFromLocalState(rawLocalState, "johnna mae")
	if err != nil {
		t.Fatalf("expected fallback profile name match, got error: %v", err)
	}
	if profileDirectory != "Profile 99" {
		t.Fatalf("expected Profile 99, got %q", profileDirectory)
	}
}

func TestResolveChromeProfileDirectoryFromLocalStateRejectsMissingMatch(t *testing.T) {
	rawLocalState := []byte(`{
		"profile": {
			"info_cache": {
				"Profile 3": {
					"user_name": "someone@example.com"
				}
			}
		}
	}`)

	_, err := resolveChromeProfileDirectoryFromLocalState(rawLocalState, "johnnamaeyodico@gmail.com")
	if err == nil {
		t.Fatalf("expected error when no local chrome profile matches")
	}
}

func TestFesWebAppChromeLaunchTargetUsesMatchingEmployeeProfile(t *testing.T) {
	profileDirectory, useIncognito := fesWebAppChromeLaunchTarget(
		"employee@firstencoding.app",
		func(email string) (string, error) {
			if email != "employee@firstencoding.app" {
				t.Fatalf("resolver received %q, want employee email", email)
			}
			return "Profile 4", nil
		},
	)

	if useIncognito {
		t.Fatal("expected matching Chrome profile to be used")
	}
	if profileDirectory != "Profile 4" {
		t.Fatalf("profile directory = %q, want %q", profileDirectory, "Profile 4")
	}
}

func TestFesWebAppChromeLaunchTargetFallsBackToIncognito(t *testing.T) {
	tests := []struct {
		name          string
		employeeEmail string
		resolver      func(string) (string, error)
	}{
		{
			name:          "employee email is unavailable",
			employeeEmail: " ",
			resolver: func(string) (string, error) {
				t.Fatal("resolver must not run without an employee email")
				return "", nil
			},
		},
		{
			name:          "no local Chrome profile matches",
			employeeEmail: "employee@firstencoding.app",
			resolver: func(string) (string, error) {
				return "", errNoMatchingChromeProfile
			},
		},
		{
			name:          "matching profile has no directory",
			employeeEmail: "employee@firstencoding.app",
			resolver: func(string) (string, error) {
				return " ", nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profileDirectory, useIncognito := fesWebAppChromeLaunchTarget(test.employeeEmail, test.resolver)
			if !useIncognito {
				t.Fatal("expected incognito fallback")
			}
			if profileDirectory != "" {
				t.Fatalf("profile directory = %q, want empty", profileDirectory)
			}
		})
	}
}

func TestDescribeCredentialValue(t *testing.T) {
	got := describeCredentialValue("johnna-mae@example.com")
	want := "len=22 has_at=true"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	if empty := describeCredentialValue("   "); empty != "empty" {
		t.Fatalf("expected empty description, got %q", empty)
	}
}

func TestUpworkLoginFormValuesUsesUsernameFirstThenPassword(t *testing.T) {
	first, second, err := upworkLoginFormValues(&upworkCredentials{
		Username: "johnna-mae@example.com\n",
		Password: "super-secret-password\n",
	})
	if err != nil {
		t.Fatalf("expected login form values without error, got %v", err)
	}
	if first != "johnna-mae@example.com" {
		t.Fatalf("expected first login form value to be username, got %q", first)
	}
	if second != "super-secret-password" {
		t.Fatalf("expected second login form value to be password, got %q", second)
	}
}

func TestUpworkLoginFormValuesRejectsIncompleteCredentials(t *testing.T) {
	_, _, err := upworkLoginFormValues(&upworkCredentials{
		Password: "super-secret-password",
	})
	if err == nil {
		t.Fatalf("expected error for incomplete login form credentials")
	}
}

func TestChromeDevToolsInsertTextRejectsEmptyValue(t *testing.T) {
	client := &chromeDevToolsClient{}
	if err := client.insertText(" \n "); err == nil {
		t.Fatal("expected empty text to be rejected")
	}
}

func TestUpworkChromeLaunchArgsUseIsolatedLocalDevToolsProfile(t *testing.T) {
	userDataDir := filepath.Join(os.TempDir(), upworkChromeUserDataDirPrefix+"test")
	args, err := upworkChromeLaunchArgs(userDataDir, " https://www.upwork.com/nx/login ")
	if err != nil {
		t.Fatalf("expected launch args without error, got %v", err)
	}

	want := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		"--user-data-dir=" + userDataDir,
		"--remote-debugging-address=127.0.0.1",
		chromeRemoteDebuggingFlag(0),
		"https://www.upwork.com/nx/login",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("launch args = %#v, want %#v", args, want)
	}
}

func TestUpworkChromeLaunchArgsRejectMissingProfileDirectory(t *testing.T) {
	if _, err := upworkChromeLaunchArgs(" ", upworkChromeLoginURL); err == nil {
		t.Fatal("expected missing isolated profile directory to be rejected")
	}
}

func TestUpworkBrowserSessionReadsChromeSelectedDevToolsPort(t *testing.T) {
	userDataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDataDir, "DevToolsActivePort"), []byte("45123\n/devtools/browser/test\n"), 0600); err != nil {
		t.Fatal(err)
	}

	port, err := (&upworkBrowserSession{userDataDir: userDataDir}).devToolsPort()
	if err != nil {
		t.Fatalf("read DevTools port: %v", err)
	}
	if port != 45123 {
		t.Fatalf("port = %d, want 45123", port)
	}
}

func TestUpworkBrowserSessionRejectsInvalidDevToolsPort(t *testing.T) {
	userDataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDataDir, "DevToolsActivePort"), []byte("not-a-port\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&upworkBrowserSession{userDataDir: userDataDir}).devToolsPort(); err == nil {
		t.Fatal("expected invalid DevTools port to fail")
	}
}

func TestNativeUnicodeKeyInputs(t *testing.T) {
	inputs, err := nativeUnicodeKeyInputs("A🙂")
	if err != nil {
		t.Fatalf("build native unicode key inputs: %v", err)
	}
	if len(inputs) != 6 {
		t.Fatalf("unicode input count = %d, want 6", len(inputs))
	}

	for index, input := range inputs {
		if input.Type != win.INPUT_KEYBOARD || input.Ki.DwFlags&win.KEYEVENTF_UNICODE == 0 {
			t.Fatalf("input %d is not a unicode keyboard event: %#v", index, input)
		}
		wantKeyUp := index%2 == 1
		if gotKeyUp := input.Ki.DwFlags&win.KEYEVENTF_KEYUP != 0; gotKeyUp != wantKeyUp {
			t.Fatalf("input %d key-up=%t, want %t", index, gotKeyUp, wantKeyUp)
		}
	}

	if _, err := nativeUnicodeKeyInputs(" \n "); err == nil {
		t.Fatal("expected empty native text to be rejected")
	}
}

func TestNativeVirtualKeyInputs(t *testing.T) {
	inputs := nativeVirtualKeyInputs(0x0D)
	if len(inputs) != 2 {
		t.Fatalf("virtual key input count = %d, want 2", len(inputs))
	}
	if inputs[0].Ki.WVk != 0x0D || inputs[0].Ki.DwFlags != 0 {
		t.Fatalf("unexpected virtual key-down input: %#v", inputs[0])
	}
	if inputs[1].Ki.WVk != 0x0D || inputs[1].Ki.DwFlags != win.KEYEVENTF_KEYUP {
		t.Fatalf("unexpected virtual key-up input: %#v", inputs[1])
	}
}

func TestUpworkClientControlTypeName(t *testing.T) {
	tests := []struct {
		controlType int
		want        string
	}{
		{controlType: uia.UIA_EditControlTypeId, want: "input"},
		{controlType: uia.UIA_ButtonControlTypeId, want: "button"},
		{controlType: 0, want: "automation"},
	}

	for _, test := range tests {
		if got := upworkClientControlTypeName(test.controlType); got != test.want {
			t.Errorf("control type %d = %q, want %q", test.controlType, got, test.want)
		}
	}
}

func TestRetryUpworkClientControlRetriesUntilControlActionSucceeds(t *testing.T) {
	attempts := 0
	var retryStages []string
	err := retryUpworkClientControl(context.Background(), "username input", time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("input is not ready")
		}
		return nil
	}, func(stage string, err error) {
		if err == nil {
			t.Fatal("retry error is required")
		}
		retryStages = append(retryStages, stage)
	})
	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if !reflect.DeepEqual(retryStages, []string{"username input"}) {
		t.Fatalf("retry stages = %#v, want username input once", retryStages)
	}
}

func TestRetryUpworkClientControlStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := retryUpworkClientControl(ctx, "password input", time.Millisecond, func() error {
		attempts++
		return nil
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if attempts != 0 {
		t.Fatalf("attempts = %d, want 0 after cancellation", attempts)
	}
}

func TestWaitForUpworkClientDelayStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForUpworkClientDelay(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
}

func TestRemoveManagedUpworkChromeUserDataDirRejectsUnmanagedDirectory(t *testing.T) {
	unmanagedDir := filepath.Join(os.TempDir(), "other-chrome-profile")
	if err := removeManagedUpworkChromeUserDataDir(unmanagedDir); err == nil {
		t.Fatal("expected unmanaged directory removal to be rejected")
	}
}

func TestUpworkChromeLoginURLUsesProvidedOAuthFlow(t *testing.T) {
	if upworkChromeLoginURL != "https://www.upwork.com/ab/account-security/login?redir=%2Fab%2Faccount-security%2Foauth2%2Fauthorize%3Fresponse_type%3Dcode%26redirect_uri%3Dupwork%253A%252F%252Fwww.upwork.com%252Foauth2redir%26client_id%3D522767a70366bdd00e95cd6e8756940c" {
		t.Fatalf("expected provided OAuth login URL, got %q", upworkChromeLoginURL)
	}
}

func TestIsUpworkLoginPageURL(t *testing.T) {
	if !isUpworkLoginPageURL("https://www.upwork.com/ab/account-security/login") {
		t.Fatalf("expected account-security login URL to match")
	}
	if !isUpworkLoginPageURL("https://www.upwork.com/nx/login") {
		t.Fatalf("expected nx login URL to match")
	}
	if isUpworkLoginPageURL("https://www.example.com/login") {
		t.Fatalf("expected non-Upwork login URL to be rejected")
	}
}

func TestSelectUpworkLoginTargetPrefersLoginPage(t *testing.T) {
	targets := []chromeDevToolsTarget{
		{
			Type:                 "page",
			URL:                  "https://www.upwork.com/freelancers/",
			WebSocketDebuggerURL: "ws://127.0.0.1/devtools/page/1",
		},
		{
			Type:                 "page",
			URL:                  "https://www.upwork.com/ab/account-security/login",
			WebSocketDebuggerURL: "ws://127.0.0.1/devtools/page/2",
		},
	}

	selected := selectUpworkLoginTarget(targets)
	if selected == nil {
		t.Fatalf("expected a matching Upwork target")
	}
	if selected.WebSocketDebuggerURL != "ws://127.0.0.1/devtools/page/2" {
		t.Fatalf("expected login target to be selected, got %q", selected.WebSocketDebuggerURL)
	}
}

func TestSelectUpworkLoginTargetFallsBackToAnyUpworkPage(t *testing.T) {
	targets := []chromeDevToolsTarget{
		{
			Type:                 "page",
			URL:                  "https://www.upwork.com/freelancers/",
			WebSocketDebuggerURL: "ws://127.0.0.1/devtools/page/1",
		},
		{
			Type:                 "page",
			URL:                  "https://www.example.com/login",
			WebSocketDebuggerURL: "ws://127.0.0.1/devtools/page/2",
		},
	}

	selected := selectUpworkLoginTarget(targets)
	if selected == nil {
		t.Fatalf("expected Upwork page fallback target")
	}
	if selected.WebSocketDebuggerURL != "ws://127.0.0.1/devtools/page/1" {
		t.Fatalf("expected Upwork fallback target, got %q", selected.WebSocketDebuggerURL)
	}
}

func TestBuildUpworkFieldReadinessExpressionIncludesReadinessChecks(t *testing.T) {
	expression, err := buildUpworkFieldReadinessExpression([]string{`input#username_username`, `input[name="username[username]"]`, `input[type="password"]`})
	if err != nil {
		t.Fatalf("expected field readiness expression without error, got %v", err)
	}
	if expression == "" {
		t.Fatalf("expected readiness expression to be non-empty")
	}
	if !strings.Contains(expression, `input#username_username`) {
		t.Fatalf("expected expression to contain the current username ID selector, got %q", expression)
	}
	if !strings.Contains(expression, `input[name=\"username[username]\"]`) {
		t.Fatalf("expected expression to contain the current username name selector, got %q", expression)
	}
	if !strings.Contains(expression, `input[type=\"password\"]`) {
		t.Fatalf("expected expression to contain serialized password selector, got %q", expression)
	}
	if !strings.Contains(expression, `scrollIntoView`) {
		t.Fatalf("expected expression to scroll the field into view, got %q", expression)
	}
	if !strings.Contains(expression, `document.readyState !== "complete"`) {
		t.Fatalf("expected expression to wait for the document to be complete, got %q", expression)
	}
	if !strings.Contains(expression, `document.elementFromPoint`) {
		t.Fatalf("expected expression to check that the field is unobstructed, got %q", expression)
	}
	if strings.Contains(expression, `dispatchEvent(new MouseEvent`) || strings.Contains(expression, `field.click()`) {
		t.Fatalf("expected trusted DevTools click path without synthetic DOM clicks, got %q", expression)
	}
}

func TestUpworkUsernameFieldSelectorsSupportCurrentLoginPage(t *testing.T) {
	want := []string{`input#username_username`, `input[name="username[username]"]`}
	for index, selector := range want {
		if len(upworkUsernameFieldSelectors) <= index {
			t.Fatalf("username selectors = %#v, want at least %d entries", upworkUsernameFieldSelectors, len(want))
		}
		if upworkUsernameFieldSelectors[index] != selector {
			t.Fatalf("username selector %d = %q, want %q", index, upworkUsernameFieldSelectors[index], selector)
		}
	}
}

func TestBuildUpworkFieldValueMatchExpression(t *testing.T) {
	expression, err := buildUpworkFieldValueMatchExpression(`input[name="username"]`, "test-value")
	if err != nil {
		t.Fatalf("expected value match expression without error, got %v", err)
	}
	if !strings.Contains(expression, `field.value === expectedValue`) {
		t.Fatalf("expected expression to compare the actual field value, got %q", expression)
	}
	if !strings.Contains(expression, `document.activeElement === field`) {
		t.Fatalf("expected expression to require the focused field, got %q", expression)
	}
}

func TestBuildUpworkNativeFieldValueSetExpression(t *testing.T) {
	expression, err := buildUpworkNativeFieldValueSetExpression(`input[type="password"]`, "test-value")
	if err != nil {
		t.Fatalf("expected native value-set expression without error, got %v", err)
	}
	for _, expected := range []string{
		`HTMLInputElement.prototype`,
		`Object.getOwnPropertyDescriptor`,
		`new InputEvent("input"`,
		`new Event("change"`,
		`field.value === expectedValue`,
	} {
		if !strings.Contains(expression, expected) {
			t.Fatalf("expected native value-set expression to contain %q, got %q", expected, expression)
		}
	}
}

func TestParseUpworkReadyField(t *testing.T) {
	field, err := parseUpworkReadyField(map[string]any{
		"selector": `input[name="username"]`,
		"x":        120.5,
		"y":        250.75,
	})
	if err != nil {
		t.Fatalf("expected ready field without error, got %v", err)
	}
	if field == nil || field.Selector != `input[name="username"]` || field.X != 120.5 || field.Y != 250.75 {
		t.Fatalf("unexpected parsed ready field: %#v", field)
	}

	if field, err := parseUpworkReadyField(nil); err != nil || field != nil {
		t.Fatalf("expected absent ready field without error, got field=%#v err=%v", field, err)
	}
}

func TestUpworkFieldPositionIsStable(t *testing.T) {
	previous := &upworkReadyField{Selector: "input[name=username]", X: 100, Y: 200}
	if !upworkFieldPositionIsStable(previous, &upworkReadyField{Selector: "input[name=username]", X: 101.5, Y: 198.5}) {
		t.Fatal("expected nearby consecutive field positions to be stable")
	}
	if upworkFieldPositionIsStable(previous, &upworkReadyField{Selector: "input[name=password]", X: 100, Y: 200}) {
		t.Fatal("expected different fields to be unstable")
	}
	if upworkFieldPositionIsStable(previous, &upworkReadyField{Selector: "input[name=username]", X: 103, Y: 200}) {
		t.Fatal("expected a field that moved beyond tolerance to be unstable")
	}
}

func TestUpworkMouseClickEvents(t *testing.T) {
	got := upworkMouseClickEvents(120.5, 250.75)
	want := []map[string]any{
		{"type": "mouseMoved", "x": 120.5, "y": 250.75},
		{"type": "mousePressed", "x": 120.5, "y": 250.75, "button": "left", "clickCount": 1},
		{"type": "mouseReleased", "x": 120.5, "y": 250.75, "button": "left", "clickCount": 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mouse click events = %#v, want %#v", got, want)
	}
}
