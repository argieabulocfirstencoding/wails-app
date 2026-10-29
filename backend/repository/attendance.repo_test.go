package repository

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"testing"
	"time"
)

func TestCreateAttendanceResumesOnlyRecentOpenShift(t *testing.T) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = originalDB })

	user := &models.User{ID: "user-1", Email: "user@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	stale := &models.Attendance{UserID: user.ID, TimeIn: time.Now().Add(-resumableAttendanceWindow - time.Minute)}
	if err := testDB.Create(stale).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewAttendanceRepository()
	created, err := repo.CreateAttendance(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == stale.ID {
		t.Fatalf("CreateAttendance resumed stale attendance %d", stale.ID)
	}
	if created.TimeIn.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("new attendance time_in = %s, want current time", created.TimeIn)
	}
	closedAt := time.Now()
	if err := testDB.Model(created).Update("time_out", closedAt).Error; err != nil {
		t.Fatal(err)
	}

	recent := &models.Attendance{UserID: user.ID, TimeIn: time.Now().Add(-time.Hour)}
	if err := testDB.Create(recent).Error; err != nil {
		t.Fatal(err)
	}
	resumed, err := repo.CreateAttendance(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != recent.ID {
		t.Fatalf("CreateAttendance = %d, want recent attendance %d", resumed.ID, recent.ID)
	}
}
