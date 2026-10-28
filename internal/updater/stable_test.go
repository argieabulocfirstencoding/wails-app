package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	return Manifest{Channel: "stable", Branch: "main", Version: "2.5.1", RunNumber: 1, CommitSHA: "new", PublishedAt: time.Now().UTC(), InstallerAsset: stableInstallerName, SHA256Asset: stableInstallerName + ".sha256", InstallerSHA256: strings.Repeat("a", 64)}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(value any, status int) *http.Response {
	body, _ := json.Marshal(value)
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
}

func stableService(t *testing.T) (*Service, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	service := New(stableBuild())
	service.stablePublicKey = publicKey
	service.stableKeyError = nil
	return service, privateKey
}

func signedManifestResponse(t *testing.T, manifest Manifest, privateKey ed25519.PrivateKey, status int) *http.Response {
	t.Helper()
	if status != http.StatusOK {
		return jsonResponse(map[string]string{"message": "not found"}, status)
	}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	envelope := signedManifest{
		Payload:   base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload)),
	}
	return jsonResponse(envelope, http.StatusOK)
}

// Exercise the signed B2 manifest -> selected installer flow without network
// access or launching an installer on the developer's machine.
func TestStableCheck(t *testing.T) {
	for _, name := range []string{"newer", "equal", "older", "wrong channel", "wrong branch", "wrong asset", "404", "invalid signature", "invalid version", "invalid checksum", "invalid installed version", "disabled"} {
		t.Run(name, func(t *testing.T) {
			build := stableBuild()
			manifest := stableManifestFixture()
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
			case "wrong channel":
				manifest.Channel = "development"
			case "wrong branch":
				manifest.Branch = "developer/jomary"
			case "wrong asset":
				manifest.InstallerAsset = "other.exe"
			case "404":
				status = http.StatusNotFound
			case "invalid version":
				manifest.Version = "2.6.0-beta.1"
			case "invalid checksum":
				manifest.InstallerSHA256 = "not-a-checksum"
			case "invalid installed version":
				build.Version = "unknown"
			case "disabled":
				build.UpdatesEnabled, expectError, expectUpdate = false, false, false
			}

			s, privateKey := stableService(t)
			s.build = build
			if name == "invalid signature" {
				_, privateKey, _ = ed25519.GenerateKey(rand.Reader)
			}
			requests := 0
			s.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests++
				if request.URL.String() != stableLatestManifestURL() {
					t.Fatalf("unexpected request: %s", request.URL)
				}
				return signedManifestResponse(t, manifest, privateKey, status), nil
			})
			info, err := s.Check(context.Background())
			if (err != nil) != expectError || info.Available != expectUpdate {
				t.Fatalf("available=%v error=%v; want available=%v error=%v", info.Available, err, expectUpdate, expectError)
			}
			if name == "disabled" && requests != 0 {
				t.Fatal("disabled updates must not access the network")
			}
			if expectUpdate {
				cached, ok := s.latestUpdate()
				if !ok || cached.releaseTag != manifest.Version || cached.InstallerURL != stableAssetURL(manifest.Version, stableInstallerName) || cached.expectedSHA256 != manifest.InstallerSHA256 {
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
	valid := stableAssetURL("2.5.1", stableInstallerName)
	if err := validateStableAssetURL(valid, "2.5.1", stableInstallerName); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		strings.Replace(valid, "https:", "http:", 1),
		strings.Replace(valid, "fesapp-updates.s3.us-east-005.backblazeb2.com", "fesapp-updates.s3.us-east-005.backblazeb2.com.evil.example", 1),
		strings.Replace(valid, "https://", "https://user@", 1),
		strings.Replace(valid, "/2.5.1/", "/2.5.0/", 1),
		strings.Replace(valid, stableInstallerName, "../"+stableInstallerName, 1),
		valid + "?redirect=elsewhere", valid + "#fragment",
	} {
		if err := validateStableAssetURL(raw, "2.5.1", stableInstallerName); err == nil {
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
			s, _ := stableService(t)
			s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, ".sha256") {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.checksum + "  " + stableInstallerName))}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: io.NopCloser(strings.NewReader(contents)), ContentLength: int64(len(contents))}, nil
			})
			checksum, err := s.fetchChecksum(context.Background(), stableAssetURL("2.5.1", stableInstallerName+".sha256"), stableInstallerName)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), stableInstallerName)
			err = s.downloadInstaller(context.Background(), stableAssetURL("2.5.1", stableInstallerName), destination, checksum, nil)
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
