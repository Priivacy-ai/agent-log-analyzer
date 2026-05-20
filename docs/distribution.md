# Agent Analyzer Distribution

Agent Analyzer's public launch CTA is NPX first:

```sh
npx --yes agent-analyzer@latest run
```

The npm package is deliberately boring:

- package name: `agent-analyzer`
- binary name: `agent-analyzer`
- no lifecycle scripts
- no runtime npm dependencies
- no dynamic downloader
- bundled native Go binaries for supported platforms
- published from GitHub Actions with npm provenance

The command runs the bundled native Go binary. Analysis still happens locally,
the CLI writes `agent-analyzer-report.json`, the user sees the upload boundary,
and only sanitized report JSON is uploaded after confirmation.

## GitHub Release Fallback

GitHub Releases remain the canonical binary provenance path for users who do
not want npm/NPX:

1. Open the tagged release at
   `https://github.com/Priivacy-ai/agent-log-analyzer/releases`.
2. Download the archive for your platform:
   - `agent-analyzer_<version>_darwin_amd64.tar.gz`
   - `agent-analyzer_<version>_darwin_arm64.tar.gz`
   - `agent-analyzer_<version>_linux_amd64.tar.gz`
   - `agent-analyzer_<version>_linux_arm64.tar.gz`
   - `agent-analyzer_<version>_windows_amd64.zip`
3. Verify the archive hash against `checksums.txt` from the same release.
4. Run `agent-analyzer version` and confirm the source URL and commit match
   the release page.

`go install` remains a developer fallback, not the public launch install path.

## Local Snapshot Release

Use this before tagging:

```sh
go test ./...
go vet ./...
goreleaser check
goreleaser release --snapshot --clean
```

Snapshot artifacts are written under `dist/` and are not published.

For local npm package smoke testing:

```sh
mkdir -p npm/bin
go build -o npm/bin/agent-analyzer-$(node -p '`${process.platform}-${process.arch}`') ./cmd/agent-analyzer
npm pack --dry-run
node npm/bin/agent-analyzer.js version
```

## Cutting A Release

`v0.1.0` was cut before the public NPX rename and should not be used for the
first npm launch. Start public npm distribution with `v0.1.1` or newer.

1. Confirm `main` is green and the working tree is clean.
2. Create an annotated semver tag:

   ```sh
   git tag -a v0.1.1 -m "v0.1.1"
   git push origin v0.1.1
   ```

3. The release workflow publishes a draft GitHub Release with archives and
   `checksums.txt`.
4. The release workflow also builds npm package binaries and publishes
   `agent-analyzer` to npm with provenance. For the first publish of an
   unclaimed package, add a short-lived npm automation token as the GitHub
   `NPM_TOKEN` secret. After the package exists, configure npm Trusted
   Publishing for `release.yml` and remove the token secret.
5. Review the draft, checksum asset, changelog, npm package page, and install
   commands before broad launch.

The workflow can also be run manually against an existing tag from GitHub
Actions.

## Package Manager Plan

GoReleaser is configured to publish a Homebrew formula to
`Priivacy-ai/homebrew-agent-analyzer` when the release workflow has a
`HOMEBREW_TAP_GITHUB_TOKEN` secret with write access to that tap.
If the secret is absent, the workflow skips only the Homebrew publisher and
still publishes GitHub Release archives.

Expected install command after the tap exists:

```sh
brew tap Priivacy-ai/agent-analyzer
brew install agent-analyzer
```

GoReleaser is also configured to publish a Scoop manifest to
`Priivacy-ai/scoop-agent-analyzer` when the workflow has a
`SCOOP_BUCKET_GITHUB_TOKEN` secret with write access to that bucket.
If the secret is absent, the workflow skips only the Scoop publisher.

Expected Windows install command after the bucket exists:

```powershell
scoop bucket add agent-analyzer https://github.com/Priivacy-ai/scoop-agent-analyzer
scoop install agent-analyzer
```

## Signing And Notarization Gap

The current automation does not sign or notarize macOS binaries because Apple
Developer credentials and a notarization secret set are not available in this
repository.

Launch risk: unsigned macOS downloads may trigger Gatekeeper warnings and may
require users to inspect the release provenance and approve the binary manually.
Do not market the macOS binary as signed or notarized until the release workflow
has Apple signing identity, Team ID, app-specific password or API key material,
and a verified notarization step.

Checksums are published for every archive, but checksums are integrity checks,
not publisher identity.
