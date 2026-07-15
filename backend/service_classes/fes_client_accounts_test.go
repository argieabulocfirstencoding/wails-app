package service_classes

import (
	"errors"
	"strings"
	"testing"
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

func TestUpworkLoginFormPasteValuesUsesUsernameFirstThenPassword(t *testing.T) {
	first, second, err := upworkLoginFormPasteValues(&upworkCredentials{
		Username: "johnna-mae@example.com\n",
		Password: "super-secret-password\n",
	})
	if err != nil {
		t.Fatalf("expected login form paste values without error, got %v", err)
	}
	if first != "johnna-mae@example.com" {
		t.Fatalf("expected first login form paste value to be username, got %q", first)
	}
	if second != "super-secret-password" {
		t.Fatalf("expected second login form paste value to be password, got %q", second)
	}
}

func TestUpworkLoginFormPasteValuesRejectsIncompleteCredentials(t *testing.T) {
	_, _, err := upworkLoginFormPasteValues(&upworkCredentials{
		Password: "super-secret-password",
	})
	if err == nil {
		t.Fatalf("expected error for incomplete login form credentials")
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

func TestBuildUpworkFieldFocusExpressionIncludesSelectors(t *testing.T) {
	expression, err := buildUpworkFieldFocusExpression([]string{`input[name="username"]`, `input[type="password"]`})
	if err != nil {
		t.Fatalf("expected field focus expression without error, got %v", err)
	}
	if expression == "" {
		t.Fatalf("expected field focus expression to be non-empty")
	}
	if !strings.Contains(expression, `input[name=\"username\"]`) {
		t.Fatalf("expected expression to contain serialized username selector, got %q", expression)
	}
	if !strings.Contains(expression, `input[type=\"password\"]`) {
		t.Fatalf("expected expression to contain serialized password selector, got %q", expression)
	}
	if !strings.Contains(expression, `scrollIntoView`) {
		t.Fatalf("expected expression to scroll the field into view, got %q", expression)
	}
	if !strings.Contains(expression, `dispatchEvent(new MouseEvent`) {
		t.Fatalf("expected expression to dispatch mouse events before paste, got %q", expression)
	}
	if !strings.Contains(expression, `document.activeElement === field`) {
		t.Fatalf("expected expression to verify the focused field, got %q", expression)
	}
	if !strings.Contains(expression, `field.click()`) {
		t.Fatalf("expected expression to click the field, got %q", expression)
	}
}
