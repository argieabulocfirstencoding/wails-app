// Package updater implements the development-channel update flow. It intentionally
// keeps all state in memory and never accesses the application's database.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fesapp/internal/buildinfo"
)

const (
	Repository            = "argieabulocfirstencoding/fesapp-updates"
	developmentReleaseTag = "development-latest"
	developmentManifest   = "development-manifest.json"
	expectedInstallerName = "FESApp-Development-Setup.exe"
	maxManifestSize       = 128 * 1024
	maxChecksumSize       = 16 * 1024
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// Manifest is the small, signed-by-release-process document that describes a
// development build. Its asset URLs are resolved from the trusted release API.
type Manifest struct {
	Channel        string    `json:"channel"`
	Branch         string    `json:"branch"`
	Version        string    `json:"version"`
	RunNumber      int       `json:"runNumber"`
	CommitSHA      string    `json:"commitSha"`
	PublishedAt    time.Time `json:"publishedAt"`
	Mandatory      bool      `json:"mandatory"`
	InstallerAsset string    `json:"installerAsset"`
	SHA256Asset    string    `json:"sha256Asset"`
}

// UpdateInfo is returned to Wails and retained only for the current process.
type UpdateInfo struct {
	Available      bool   `json:"available"`
	Channel        string `json:"channel"`
	Branch         string `json:"branch"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	CurrentCommit  string `json:"currentCommit"`
	LatestCommit   string `json:"latestCommit"`
	InstallerURL   string `json:"installerUrl"`
	ChecksumURL    string `json:"checksumUrl"`
}

type Service struct {
	build  buildinfo.Info
	client *http.Client

	mu     sync.Mutex
	latest *UpdateInfo
}

func New(build buildinfo.Info) *Service {
	return &Service{build: build, client: trustedHTTPClient(3 * time.Minute)}
}

func (s *Service) BuildInfo() buildinfo.Info {
	return s.build
}

func (s *Service) Check(ctx context.Context) (UpdateInfo, error) {
	result := s.currentUpdateInfo()
	if !s.build.UpdatesEnabled {
		s.clearLatest()
		return result, nil
	}

	release, err := s.fetchDevelopmentRelease(ctx)
	if err != nil {
		return result, err
	}

	manifestAsset, ok := release.asset(developmentManifest)
	if !ok {
		return result, errors.New("development manifest is missing from the release")
	}
	if err := validateReleaseAssetURL(manifestAsset.BrowserDownloadURL); err != nil {
		return result, fmt.Errorf("development manifest URL rejected: %w", err)
	}

	manifest, err := s.fetchManifest(ctx, manifestAsset.BrowserDownloadURL)
	if err != nil {
		return result, err
	}
	if err := validateManifest(s.build, manifest); err != nil {
		return result, err
	}
	if !isNewerDevelopmentBuild(s.build, manifest) {
		s.clearLatest()
		return result, nil
	}

	installer, ok := release.asset(manifest.InstallerAsset)
	if !ok {
		return result, errors.New("development installer is missing from the release")
	}
	checksum, ok := release.asset(manifest.SHA256Asset)
	if !ok {
		return result, errors.New("development checksum is missing from the release")
	}
	if err := validateReleaseAssetURL(installer.BrowserDownloadURL); err != nil {
		return result, fmt.Errorf("installer URL rejected: %w", err)
	}
	if err := validateReleaseAssetURL(checksum.BrowserDownloadURL); err != nil {
		return result, fmt.Errorf("checksum URL rejected: %w", err)
	}

	result.Available = true
	result.LatestVersion = manifest.Version
	result.LatestCommit = manifest.CommitSHA
	result.InstallerURL = installer.BrowserDownloadURL
	result.ChecksumURL = checksum.BrowserDownloadURL
	s.setLatest(result)
	return result, nil
}

// Install downloads only the installer selected from the manifest just checked,
// verifies its SHA-256 file, and starts a helper that waits for Wails to exit
// before launching NSIS. This avoids asking NSIS to overwrite a running executable.
func (s *Service) Install(ctx context.Context, progress func(int)) error {
	if runtime.GOOS != "windows" {
		return errors.New("desktop installation is only supported on Windows")
	}

	update, ok := s.latestUpdate()
	if !ok || !update.Available {
		return errors.New("check for an update before installing")
	}
	installerName := filepath.Base(expectedInstallerName)
	if err := validateReleaseAssetURL(update.InstallerURL); err != nil {
		return fmt.Errorf("installer URL rejected: %w", err)
	}
	if err := validateReleaseAssetURL(update.ChecksumURL); err != nil {
		return fmt.Errorf("checksum URL rejected: %w", err)
	}

	expectedChecksum, err := s.fetchChecksum(ctx, update.ChecksumURL, installerName)
	if err != nil {
		return err
	}

	directory, err := os.MkdirTemp("", "fesapp-update-")
	if err != nil {
		return err
	}
	installerPath := filepath.Join(directory, installerName)
	if err := s.downloadInstaller(ctx, update.InstallerURL, installerPath, expectedChecksum, progress); err != nil {
		return err
	}

	if progress != nil {
		progress(100)
	}
	return delayedInstallerCommand(installerPath, os.Getpid()).Start()
}

func delayedInstallerCommand(installerPath string, parentPID int) *exec.Cmd {
	script := fmt.Sprintf(
		"$parent = Get-Process -Id %d -ErrorAction SilentlyContinue; if ($null -ne $parent) { Wait-Process -Id %d -ErrorAction SilentlyContinue }; Start-Process -FilePath %s -ArgumentList '/S'",
		parentPID,
		parentPID,
		powerShellSingleQuoted(installerPath),
	)
	return exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
}

func powerShellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (s *Service) currentUpdateInfo() UpdateInfo {
	return UpdateInfo{
		Channel:        s.build.UpdateChannel,
		Branch:         s.build.UpdateBranch,
		CurrentVersion: s.build.Version,
		CurrentCommit:  s.build.CommitSHA,
	}
}

func (s *Service) fetchDevelopmentRelease(ctx context.Context) (*release, error) {
	releaseURL := "https://api.github.com/repos/" + Repository + "/releases/tags/" + developmentReleaseTag
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, developmentReleaseLookupError(response.StatusCode)
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/json") {
		return nil, errors.New("development release returned an unexpected content type")
	}

	var value release
	if err := json.NewDecoder(io.LimitReader(response.Body, maxManifestSize)).Decode(&value); err != nil {
		return nil, fmt.Errorf("decode development release: %w", err)
	}
	if value.TagName != developmentReleaseTag {
		return nil, errors.New("development release tag did not match")
	}
	return &value, nil
}

func developmentReleaseLookupError(statusCode int) error {
	if statusCode == http.StatusNotFound {
		return errors.New("development update release is not published or is not publicly readable (GitHub returned 404); publish the development-latest prerelease and its assets")
	}
	return fmt.Errorf("development release lookup failed with status %d", statusCode)
}

func (s *Service) fetchManifest(ctx context.Context, manifestURL string) (Manifest, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return Manifest{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("manifest download failed with status %d", response.StatusCode)
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "application/octet-stream") {
		return Manifest{}, errors.New("manifest returned an unexpected content type")
	}

	var manifest Manifest
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxManifestSize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode development manifest: %w", err)
	}
	return manifest, nil
}

func (s *Service) fetchChecksum(ctx context.Context, checksumURL, installerName string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download failed with status %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumSize))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != installerName {
			continue
		}
		checksum := strings.ToLower(fields[0])
		if len(checksum) != sha256.Size*2 {
			return "", errors.New("checksum has an invalid length")
		}
		if _, err := hex.DecodeString(checksum); err != nil {
			return "", errors.New("checksum is not hexadecimal")
		}
		return checksum, nil
	}
	return "", errors.New("checksum does not contain the development installer")
}

func (s *Service) downloadInstaller(ctx context.Context, installerURL, destination, expectedChecksum string, progress func(int)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, installerURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/octet-stream")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("installer download failed with status %d", response.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Type")))
	if contentType == "" || strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/json") {
		return errors.New("installer returned an unexpected content type")
	}

	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, 64*1024)
	var written int64
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			if _, err := file.Write(buffer[:read]); err != nil {
				return err
			}
			if _, err := hash.Write(buffer[:read]); err != nil {
				return err
			}
			written += int64(read)
			if progress != nil && response.ContentLength > 0 {
				progress(int(written * 100 / response.ContentLength))
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if written == 0 {
		return errors.New("installer download was empty")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedChecksum) {
		return errors.New("installer checksum mismatch")
	}
	return nil
}

func validateManifest(build buildinfo.Info, manifest Manifest) error {
	if manifest.Channel != "development" || build.UpdateChannel != "development" {
		return errors.New("manifest channel did not match the development build")
	}
	if manifest.Branch != build.UpdateBranch || manifest.Branch != "developer/jomary" {
		return errors.New("manifest branch did not match the development build")
	}
	if strings.TrimSpace(manifest.Version) == "" || manifest.RunNumber < 1 || strings.TrimSpace(manifest.CommitSHA) == "" {
		return errors.New("manifest has incomplete build identity")
	}
	if manifest.PublishedAt.IsZero() {
		return errors.New("manifest does not contain a publication time")
	}
	if manifest.InstallerAsset != expectedInstallerName || manifest.SHA256Asset != expectedInstallerName+".sha256" {
		return errors.New("manifest contains an unexpected installer asset")
	}
	return nil
}

func isNewerDevelopmentBuild(build buildinfo.Info, manifest Manifest) bool {
	return manifest.CommitSHA != build.CommitSHA && manifest.RunNumber > build.BuildNumber
}

func (r *release) asset(name string) (releaseAsset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func (s *Service) setLatest(update UpdateInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = &update
}

func (s *Service) clearLatest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = nil
}

func (s *Service) latestUpdate() (UpdateInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.latest == nil {
		return UpdateInfo{}, false
	}
	return *s.latest, true
}

func trustedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			return validateRedirectURL(request.URL)
		},
	}
}

func validateReleaseAssetURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !isTrustedReleaseHost(parsed.Hostname()) {
		return errors.New("URL is not an HTTPS GitHub release asset")
	}
	if strings.EqualFold(parsed.Hostname(), "github.com") {
		expectedPath := "/" + Repository + "/releases/download/" + developmentReleaseTag + "/"
		if !strings.HasPrefix(path.Clean(parsed.Path)+"/", expectedPath) {
			return errors.New("URL is not in the expected development release")
		}
	}
	return nil
}

func validateRedirectURL(parsed *url.URL) error {
	if !strings.EqualFold(parsed.Scheme, "https") || !isTrustedReleaseHost(parsed.Hostname()) {
		return errors.New("redirect left trusted HTTPS GitHub hosts")
	}
	return nil
}

func isTrustedReleaseHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}
