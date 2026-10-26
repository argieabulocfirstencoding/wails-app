# Publishing production updates

## Which repository receives what?

| Repository | Purpose |
| --- | --- |
| `FES-Paul/wails-app` | Application source, development branches, pull requests, version tags, and GitHub Actions |
| `argieabulocfirstencoding/fes-app-tracker-new` | Public production installers, checksums, and update manifests |

Developers push their code to **wails-app**. The release workflow publishes the
compiled application to **fes-app-tracker-new**. Do not change the source repository's
`origin` to the installer repository or push application source there.

The production release flow is:

```text
development -> main -> vMAJOR.MINOR.PATCH tag
    -> Build and Release workflow
    -> argieabulocfirstencoding/fes-app-tracker-new release
    -> installed stable app checks -> user chooses Install now
```

Merging into `main` does not publish by itself. The version tag is the explicit
release trigger. A release tag must point to a commit already merged into `main`.

## One-time setup by the repository owner

1. Ensure `argieabulocfirstencoding/fes-app-tracker-new` exists, is **public**, and has an initial
   commit on its default branch. Use the provided
   [`release-repository-readme.md`](./release-repository-readme.md) as its
   `README.md`. The desktop app downloads
   updates without a GitHub login. A private repository will return 404 to it.
2. Create a fine-grained GitHub personal access token with resource owner
   `argieabulocfirstencoding`, access only to `fes-app-tracker-new`, and **Contents: Read and write**.
   The token owner must have write access to that repository. Complete any
   organization approval required by GitHub and record the token expiry date.
3. In **FES-Paul/wails-app**, open **Settings -> Secrets and variables -> Actions**.
   Add a repository secret named **RELEASE_TOKEN** containing that token.
   The source repository's ordinary `GITHUB_TOKEN` cannot publish to the separate
   release repository. Never put the token in the app, source, or public assets.
4. Ensure GitHub Actions is enabled for the source repository and the updated
   `.github/workflows/build.yml`, updater code, and required local merge repairs
   are committed and merged into `main` before tagging the first release.
5. Keep the development release configuration separate. Its existing
   `UPDATES_REPO_TOKEN` and `argieabulocfirstencoding/fesapp-updates` destination
   still serve development builds. They are not used for stable publication.

## Each release: developer and maintainer steps

### 1. Finish and push the developer's work

Run commands in the **wails-app source checkout**. Start with a clean working tree
before switching branches; commit or stash your existing work intentionally.

```powershell
git remote -v
git switch development
git pull --ff-only origin development
```

Confirm `origin` points to `FES-Paul/wails-app`. Make and test the changes, then
stage the intended files (replace the example paths with the real files):

```powershell
git status --short
git add path/to/changed-file path/to/another-file
git diff --cached
git commit -m "Describe the completed change"
git push origin development
```

Pushing to `development` does not publish a production version. The separate
legacy **Development Desktop Update** workflow still listens to
`developer/jomary`; it is not required for this production release process.

### 2. Test development

Resolve any conflicts and confirm no conflict markers or duplicate declarations
remain. Test the development branch, including login, task
start/stop, offline records, synchronization, and shutdown.

### 3. Merge development into main

In `FES-Paul/wails-app`, open a pull request with:

- **base:** `main`
- **compare:** `development`

Review and merge it after testing. Then update your clean local main checkout:

```powershell
git switch main
git pull --ff-only origin main
go test ./backend/... ./internal/...
wails build
```

Wails generates the frontend bindings and runs the frontend TypeScript/Vite
build. Stop if either test/build command fails. Smoke-test the resulting app.

### 4. Tag the production version and push the tag

Use an unused version greater than the latest released version. The example
`v2.5.1` is only appropriate if it is still unused and newer than all published
versions. Use exactly `vMAJOR.MINOR.PATCH` with numeric components and no beta
suffix. The workflow stamps the version into the executable and Windows installer
metadata; you do not need to change version strings manually for tagged builds.

```powershell
git fetch origin --tags
git tag --list "v*" --sort=-version:refname
git tag -a v2.5.1 -m "Release 2.5.1"
git push origin v2.5.1
```

Push the tag to **wails-app**, not fes-app-tracker-new. Do not move or reuse published
version tags. The workflow creates the corresponding release in the asset
repository; the manifest records the actual source commit.

### 5. Wait for Build and Release to succeed

In **wails-app -> Actions -> Build and Release**, open the run for the tag.
It validates the version and main ancestry, tests the backend/updater, installs
the build tools, and creates the Windows amd64 installer.

The workflow uploads these files to a **draft** in fes-app-tracker-new, then publishes
the completed release and marks it Latest when it is newer than the current one:

- `FESApp-Setup.exe`
- `FESApp-Setup.exe.sha256`
- `stable-manifest.json`

If a run fails, read the failed step and correct the cause. An unfinished draft
can be retried by rerunning the failed workflow. Manual **Run workflow** requires
the existing source tag in its `tag` input; select `main` for the workflow branch.
Once published, the workflow refuses to overwrite that version. Code fixes need
a new commit merged into main and a new version tag.

### 6. Verify the public release and user update

Open [the production releases](https://github.com/argieabulocfirstencoding/fes-app-tracker-new/releases)
in a signed-out/private browser window. Check that all three assets download.
The [latest-release API](https://api.github.com/repos/argieabulocfirstencoding/fes-app-tracker-new/releases/latest)
must return the intended stable tag and those assets. The manifest's `version`
must match the tag without `v`, its channel must be `stable`, and branch `main`.

On a test machine with an older compatible stable build:

1. Finish any active tracking session before updating.
2. Open **Account -> Build & updates -> Check for updates**.
3. Confirm the offered version, then select **Install now**.
4. Wait for download/checksum verification. The app exits and starts the Windows
   installer. Approve Windows elevation if requested.
5. Reopen the app after installation if needed. Verify the version, existing
   data, login, and task tracking. Checking again must report no newer update.

The app also checks when opened and every two minutes while running and online.
The update is optional; users can select Later. They do not need Git or a GitHub
account. GitHub rate limits or network failures can temporarily prevent checks;
retry later through the account screen.

## First deployment of this updater

An already-installed binary cannot acquire this fix just because source code was
merged. Users of builds with updates disabled, a different release source, or
the development channel should manually download and install the first fixed
**FESApp-Setup.exe** from the production release. Close the app before installing.
Future higher stable versions will then be offered inside the app.

The updater never reads or modifies the local database. The installer updates
the application files; any data migrations belong to application startup.

## Troubleshooting and recovery

- **404:** confirm the exact repository exists, is public, and has a published
  non-prerelease. Drafts are not visible to the desktop updater.
- **Missing manifest/checksum:** publish through the updated workflow so all
  three assets are included; an installer-only release is insufficient.
- **No update offered:** check Account shows channel `stable`, branch `main`, and
  that the Latest release has a strictly higher version. Run numbers do not
  determine stable updates.
- **Publishing permission failure:** check `RELEASE_TOKEN` access, expiry, and
  organization approval in the source repository's Actions secrets.
- **Release already published:** choose a new higher version; never replace
  published installers in place.
- **Bad release:** stop offering the affected release and publish the corrected
  code with a higher version. The updater does not automatically downgrade users.
  If restoring older code, make a revert commit and release it under a new version.

## Implementation references

- `.github/workflows/build.yml`: tagged production build and publication
- `internal/buildinfo/buildinfo.go`: channel and compiled version identity
- `internal/updater/updater.go`: production repository, manifest and checksum checks
- `frontend/src/App.tsx`: startup and periodic checks

GitHub references: [creating releases](https://cli.github.com/manual/gh_release_create),
[publishing a draft](https://cli.github.com/manual/gh_release_edit), and
[release API](https://docs.github.com/en/rest/releases/releases#get-the-latest-release).
