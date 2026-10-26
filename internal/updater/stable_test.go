package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fesapp/internal/buildinfo"
)

func stableBuild() buildinfo.Info {
	return buildinfo.Info{Version: "2.5", Environment: "production", UpdateChannel: "stable", UpdateBranch: "main", CommitSHA: "old", BuildNumber: 99, UpdatesEnabled: true}
}

func stableManifestFixture() Manifest {
	return Manifest{Channel: "stable", Branch: "main", Version: "2.5.1", RunNumber: 1, CommitSHA: "new", PublishedAt: time.Now().UTC(), InstallerAsset: stableInstallerName, SHA256Asset: stableInstallerName + ".sha256"}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(value any, status int) *http.Response {
	body, _ := json.Marshal(value)
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
}

// Exercise the complete release -> manifest -> selected installer flow without
// network access or launching an installer on the developer's machine.
func TestStableCheck(t *testing.T) {
	for _, name := range []string{"newer", "equal", "older", "tag mismatch", "prerelease", "draft", "wrong channel", "wrong branch", "missing manifest", "missing installer", "missing checksum", "wrong repository", "wrong asset", "wrong release", "404", "rate limited", "invalid version", "invalid installed version", "disabled"} {
		t.Run(name, func(t *testing.T) {
			build := stableBuild()
			manifest := stableManifestFixture()
			tag := "v2.5.1"
			status := http.StatusOK
			expectError := true
			expectUpdate := false
			switch name {
			case "newer":
				expectError, expectUpdate = false, true
			case "equal":
				build.Version, expectError = "2.5.1", false
			case "older":
				build.Version, expectError = "2.6.0", false
			case "tag mismatch":
				manifest.Version = "2.5.2"
			case "wrong channel":
				manifest.Channel = "development"
			case "wrong branch":
				manifest.Branch = "developer/jomary"
			case "404":
				status = http.StatusNotFound
			case "rate limited":
				status = http.StatusForbidden
			case "invalid version":
				manifest.Version, tag = "2.6.0-beta.1", "v2.6.0-beta.1"
			case "invalid installed version":
				build.Version = "unknown"
			case "disabled":
				build.UpdatesEnabled, expectError = false, false
			}
			base := "https://github.com/" + Repository + "/releases/download/" + tag + "/"
			r := release{TagName: tag, Draft: name == "draft", Prerelease: name == "prerelease"}
			for _, asset := range []string{stableManifest, stableInstallerName, stableInstallerName + ".sha256"} {
				if (name == "missing manifest" && asset == stableManifest) || (name == "missing installer" && asset == stableInstallerName) || (name == "missing checksum" && asset == stableInstallerName+".sha256") {
					continue
				}
				assetURL := base + asset
				if name == "wrong repository" {
					assetURL = strings.Replace(assetURL, Repository, developmentRepository, 1)
				}
				if name == "wrong asset" && asset == stableInstallerName {
					assetURL = base + "other.exe"
				}
				if name == "wrong release" && asset == stableInstallerName {
					assetURL = strings.Replace(assetURL, tag, "v2.0.0", 1)
				}
				r.Assets = append(r.Assets, releaseAsset{Name: asset, BrowserDownloadURL: assetURL})
			}
			s := New(build)
			requests := 0
			s.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests++
				switch request.URL.String() {
				case "https://api.github.com/repos/" + Repository + "/releases/latest":
					return jsonResponse(r, status), nil
				case base + stableManifest:
					return jsonResponse(manifest, http.StatusOK), nil
				default:
					t.Fatalf("unexpected request: %s", request.URL)
					return nil, fmt.Errorf("unexpected request")
				}
			})
			info, err := s.Check(context.Background())
			if (err != nil) != expectError || info.Available != expectUpdate {
				t.Fatalf("available=%v error=%v; want available=%v error=%v", info.Available, err, expectUpdate, expectError)
			}
			if name == "404" && !strings.Contains(err.Error(), Repository) {
				t.Fatalf("404 must identify the release repository: %v", err)
			}
			if name == "disabled" && requests != 0 {
				t.Fatal("disabled updates must not access the network")
			}
			if expectUpdate {
				cached, ok := s.latestUpdate()
				if !ok || cached.releaseTag != tag || cached.InstallerURL != base+stableInstallerName {
					t.Fatalf("wrong cached update: %#v", cached)
				}
			}
		})
	}
}

func TestDevelopmentCheckKeepsExistingReleaseSource(t *testing.T) {
	s := New(developmentBuild())
	base := "https://github.com/" + developmentRepository + "/releases/download/" + developmentReleaseTag + "/"
	r := release{TagName: developmentReleaseTag, Prerelease: true}
	for _, name := range []string{developmentManifest, expectedInstallerName, expectedInstallerName + ".sha256"} {
		r.Assets = append(r.Assets, releaseAsset{Name: name, BrowserDownloadURL: base + name})
	}
	s.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://api.github.com/repos/" + developmentRepository + "/releases/tags/" + developmentReleaseTag:
			return jsonResponse(r, http.StatusOK), nil
		case base + developmentManifest:
			return jsonResponse(validDevelopmentManifest(), http.StatusOK), nil
		default:
			t.Fatalf("unexpected development request: %s", request.URL)
			return nil, fmt.Errorf("unexpected request")
		}
	})
	info, err := s.Check(context.Background())
	if err != nil || !info.Available {
		t.Fatalf("development update broken: %#v %v", info, err)
	}
}

func TestStableAssetURLRejectsUntrustedLocations(t *testing.T) {
	valid := "https://github.com/" + Repository + "/releases/download/v2.5.1/" + stableInstallerName
	if err := validateReleaseAssetURL(valid, Repository, "v2.5.1", stableInstallerName); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		strings.Replace(valid, "https:", "http:", 1),
		strings.Replace(valid, "github.com", "github.com.evil.example", 1),
		strings.Replace(valid, "github.com", "github.com:444", 1),
		strings.Replace(valid, "github.com", "user@github.com", 1),
		strings.Replace(valid, Repository, developmentRepository, 1),
		strings.Replace(valid, "v2.5.1", "v2.5.0", 1),
		strings.Replace(valid, stableInstallerName, "../"+stableInstallerName, 1),
		valid + "?redirect=elsewhere", valid + "#fragment",
		"https://objects.githubusercontent.com/unrelated.exe",
	} {
		if err := validateReleaseAssetURL(raw, Repository, "v2.5.1", stableInstallerName); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestStableVersionOrdering(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"2.5", "2.5.1", true}, {"2.5.1", "2.5.1", false}, {"2.6.0", "2.5.1", false}, {"2.9.0", "2.10.0", true},
	} {
		build, manifest := stableBuild(), stableManifestFixture()
		build.Version, manifest.Version = tc.current, tc.latest
		// Stable versions do not depend on development workflow run numbers or SHAs.
		manifest.CommitSHA = build.CommitSHA
		got, err := isNewerBuild(build, manifest)
		if err != nil || got != tc.want {
			t.Fatalf("%s -> %s: %v %v", tc.current, tc.latest, got, err)
		}
	}
}

func TestInstallerDownloadRequiresMatchingChecksum(t *testing.T) {
	contents := "test installer bytes"
	hash := sha256.Sum256([]byte(contents))
	expected := hex.EncodeToString(hash[:])
	for _, tc := range []struct {
		name, checksum string
		wantError      bool
	}{
		{"matching", expected, false}, {"tampered", strings.Repeat("0", 64), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(stableBuild())
			s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, ".sha256") {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.checksum + "  " + stableInstallerName))}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: io.NopCloser(strings.NewReader(contents)), ContentLength: int64(len(contents))}, nil
			})
			checksum, err := s.fetchChecksum(context.Background(), "https://github.com/check.sha256", stableInstallerName)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), stableInstallerName)
			err = s.downloadInstaller(context.Background(), "https://github.com/installer", destination, checksum, nil)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected verification result: %v", err)
			}
			if !tc.wantError {
				got, err := os.ReadFile(destination)
				if err != nil || string(got) != contents {
					t.Fatalf("bad download: %s %v", got, err)
				}
			}
		})
	}
}
