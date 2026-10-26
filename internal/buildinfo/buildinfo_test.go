package buildinfo

import "testing"

func TestCurrentDefaultBuildChecksStableUpdates(t *testing.T) {
	t.Setenv("ENABLE_DEV_UPDATE_CHECK", "")
	info := Current()
	if !info.UpdatesEnabled || info.UpdateChannel != "stable" || info.UpdateBranch != "main" {
		t.Fatalf("a default build must monitor stable releases from main, got %#v", info)
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

func TestCurrentDisablesUnsupportedChannelsAndBranches(t *testing.T) {
	for _, tc := range []struct{ channel, branch string }{
		{"beta", "main"}, {"stable", "development"}, {"development", "main"},
	} {
		t.Run(tc.channel+"/"+tc.branch, func(t *testing.T) {
			t.Setenv("ENABLE_DEV_UPDATE_CHECK", "true")
			t.Setenv("UPDATE_CHANNEL", tc.channel)
			t.Setenv("UPDATE_BRANCH", tc.branch)
			if Current().UpdatesEnabled {
				t.Fatal("unsupported channel/branch must not check updates")
			}
		})
	}
}
