# FES App Tracker releases

This public repository hosts official Windows update files for FES App Tracker.

It contains GitHub Releases created automatically by the release workflow in the
private source repository. Each stable release includes:

- `FESApp-Setup.exe` — the Windows installer
- `FESApp-Setup.exe.sha256` — the installer checksum
- `stable-manifest.json` — update metadata used by the desktop app

End users can install the latest version from the
[Releases page](https://github.com/argieabulocfirstencoding/fes-app-tracker-new/releases).

Do not commit application source code, credentials, API keys, databases, or user
data to this repository. Release assets are published by GitHub Actions.
