package service_classes

import (
	"fesapp/backend/db/models"
	"testing"
)

func TestExtractEmployeeEmail(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		employee map[string]any
		want     string
	}{
		{
			name:     "top level email is canonical",
			response: map[string]any{"email": " employee@example.com "},
			employee: map[string]any{"Email": "other@example.com"},
			want:     "employee@example.com",
		},
		{
			name:     "employee email is used when response does not include one",
			response: map[string]any{},
			employee: map[string]any{"Email": "employee@example.com"},
			want:     "employee@example.com",
		},
		{
			name:     "account email address is supported",
			response: map[string]any{},
			employee: map[string]any{"Account email address": "employee@example.com"},
			want:     "employee@example.com",
		},
		{
			name:     "email field aliases are case and punctuation insensitive",
			response: map[string]any{"User_Email": "employee@example.com"},
			employee: map[string]any{},
			want:     "employee@example.com",
		},
		{
			name:     "FES ID is not an email",
			response: map[string]any{"email": "FES-12345"},
			employee: map[string]any{},
			want:     "",
		},
		{
			name:     "display name email values are rejected",
			response: map[string]any{"email": "Employee <employee@example.com>"},
			employee: map[string]any{},
			want:     "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := extractEmployeeEmail(test.response, test.employee); got != test.want {
				t.Fatalf("extractEmployeeEmail() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAuthenticatedEmailFromLoginResponse(t *testing.T) {
	tests := []struct {
		name       string
		response   map[string]any
		identifier string
		want       string
	}{
		{
			name:       "FES ID uses the email returned by the login endpoint",
			response:   map[string]any{"email": "employee@example.com"},
			identifier: "FES066",
			want:       "employee@example.com",
		},
		{
			name:       "email login uses the identifier when response omits email",
			response:   map[string]any{},
			identifier: "employee@example.com",
			want:       "employee@example.com",
		},
		{
			name:       "FES ID is never used as an email fallback",
			response:   map[string]any{},
			identifier: "FES066",
			want:       "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := authenticatedEmailFromLoginResponse(test.response, test.identifier); got != test.want {
				t.Fatalf("authenticatedEmailFromLoginResponse() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolvedAuthenticatedEmployeeEmail(t *testing.T) {
	tests := []struct {
		name string
		user *models.User
		want string
	}{
		{name: "canonical employee email", user: &models.User{Email: "employee@example.com"}, want: "employee@example.com"},
		{name: "FES ID produces no session email", user: &models.User{Email: "FES-12345"}, want: ""},
		{name: "missing user", user: nil, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvedAuthenticatedEmployeeEmail(test.user); got != test.want {
				t.Fatalf("resolvedAuthenticatedEmployeeEmail() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestHydrateEmployeeEmail(t *testing.T) {
	t.Run("replaces FES ID with canonical employee email", func(t *testing.T) {
		user := &models.User{Email: "FES-12345"}

		hydrateEmployeeEmail(
			user,
			map[string]any{},
			map[string]any{"Email": "employee@example.com"},
		)

		if user.Email != "employee@example.com" {
			t.Fatalf("user.Email = %q, want canonical employee email", user.Email)
		}
	})

	t.Run("clears FES ID when employee details have no valid email", func(t *testing.T) {
		user := &models.User{Email: "FES-12345"}

		hydrateEmployeeEmail(user, map[string]any{}, map[string]any{})

		if user.Email != "" {
			t.Fatalf("user.Email = %q, want empty", user.Email)
		}
	})

	t.Run("retains a valid direct-login email when response lacks one", func(t *testing.T) {
		user := &models.User{Email: "employee@example.com"}

		hydrateEmployeeEmail(user, map[string]any{}, map[string]any{})

		if user.Email != "employee@example.com" {
			t.Fatalf("user.Email = %q, want direct-login email", user.Email)
		}
	})
}
