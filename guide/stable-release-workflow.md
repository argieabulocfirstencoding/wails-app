# Publishing stable updates with Backblaze B2

Stable FES App updates are built in the source repository and published to the
public Backblaze B2 bucket `fesapp-updates`. GitHub remains the source-control
and build system; GitHub Releases are not used for stable installer downloads.

```text
development -> main -> vMAJOR.MINOR.PATCH tag -> GitHub Actions
    -> signed update manifest + installer -> public Backblaze B2
    -> installed stable app checks B2 -> user chooses Install now
```

Development-channel updates remain on their existing GitHub release path.

## One-time setup

### 1. Backblaze B2 bucket

The `fesapp-updates` bucket must be public and use this endpoint:

```text
https://fesapp-updates.s3.us-east-005.backblazeb2.com
```

The app downloads only public files. It never contains a Backblaze key.

Create a restricted B2 application key with:

| Setting | Value |
| --- | --- |
| Bucket | `fesapp-updates` only |
| Access | Write Only |
| File-name prefix | `stable/` |
| List all bucket names | Off |

In the source repository (`wails-app`), add these GitHub Actions repository
secrets under **Settings -> Secrets and variables -> Actions**:

```text
B2_KEY_ID
B2_APPLICATION_KEY
```

### 2. Manifest-signing key

On a trusted developer machine, from this source checkout, generate a key pair:

```powershell
go run ./cmd/update-manifest-sign -generate-key
```

The command prints two one-line base64 values. Do not put either value in Git,
source code, issue comments, or chat.

In the `wails-app` GitHub repository:

1. Go to **Settings -> Secrets and variables -> Actions**.
2. Under **Variables**, add `UPDATE_MANIFEST_PUBLIC_KEY` with the printed public
   key. This value is safe to compile into the app.
3. Under **Secrets**, add `UPDATE_MANIFEST_SIGNING_KEY` with the printed private
   key. This value is never compiled into the app or uploaded to B2.

The release workflow will fail safely if either signing value is missing or has
the wrong length.

## Release procedure

1. Test and merge the change into `main` as normal.
2. Create a new, unused tag in `vMAJOR.MINOR.PATCH` format.
3. Push the tag to the **source** repository.
4. In GitHub Actions, open **Build and Release** and wait for it to complete.
5. In a signed-out/private browser, verify all of these public URLs, replacing
   `2.6.0` with the release version:

   ```text
   https://fesapp-updates.s3.us-east-005.backblazeb2.com/stable/2.6.0/FESApp-Setup.exe
   https://fesapp-updates.s3.us-east-005.backblazeb2.com/stable/2.6.0/FESApp-Setup.exe.sha256
   https://fesapp-updates.s3.us-east-005.backblazeb2.com/stable/2.6.0/stable-manifest.json
   https://fesapp-updates.s3.us-east-005.backblazeb2.com/stable/latest.json
   ```

6. On a test machine with an older **B2-aware** stable build, use
   **Account -> Build & updates -> Check for updates**, then install the update.

The workflow uploads versioned files before publishing `stable/latest.json`.
Versioned assets are immutable; a tag cannot be reused to replace a bad build.
Publish corrected code as a higher version instead.

## Files in B2

```text
stable/
  2.6.0/
    FESApp-Setup.exe
    FESApp-Setup.exe.sha256
    stable-manifest.json
  latest.json
```

`latest.json` is a signed envelope. Its signed payload contains the installer
SHA-256. The app verifies the Ed25519 signature, validates the expected
versioned B2 paths, and compares the downloaded installer with that signed hash
before launching the Windows installer.

## First B2-aware release

Existing stable installations still check GitHub Releases. They cannot discover
the first build containing this B2 updater. Distribute that first installer
manually from its versioned B2 URL, then later stable updates will be offered
in the app.

## Recovery

- **Missing B2 secret:** add `B2_KEY_ID` and `B2_APPLICATION_KEY` in the source
  repository, then run a new release.
- **Missing signing values:** add the public repository variable and private
  secret described above; do not weaken signature verification.
- **Bad release:** release a corrected, higher version. The updater never
  automatically downgrades users.
- **Signing-key rotation:** first publish a build signed by the old private key
  that contains the new public key, then switch the GitHub signing secret.
