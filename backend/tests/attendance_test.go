package services_test

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	service_classes "fesapp/backend/service_classes"
	"fesapp/backend/services"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, *models.User) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() {
		db.DB = originalDB
	})

	testUser := &models.User{
		ID:    "user-1",
		Email: "test@example.com",
	}
	if err := db.DB.Create(&testUser).Error; err != nil {
		t.Fatal(err)
	}

	return testDB, testUser
}

func TestLoginAttendance(t *testing.T) {
	db, testUser := setupTestDB(t)

	getAttendance := func() *models.Attendance {
		att, err := services.LoginAttendance(testUser.ID)
		assert.NoError(t, err)
		return att
	}

	attendance := getAttendance()
	assert.Equal(t, testUser.ID, attendance.UserID)
	assert.NotNil(t, attendance.TimeIn)
	assert.Nil(t, attendance.TimeOut, "Timeout should be nil for a new attendance")

	attendance_attempt_no_timeout := getAttendance()
	assert.Equal(t, attendance.ID, attendance_attempt_no_timeout.ID)

	db.Model(&attendance).Update("time_out", time.Now())

	attendance_attempt_with_timeout := getAttendance()
	assert.NotEqual(t, attendance.ID, attendance_attempt_with_timeout.ID)
}

// ================ Unit Testing ================

func TestOfflineLoginFlow(t *testing.T) {
	originalOfflineLogin := services.OfflineLogin
	originalLoginAttendance := services.LoginAttendance
	defer func() {
		services.OfflineLogin = originalOfflineLogin
		services.LoginAttendance = originalLoginAttendance
	}()

	// Replace OfflineLogin with a mock for testing
	services.OfflineLogin = func(email string) (*models.User, error) {
		return &models.User{
			ID:    "mock-id",
			Email: email,
		}, nil
	}

	// Replace LoginAttendance with a mock
	services.LoginAttendance = func(userId string) (*models.Attendance, error) {
		return &models.Attendance{
			Model:  gorm.Model{ID: 1},
			UserID: userId,
		}, nil
	}

	email := "test@example.com"
	offlineData, attendanceID, authUser, err := services.LoginUser(email)

	assert.NoError(t, err)
	assert.Equal(t, email, offlineData.AuthUser.Email)
	assert.Equal(t, uint(1), attendanceID.ID)
	assert.Equal(t, "mock-id", authUser.LocalID)
}

func TestFesOnlineLogin(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping online integration test. Set RUN_INTEGRATION_TESTS=1 to enable.")
	}

	authService := service_classes.NewAuthService()

	email := os.Getenv("FES_TEST_EMAIL")
	password := os.Getenv("FES_TEST_PASSWORD")
	if email == "" || password == "" {
		t.Skip("Skipping online integration test. Set FES_TEST_EMAIL and FES_TEST_PASSWORD.")
	}

	_, err := authService.AttemptLogin(email, password)

	assert.NoError(t, err)
}
