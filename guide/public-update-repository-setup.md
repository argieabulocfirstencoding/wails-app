# Private Source Repository with a Public Update Repository

This guide applies to the **development channel**. For production updates from
`main` using `argieabulocfirstencoding/fes-app-tracker-new`, follow
[`stable-release-workflow.md`](./stable-release-workflow.md).

This guide keeps the application source repository private while allowing the
desktop app to download development updates without a GitHub login.

## What this creates

| Repository                                | Visibility | Purpose                                       |
| ----------------------------------------- | ---------- | --------------------------------------------- |
| `FESTrainee01/wails-app`                  | Private    | Source code and build workflow                |
| `argieabulocfirstencoding/fesapp-updates` | Public     | Installer, checksum, and update manifest only |

The desktop app downloads only the public release assets. It never contains a
GitHub token.

## 1. Create the public update repository

1. On GitHub, select **New repository**.
2. Set the owner to `argieabulocfirstencoding`.
3. Name it `fesapp-updates`.
4. Select **Public**.
5. Do not add source code, application configuration, environment files, or
   credentials to this repository.
6. Create the repository.

Optionally add a short README such as:

```text
Public release assets for FESApp desktop updates. No application source is stored here.
```

## 2. Create a minimal publishing token

The private source repository's normal `GITHUB_TOKEN` cannot publish to another
repository. Create a fine-grained personal access token instead.

1. In GitHub, open **Settings** → **Developer settings** → **Personal access tokens** → **Fine-grained tokens**.
2. Select **Generate new token**.
3. Give it a clear name, for example `fesapp-development-update-publisher`.
4. Choose an expiration date and record its rotation date. A short expiration is preferred.
5. Under **Repository access**, choose **Only select repositories**.
6. Select only `argieabulocfirstencoding/fesapp-updates`.
7. Under **Repository permissions**, set **Contents** to **Read and write**.
8. Generate the token and copy it immediately. GitHub shows it only once.

Do not put this token in source code, a local `.env` file committed to Git, or
the public update repository.

## 3. Store the token as a source-repository secret

1. Open the private `FESTrainee01/wails-app` repository.
2. Go to **Settings** → **Secrets and variables** → **Actions**.
3. Select **New repository secret**.
4. Set the name to `UPDATES_REPO_TOKEN`.
5. Paste the fine-grained token created in the previous step.
6. Save the secret.

## 4. Update the release workflow

Edit `.github/workflows/development-update.yml` in the private source
repository.

### 4.1 Add the public repository name

At the top of the workflow, after `permissions`, add:

```yaml
env:
  UPDATES_REPOSITORY: argieabulocfirstencoding/fesapp-updates
```

### 4.2 Publish the release to the public repository

Replace the two release steps with the following. The `--repo` option ensures
the assets are published to the public update repository instead of the private
source repository.

```yaml
- name: Create the fixed development prerelease when needed
  shell: pwsh
  env:
    GH_TOKEN: ${{ secrets.UPDATES_REPO_TOKEN }}
  run: |
    gh release view development-latest --repo "$env:UPDATES_REPOSITORY" 2>$null
    if ($LASTEXITCODE -ne 0) {
      # The tag belongs to the updates repository, so do not target the source
      # repository's commit SHA here.
      gh release create development-latest `
        --repo "$env:UPDATES_REPOSITORY" `
        --target main `
        --title "Latest Development Build" `
        --prerelease `
        --notes "Automated development build."
    }

- name: Publish development installer and manifest
  shell: pwsh
  env:
    GH_TOKEN: ${{ secrets.UPDATES_REPO_TOKEN }}
  run: |
    gh release upload development-latest `
      --repo "$env:UPDATES_REPOSITORY" `
      "${{ steps.build.outputs.installer_path }}" `
      "${{ steps.build.outputs.checksum_path }}" `
      development-manifest.json `
      --clobber
```

`--target main` refers to the public update repository's default branch. The
manifest remains the source of truth for the real source commit SHA and build
number.

If the update repository uses `master` instead of `main`, change `--target
main` accordingly.

## 5. Point the desktop updater at the public repository

In `internal/updater/updater.go`, the development channel uses:

```go
developmentRepository = "argieabulocfirstencoding/fesapp-updates"
```

The updater already validates that downloaded assets come from the configured
repository and from the `development-latest` release, so no URL-validation
weakening is required.

Keep the separate stable `Repository` constant pointed to
`argieabulocfirstencoding/fes-app-tracker-new`.

## 6. Commit and publish the configuration

From the private source repository:

```powershell
git add .github/workflows/development-update.yml internal/updater/updater.go
git commit -m "Publish development updates to public asset repository"
git push origin developer/jomary
```

Do not commit the token. It must exist only in the GitHub Actions secret.

## 7. Run and verify the workflow

1. Open **Actions** in `FESTrainee01/wails-app`.
2. Select **Development Desktop Update**.
3. Select **Run workflow**.
4. Choose the `developer/jomary` branch.
5. Start the workflow and wait for every step to finish successfully.
6. Open `argieabulocfirstencoding/fesapp-updates` → **Releases** → `development-latest`.
7. Confirm these exact assets exist:
   - `FESApp-Development-Setup.exe`
   - `FESApp-Development-Setup.exe.sha256`
   - `development-manifest.json`

8. In an incognito/private browser session, open:

   ```text
   https://api.github.com/repos/argieabulocfirstencoding/fesapp-updates/releases/tags/development-latest
   ```

   It must return JSON with the three assets listed above. A `404` means the
   updater will not be able to retrieve the release.

## 8. Verify the desktop behavior

1. Install a development build with a lower build number than the newly
   published release.
2. Open the desktop app.
3. Open **Account** → **Build & updates**.
4. Select **Check for updates**.

Expected results:

- A newer published build displays the update dialog.
- The same or a newer installed build displays **No newer update is available**.
- A missing or private update release displays the actionable release-access
  error, rather than a generic error.

## Security checklist

- [ ] `wails-app` remains private.
- [ ] `argieabulocfirstencoding/fesapp-updates` contains release assets only.
- [ ] `UPDATES_REPO_TOKEN` has access only to `argieabulocfirstencoding/fesapp-updates`.
- [ ] The token has an expiry date and a documented rotation owner.
- [ ] No token appears in commits, workflow logs, installer files, or the
      public update repository.
- [ ] The three release assets are visible without signing in to GitHub.

## Rollback

If publishing fails:

1. Revoke the fine-grained token from GitHub.
2. Delete the `UPDATES_REPO_TOKEN` Actions secret.
3. Remove the cross-repository release steps from the workflow.
4. Keep the desktop updater pointed at the last known-good public update
   repository, or disable development update checks until a new host is ready.
