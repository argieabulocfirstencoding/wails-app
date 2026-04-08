package service_classes_test // Using _test suffix is best practice

import (
	service_classes "fesapp/backend/service_classes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFesOnlineLogin(t *testing.T) {
	// 1. Initialize
	authService := service_classes.NewAuthService()

	// 2. Call the real API
	_, err := authService.AttemptLogin("johnpaultunacao.firstencoding@gmail.com", "012023choy")

	// 3. Assertions
	assert.NoError(t, err, "The API request should not return an error")

	// If your API returns data on success, verify it here
	if err == nil {
		t.Log("Login successful!")
	}
}
