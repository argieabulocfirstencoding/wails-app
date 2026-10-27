// Package updater implements stable and development update flows. It intentionally
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
	"strconv"
	"strings"
	"sync"
	"time"

	"fesapp/internal/buildinfo"

	"github.com/blang/semver"
)

const (
	Repository            = "argieabulocfirstencoding/fes-app-tracker-new"
	developmentRepository = "argieabulocfirstencoding/fesapp-updates"
	developmentReleaseTag = "development-latest"
	developmentManifest   = "development-manifest.json"
	expectedInstallerName = "FESApp-Development-Setup.exe"
	stableManifest        = "stable-manifest.json"
	stableInstallerName   = "FESApp-Setup.exe"
	maxManifestSize       = 128 * 1024
	maxChecksumSize       = 16 * 1024
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type release struct {
	TagName    string         `json:"tag_name"`
	Assets     []releaseAsset `json:"assets"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
}

// Manifest describes a build published by the release workflow. It is not a
// digital signature; its asset URLs are resolved from the trusted release API.
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
	releaseTag     string
}

type channelConfig struct {
	repository  string
	branch      string
	manifest    string
	installer   string
	releasePath string
}

func configFor(build buildinfo.Info) (channelConfig, error) {
	switch {
	case build.UpdateChannel == "stable" && build.UpdateBranch == "main":
		return channelConfig{Repository, "main", stableManifest, stableInstallerName, "latest"}, nil
	case build.UpdateChannel == "development" && build.UpdateBranch == "developer/jomary":
		return channelConfig{developmentRepository, "developer/jomary", developmentManifest, expectedInstallerName, "tags/" + developmentReleaseTag}, nil
	default:
		return channelConfig{}, errors.New("unsupported update channel or branch")
	}
}

type Service struct {
	build  buildinfo.Info
	client *http.Client

	mu               sync.Mutex
	latest           *UpdateInfo
	rateLimitedUntil time.Time
}

type githubRateLimitError struct {
	until time.Time
}

func (e *githubRateLimitError) Error() string {
	return fmt.Sprintf("GitHub update checks are temporarily rate-limited; try again after %s.", e.until.Local().Format("Jan 2, 3:04 PM"))
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
	if err := s.rateLimitError(time.Now()); err != nil {
		return result, err
	}

	config, err := configFor(s.build)
	if err != nil {
		return result, err
	}
	release, err := s.fetchRelease(ctx, config)
	if err != nil {
		var rateLimitErr *githubRateLimitError
		if errors.As(err, &rateLimitErr) {
			s.setRateLimitedUntil(rateLimitErr.until)
		}
		return result, err
	}

	manifestAsset, ok := release.asset(config.manifest)
	if !ok {
		return result, fmt.Errorf("%s is missing from the release; publish it with the installer and checksum", config.manifest)
	}
	if err := validateReleaseAssetURL(manifestAsset.BrowserDownloadURL, config.repository, release.TagName, config.manifest); err != nil {
		return result, fmt.Errorf("manifest URL rejected: %w", err)
	}

	manifest, err := s.fetchManifest(ctx, manifestAsset.BrowserDownloadURL)
	if err != nil {
		return result, err
	}
	if err := validateManifest(s.build, manifest); err != nil {
		return result, err
	}
	if s.build.UpdateChannel == "stable" && release.TagName != "v"+manifest.Version {
		return result, errors.New("stable manifest version does not match the release tag")
	}
	newer, err := isNewerBuild(s.build, manifest)
	if err != nil {
		return result, err
	}
	if !newer {
		s.clearLatest()
		return result, nil
	}

	installer, ok := release.asset(manifest.InstallerAsset)
	if !ok {
		return result, errors.New("installer is missing from the release")
	}
	checksum, ok := release.asset(manifest.SHA256Asset)
	if !ok {
		return result, errors.New("checksum is missing from the release")
	}
	if err := validateReleaseAssetURL(installer.BrowserDownloadURL, config.repository, release.TagName, config.installer); err != nil {
		return result, fmt.Errorf("installer URL rejected: %w", err)
	}
	if err := validateReleaseAssetURL(checksum.BrowserDownloadURL, config.repository, release.TagName, config.installer+".sha256"); err != nil {
		return result, fmt.Errorf("checksum URL rejected: %w", err)
	}

	result.Available = true
	result.LatestVersion = manifest.Version
	result.LatestCommit = manifest.CommitSHA
	result.InstallerURL = installer.BrowserDownloadURL
	result.ChecksumURL = checksum.BrowserDownloadURL
	result.releaseTag = release.TagName
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
	config, err := configFor(s.build)
	if err != nil {
		return err
	}
	installerName := config.installer
	if err := validateReleaseAssetURL(update.InstallerURL, config.repository, update.releaseTag, installerName); err != nil {
		return fmt.Errorf("installer URL rejected: %w", err)
	}
	if err := validateReleaseAssetURL(update.ChecksumURL, config.repository, update.releaseTag, installerName+".sha256"); err != nil {
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

func (s *Service) fetchRelease(ctx context.Context, config channelConfig) (*release, error) {
	releaseURL := "https://api.github.com/repos/" + config.repository + "/releases/" + config.releasePath
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
		if response.StatusCode == http.StatusForbidden {
			if rateLimitErr := githubRateLimitErrorFromHeaders(response.Header, time.Now()); rateLimitErr != nil {
				return nil, rateLimitErr
			}
		}
		if s.build.UpdateChannel == "development" {
			return nil, developmentReleaseLookupError(response.StatusCode)
		}
		if response.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("stable update release is not published or is not publicly readable (GitHub returned 404); publish a stable release and its assets in %s", config.repository)
		}
		return nil, fmt.Errorf("release lookup failed with status %d", response.StatusCode)
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "application/json") {
		return nil, errors.New("release returned an unexpected content type")
	}

	var value release
	if err := json.NewDecoder(io.LimitReader(response.Body, maxManifestSize)).Decode(&value); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	if value.Draft {
		return nil, errors.New("draft releases cannot be installed")
	}
	if s.build.UpdateChannel == "stable" {
		if value.Prerelease {
			return nil, errors.New("prereleases cannot be installed on the stable channel")
		}
		if !strings.HasPrefix(value.TagName, "v") {
			return nil, errors.New("stable release tag must be vMAJOR.MINOR.PATCH")
		}
		if _, err := parseStableVersion(strings.TrimPrefix(value.TagName, "v")); err != nil {
			return nil, err
		}
	} else if value.TagName != developmentReleaseTag {
		return nil, errors.New("development release tag did not match")
	}
	return &value, nil
}

func githubRateLimitErrorFromHeaders(headers http.Header, now time.Time) *githubRateLimitError {
	if strings.TrimSpace(headers.Get("X-RateLimit-Remaining")) != "0" {
		return nil
	}
	resetSeconds, err := strconv.ParseInt(strings.TrimSpace(headers.Get("X-RateLimit-Reset")), 10, 64)
	if err != nil {
		return nil
	}
	until := time.Unix(resetSeconds, 0)
	if !until.After(now) {
		return nil
	}
	return &githubRateLimitError{until: until}
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
		return Manifest{}, fmt.Errorf("decode update manifest: %w", err)
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
	config, err := configFor(build)
	if err != nil {
		return err
	}
	if manifest.Channel != build.UpdateChannel {
		return errors.New("manifest channel did not match the installed build")
	}
	if manifest.Branch != config.branch {
		return errors.New("manifest branch did not match the installed build")
	}
	if strings.TrimSpace(manifest.Version) == "" || manifest.RunNumber < 1 || strings.TrimSpace(manifest.CommitSHA) == "" {
		return errors.New("manifest has incomplete build identity")
	}
	if manifest.PublishedAt.IsZero() {
		return errors.New("manifest does not contain a publication time")
	}
	if manifest.InstallerAsset != config.installer || manifest.SHA256Asset != config.installer+".sha256" {
		return errors.New("manifest contains an unexpected installer asset")
	}
	if build.UpdateChannel == "stable" {
		_, err := parseStableVersion(manifest.Version)
		return err
	}
	return nil
}

func parseStableVersion(value string) (semver.Version, error) {
	version, err := semver.Parse(value)
	if err != nil || len(version.Pre) != 0 || len(version.Build) != 0 {
		return semver.Version{}, fmt.Errorf("invalid stable version %q; expected MAJOR.MINOR.PATCH", value)
	}
	return version, nil
}

func isNewerBuild(build buildinfo.Info, manifest Manifest) (bool, error) {
	if build.UpdateChannel == "development" {
		return isNewerDevelopmentBuild(build, manifest), nil
	}
	// Existing installations used two-part versions such as 2.5.
	currentVersion := strings.TrimPrefix(strings.TrimSpace(build.Version), "v")
	if strings.Count(currentVersion, ".") == 1 {
		currentVersion += ".0"
	}
	current, err := semver.Parse(currentVersion)
	if err != nil {
		return false, fmt.Errorf("invalid installed version: %w", err)
	}
	latest, err := parseStableVersion(manifest.Version)
	if err != nil {
		return false, err
	}
	return latest.GT(current), nil
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

func (s *Service) rateLimitError(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rateLimitedUntil.IsZero() {
		return nil
	}
	if !now.Before(s.rateLimitedUntil) {
		s.rateLimitedUntil = time.Time{}
		return nil
	}
	return &githubRateLimitError{until: s.rateLimitedUntil}
}

func (s *Service) setRateLimitedUntil(until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if until.After(s.rateLimitedUntil) {
		s.rateLimitedUntil = until
	}
}

func trustedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			return validateRedirectURL(request.URL)
		},
	}
}

func validateReleaseAssetURL(raw, repository, tag, asset string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("URL is not an HTTPS GitHub release asset")
	}
	expectedPath := "/" + repository + "/releases/download/" + tag + "/" + asset
	if tag == "" || parsed.Path != expectedPath || path.Clean(parsed.Path) != expectedPath {
		return errors.New("URL is not the expected asset in the selected release")
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
