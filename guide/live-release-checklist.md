# Development to live-app release checklist

Use this checklist whenever a tested change needs to reach users of FES App
Tracker.

```text
development -> main -> vMAJOR.MINOR.PATCH tag -> GitHub Actions
    -> public release -> user installs the update
```

## Repositories and one-time setup

| Repository | Purpose |
| --- | --- |
| Source repository | Developers push code here. The current workspace uses `FES-Paul/wails-app`. |
| `argieabulocfirstencoding/fes-app-tracker-new` | Public installer, checksum, and update-manifest releases. |

Before the first live release, confirm all of the following:

- The source repository contains [`.github/workflows/build.yml`](../.github/workflows/build.yml).
- `argieabulocfirstencoding/fes-app-tracker-new` is public and has a `main` branch.
- The source repository has a GitHub Actions repository secret named
  `RELEASE_TOKEN`.
- The token belongs to an account with write access to
  `argieabulocfirstencoding/fes-app-tracker-new` and has **Contents: Read and
  write** permission for that repository.

The secret belongs in the **source repository**, not in the public release
repository. GitHub does not reveal an existing secret value; replace it if the
workflow reports `HTTP 401: Bad credentials`.

## 1. Push the finished work to development

Start in the source-project folder. Commit or stash unrelated local work before
switching branches.

```powershell
git switch development
git pull --ff-only origin development
go test ./backend/... ./internal/...
```

Make the change and test it. Stage only the intended files, review them, then
commit and push:

```powershell
git status --short
git add path/to/changed-file path/to/another-file
git diff --cached
git commit -m "Describe the completed change"
git push origin development
```

Pushing to `development` does not publish a user update.

## 2. Test and merge development into main

Verify the development branch, including the affected screen or workflow.
For changes that affect normal app usage, test login, task tracking, sync, and
shutdown as applicable.

On GitHub, create a pull request with:

```text
base: main
compare: development
```

Review it, resolve conflicts, and merge only when it is ready for users.

## 3. Verify main before release

After the pull request is merged:

```powershell
git switch main
git pull --ff-only origin main
go test ./backend/... ./internal/...
wails build
```

Do not create a release tag if these checks fail. The tag represents the exact
source commit users will receive.

## 4. Create and push a new release tag

The release workflow runs only when you push a new tag in this exact format:

```text
vMAJOR.MINOR.PATCH
```

Examples: `v2.5.5`, `v2.6.0`, and `v3.0.0`.

Do not use a fourth number (`v2.5.4.1`), a suffix (`v2.5.5-beta`), or an
existing tag. First inspect existing tags and select a new version higher than
the latest published release:

```powershell
git fetch origin --tags
git tag --list "v*" --sort=-version:refname
git ls-remote --tags origin refs/tags/v2.5.5
```

If the final command prints nothing, create and push the chosen tag. Replace
`v2.5.5` below with the version selected for this release.

```powershell
git tag -a v2.5.5 -m "Release 2.5.5"
git push origin v2.5.5
```

Never move a published tag or overwrite a published release. Fixes require a
new commit and a higher version tag.

## 5. Confirm GitHub Actions publishes the release

Open the source repository's **Actions** tab and select **Build and Release**.
The workflow validates the tag, confirms it belongs to `main`, builds the Windows
installer, uploads assets to a draft, then publishes the completed release to
`argieabulocfirstencoding/fes-app-tracker-new`.

The release must contain all three files:

- `FESApp-Setup.exe`
- `FESApp-Setup.exe.sha256`
- `stable-manifest.json`

If the final publishing step reports `HTTP 401: Bad credentials`, recreate the
token with the required release-repository access and update `RELEASE_TOKEN` in
the source repository before rerunning the failed job.

## 6. Verify the user update

Open [the public Releases page](https://github.com/argieabulocfirstencoding/fes-app-tracker-new/releases)
in a private browser window. Confirm the new release is published, not a draft
or prerelease, and has all three assets.

On a test computer with an older updater-enabled stable build:

1. Finish any active task.
2. Open **Account -> Build & updates**.
3. Select **Check for updates**.
4. Confirm the new version and select **Install now**.
5. Allow the application to close and the installer to run.
6. Reopen the app and confirm the installed version and normal operation.

The app also checks when opened and every two minutes while it runs. Users with
an old build that lacks the new updater must manually install the first release
from the public Releases page. Later releases can then be installed in the app.

## More detail

For release repository setup, version metadata, rollback guidance, and deeper
troubleshooting, see [stable-release-workflow.md](./stable-release-workflow.md).
