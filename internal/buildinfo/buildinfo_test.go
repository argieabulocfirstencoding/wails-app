package buildinfo

import "testing"

func TestCurrentDefaultBuildDoesNotCheckDevelopmentUpdates(t *testing.T) {
	t.Setenv("ENABLE_DEV_UPDATE_CHECK", "")
	info := Current()
	if info.UpdatesEnabled {
		t.Fatal("a default build must not monitor the development release")
	}
}

func TestCurrentEnablesExplicitLocalDevelopmentChecks(t *testing.T) {
	t.Setenv("ENABLE_DEV_UPDATE_CHECK", "true")
	t.Setenv("APP_ENV", "development")
	t.Setenv("UPDATE_CHANNEL", "development")
	t.Setenv("UPDATE_BRANCH", "developer/jomary")
	info := Current()
	if !info.UpdatesEnabled || info.UpdateChannel != "development" || info.UpdateBranch != "developer/jomary" {
		t.Fatalf("expected explicit development update check, got %#v", info)
	}
}
