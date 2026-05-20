# CLI Distribution

Claude Log Analyzer releases are built from git tags by GoReleaser. Release
artifacts are source-provenance assets: each binary embeds the semantic version,
git commit, build timestamp, and source repository shown by
`claude-analyzer version`.

## User Install Path

The launch install path is GitHub Releases first:

1. Open the tagged release at
   `https://github.com/robertDouglass/claude-log-analyzer/releases`.
2. Download the archive for your platform:
   - `claude-analyzer_<version>_darwin_amd64.tar.gz`
   - `claude-analyzer_<version>_darwin_arm64.tar.gz`
   - `claude-analyzer_<version>_linux_amd64.tar.gz`
   - `claude-analyzer_<version>_linux_arm64.tar.gz`
   - `claude-analyzer_<version>_windows_amd64.zip`
3. Verify the archive hash against `checksums.txt` from the same release.
4. Run `claude-analyzer version` and confirm the source URL and commit match
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

## Cutting A Release

1. Confirm `main` is green and the working tree is clean.
2. Create an annotated semver tag:

   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

3. The release workflow publishes a draft GitHub Release with archives and
   `checksums.txt`.
4. Review the draft, checksum asset, changelog, and install commands before
   publishing it.

The workflow can also be run manually against an existing tag from GitHub
Actions.

## Package Manager Plan

GoReleaser is configured to publish a Homebrew formula to
`robertDouglass/homebrew-claude-log-analyzer` when the release workflow has a
`HOMEBREW_TAP_GITHUB_TOKEN` secret with write access to that tap.
If the secret is absent, the workflow skips only the Homebrew publisher and
still publishes GitHub Release archives.

Expected install command after the tap exists:

```sh
brew tap robertDouglass/claude-log-analyzer
brew install claude-analyzer
```

GoReleaser is also configured to publish a Scoop manifest to
`robertDouglass/scoop-claude-log-analyzer` when the workflow has a
`SCOOP_BUCKET_GITHUB_TOKEN` secret with write access to that bucket.
If the secret is absent, the workflow skips only the Scoop publisher.

Expected Windows install command after the bucket exists:

```powershell
scoop bucket add claude-log-analyzer https://github.com/robertDouglass/scoop-claude-log-analyzer
scoop install claude-analyzer
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
