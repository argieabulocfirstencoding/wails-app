package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"testing"
)

func TestTmpLogoutAttendanceMutatesStruct(t *testing.T) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	original := db.DB
	db.DB = testDB
	defer func() { db.DB = original }()

	user := &models.User{ID: "u1", Email: "u1@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	att := &models.Attendance{UserID: user.ID}
	if err := testDB.Create(att).Error; err != nil {
		t.Fatal(err)
	}
	if att.TimeOut != nil {
		t.Fatal("expected nil timeout before")
	}

	if err := LogoutAttendance(att); err != nil {
		t.Fatal(err)
	}
	if att.TimeOut == nil {
		t.Fatal("timeout not updated on struct")
	}
}
