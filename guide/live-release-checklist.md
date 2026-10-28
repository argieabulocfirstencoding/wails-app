# Development to live-app release checklist

```text
development -> main -> vMAJOR.MINOR.PATCH tag -> GitHub Actions -> Backblaze B2
```

## One-time checks

- The source repository is `FES-Paul/wails-app`.
- Public B2 bucket: `fesapp-updates`.
- B2 endpoint: `https://fesapp-updates.s3.us-east-005.backblazeb2.com`.
- Source-repository Actions secrets exist: `B2_KEY_ID`, `B2_APPLICATION_KEY`,
  and `UPDATE_MANIFEST_SIGNING_KEY`.
- Source-repository Actions variable exists: `UPDATE_MANIFEST_PUBLIC_KEY`.
- The B2 key is limited to the `fesapp-updates` bucket and `stable/` prefix.

## Each release

1. Commit and test the work on `development`.
2. Merge to `main`.
3. On a clean local `main`, run:

   ```powershell
   go test ./backend/... ./internal/...
   wails build
   ```

4. Create and push an unused higher tag, for example:

   ```powershell
   git tag -a v2.6.0 -m "Release 2.6.0"
   git push origin v2.6.0
   ```

5. Wait for **Build and Release** in GitHub Actions.
6. In a private browser, verify that the installer, checksum, manifest payload,
   and `stable/latest.json` are available under:

   ```text
   https://fesapp-updates.s3.us-east-005.backblazeb2.com/stable/<version>/
   ```

7. On a test machine with an older B2-aware build, select
   **Account -> Build & updates -> Check for updates**, install it, then verify
   the version and normal task tracking.

For signing-key setup, first-release guidance, and recovery instructions, read
[stable-release-workflow.md](./stable-release-workflow.md).
