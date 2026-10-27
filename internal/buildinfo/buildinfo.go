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
	Version       = "2.5.5"
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

// Current returns the compiled build identity. Release builds default to stable
// updates. Local development sessions disable update checks unless explicitly
// opted into development update testing.
func Current() Info {
	info := Info{
		Version:       strings.TrimSpace(Version),
		Environment:   strings.TrimSpace(Environment),
		UpdateChannel: strings.TrimSpace(UpdateChannel),
		UpdateBranch:  strings.TrimSpace(UpdateBranch),
		CommitSHA:     strings.TrimSpace(CommitSHA),
		BuildNumber:   parseBuildNumber(BuildNumber),
	}

	if isLocalDevelopmentSession() && !isLocalDevelopmentCheckEnabled() {
		info.UpdatesEnabled = false
		return info
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

// Keep this aligned with main.go, which uses DEVELOPMENT to separate local
// development data and desktop identity from a production installation.
func isLocalDevelopmentSession() bool {
	_, enabled := os.LookupEnv("DEVELOPMENT")
	return enabled
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
