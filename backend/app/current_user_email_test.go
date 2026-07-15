package app

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"testing"
)

func setupCurrentUserEmailTestDB(t *testing.T) {
	t.Helper()

	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatalf("create test database: %v", err)
	}

	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() {
		db.DB = originalDB
	})
}

func TestCurrentUserEmailUsesValidatedSessionEmail(t *testing.T) {
	setupCurrentUserEmailTestDB(t)
	if err := db.DB.Create(&models.User{ID: "active", Email: "stored@example.com"}).Error; err != nil {
		t.Fatal(err)
	}

	a := &App{AuthUser: &models.AuthUser{LocalID: "active", Email: "session@example.com"}}
	if got := a.currentUserEmail(); got != "session@example.com" {
		t.Fatalf("currentUserEmail() = %q, want session email", got)
	}
}

func TestCurrentUserEmailResolvesFESIDFromActiveUsersLocalRecord(t *testing.T) {
	setupCurrentUserEmailTestDB(t)
	if err := db.DB.Create(&models.User{ID: "active", Email: "employee@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.User{ID: "other", Email: "other@example.com"}).Error; err != nil {
		t.Fatal(err)
	}

	a := &App{AuthUser: &models.AuthUser{LocalID: "active", Email: "FES066"}}
	if got := a.currentUserEmail(); got != "employee@example.com" {
		t.Fatalf("currentUserEmail() = %q, want active user's stored email", got)
	}
	if a.AuthUser.Email != "employee@example.com" {
		t.Fatalf("AuthUser.Email = %q, want canonical employee email", a.AuthUser.Email)
	}
}

func TestCurrentUserEmailDoesNotUseAnotherCachedUser(t *testing.T) {
	setupCurrentUserEmailTestDB(t)
	if err := db.DB.Create(&models.User{ID: "other", Email: "other@example.com"}).Error; err != nil {
		t.Fatal(err)
	}

	a := &App{AuthUser: &models.AuthUser{LocalID: "active", Email: "FES066"}}
	if got := a.currentUserEmail(); got != "" {
		t.Fatalf("currentUserEmail() = %q, want empty", got)
	}
}
