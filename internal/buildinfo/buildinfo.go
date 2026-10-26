// Package buildinfo contains values stamped into release builds with Go linker flags.
package buildinfo

import (
	"os"
	"strconv"
	"strings"
)

// These variables are intentionally variables, rather than constants, so the release
// workflow can set them with -ldflags. Do not put credentials or user data here.
var (
	Version       = "2.5"
	Environment   = "production"
	UpdateChannel = "stable"
	UpdateBranch  = "main"
	CommitSHA     = "local"
	BuildNumber   = "0"
)

// Info is the non-persistent build identity exposed to the desktop frontend.
type Info struct {
	Version        string `json:"version"`
	Environment    string `json:"environment"`
	UpdateChannel  string `json:"updateChannel"`
	UpdateBranch   string `json:"updateBranch"`
	CommitSHA      string `json:"commitSha"`
	BuildNumber    int    `json:"buildNumber"`
	UpdatesEnabled bool   `json:"updatesEnabled"`
}

// Current returns the compiled build identity. Builds default to stable updates;
// local development checks can opt in with ENABLE_DEV_UPDATE_CHECK=true.
func Current() Info {
	info := Info{
		Version:       strings.TrimSpace(Version),
		Environment:   strings.TrimSpace(Environment),
		UpdateChannel: strings.TrimSpace(UpdateChannel),
		UpdateBranch:  strings.TrimSpace(UpdateBranch),
		CommitSHA:     strings.TrimSpace(CommitSHA),
		BuildNumber:   parseBuildNumber(BuildNumber),
	}

	if isLocalDevelopmentCheckEnabled() {
		info.Environment = envOrDefault("APP_ENV", "development")
		info.UpdateChannel = envOrDefault("UPDATE_CHANNEL", "development")
		info.UpdateBranch = envOrDefault("UPDATE_BRANCH", "developer/jomary")
	}

	info.UpdatesEnabled = (info.UpdateChannel == "stable" && info.UpdateBranch == "main") ||
		(info.UpdateChannel == "development" && info.UpdateBranch == "developer/jomary")
	return info
}

func isLocalDevelopmentCheckEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_DEV_UPDATE_CHECK")), "true")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseBuildNumber(value string) int {
	buildNumber, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || buildNumber < 0 {
		return 0
	}
	return buildNumber
}
