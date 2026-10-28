package updater

import (
	"strings"
	"testing"
	"time"

	"fesapp/internal/buildinfo"
)

func developmentBuild() buildinfo.Info {
	return buildinfo.Info{
		Version:        "0.0.0-dev.18",
		Environment:    "development",
		UpdateChannel:  "development",
		UpdateBranch:   "developer/jomary",
		CommitSHA:      "current-commit",
		BuildNumber:    18,
		UpdatesEnabled: true,
	}
}

func validDevelopmentManifest() Manifest {
	return Manifest{
		Channel:        "development",
		Branch:         "developer/jomary",
		Version:        "0.0.0-dev.19",
		RunNumber:      19,
		CommitSHA:      "next-commit",
		PublishedAt:    time.Now().UTC(),
		InstallerAsset: expectedInstallerName,
		SHA256Asset:    expectedInstallerName + ".sha256",
	}
}

func TestValidateManifestAcceptsExpectedDevelopmentBuild(t *testing.T) {
	if err := validateManifest(developmentBuild(), validDevelopmentManifest()); err != nil {
		t.Fatalf("expected valid manifest, got %v", err)
	}
}

func TestValidateManifestRejectsOtherBranchAndAsset(t *testing.T) {
	manifest := validDevelopmentManifest()
	manifest.Branch = "main"
	if err := validateManifest(developmentBuild(), manifest); err == nil {
		t.Fatal("expected other branch to be rejected")
	}

	manifest = validDevelopmentManifest()
	manifest.InstallerAsset = "unexpected.exe"
	if err := validateManifest(developmentBuild(), manifest); err == nil {
		t.Fatal("expected unexpected installer to be rejected")
	}
}

func TestIsNewerDevelopmentBuildRequiresBothNewRunAndCommit(t *testing.T) {
	build := developmentBuild()
	manifest := validDevelopmentManifest()
	if !isNewerDevelopmentBuild(build, manifest) {
		t.Fatal("expected newer build to be available")
	}

	manifest.CommitSHA = build.CommitSHA
	if isNewerDevelopmentBuild(build, manifest) {
		t.Fatal("same commit must not be an update")
	}

	manifest = validDevelopmentManifest()
	manifest.RunNumber = build.BuildNumber
	if isNewerDevelopmentBuild(build, manifest) {
		t.Fatal("equal build number must not be an update")
	}
}

func TestValidateReleaseAssetURL(t *testing.T) {
	valid := "https://github.com/" + developmentRepository + "/releases/download/development-latest/development-manifest.json"
	if err := validateReleaseAssetURL(valid, developmentRepository, developmentReleaseTag, developmentManifest); err != nil {
		t.Fatalf("expected valid URL, got %v", err)
	}

	for _, raw := range []string{
		"http://github.com/FESTrainee01/wails-app/releases/download/development-latest/file.exe",
		"https://github.com/argieabulocfirstencoding/fes-app-tracker-new/releases/download/v1.0.0/file.exe",
		"https://example.com/file.exe",
	} {
		if err := validateReleaseAssetURL(raw, developmentRepository, developmentReleaseTag, developmentManifest); err == nil {
			t.Fatalf("expected URL %q to be rejected", raw)
		}
	}
}

func TestDevelopmentReleaseLookupErrorExplainsMissingOrPrivateRelease(t *testing.T) {
	err := developmentReleaseLookupError(404)
	if err == nil || !strings.Contains(err.Error(), "not published or is not publicly readable") {
		t.Fatalf("expected an actionable 404 error, got %v", err)
	}
}

func TestDelayedInstallerCommandWaitsForTheCurrentAppToExit(t *testing.T) {
	command := delayedInstallerCommand(`C:\Temp\FES App's Update.exe`, 4321)
	args := strings.Join(command.Args, " ")
	if command.Args[0] != "powershell.exe" {
		t.Fatalf("expected PowerShell helper, got %#v", command.Args)
	}
	if !strings.Contains(args, "Wait-Process -Id 4321") {
		t.Fatalf("expected helper to wait for the app process, got %q", args)
	}
	if !strings.Contains(args, "Start-Process -FilePath 'C:\\Temp\\FES App''s Update.exe' -ArgumentList '/S'") {
		t.Fatalf("expected safely quoted silent installer launch, got %q", args)
	}
}
