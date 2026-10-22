# Releasing lazyregex

A release is a `v*` tag. Pushing it runs `.github/workflows/release.yaml`:

1. **goreleaser** (ubuntu): builds six archives (linux/darwin/windows x amd64/arm64,
   `CGO_ENABLED=0`), `checksums.txt`, a deb and an rpm per Linux arch, creates the
   GitHub release, and publishes the Homebrew cask, Scoop manifest and AUR package.
2. **chocolatey** (windows-latest, needs `goreleaser`): packs and pushes the
   Chocolatey package (after checking its hashes match the release). Without `CHOCOLATEY_API_KEY` every step is skipped and the job
   still succeeds.

```sh
git tag -a v1.0.0 -m "lazyregex 1.0.0" && git push origin v1.0.0
```

Try a release locally first (publishes nothing, writes `dist/`):

```sh
make release-snapshot     # needs goreleaser v2: brew install goreleaser
```

## Secrets

Set these in the repository's Settings > Secrets and variables > Actions.

| Secret | Who creates it | Without it |
| --- | --- | --- |
| `HOMEBREW_TAP_GITHUB_TOKEN` | The maintainer: a fine-grained PAT with Contents read/write on `dor1202/homebrew-lazyregex`. GoReleaser writes `Casks/lazyregex.rb` there. | **Required.** The release fails at the Homebrew step (after the GitHub release and archives exist). |
| `SCOOP_GITHUB_TOKEN` | The maintainer: a PAT with Contents read/write on the bucket repo `dor1202/scoop-lazyregex`, which must be created first (empty, public). | The Scoop manifest is still generated but not pushed. |
| `AUR_KEY` | The maintainer: an SSH private key whose public half is added to an AUR account, with the `lazyregex-bin` package registered (first push creates it). | The AUR package is generated but not pushed. |
| `CHOCOLATEY_API_KEY` | The maintainer: from the account page on chocolatey.org. The first push goes through moderation. | The `chocolatey` job does nothing. |

`GITHUB_TOKEN` is provided by Actions.

**v1.0.0 needs only `HOMEBREW_TAP_GITHUB_TOKEN`.** Add the other three whenever the
accounts and repos exist; later tags then publish to them. A tag that was released
without one of them is not re-published automatically: re-run the workflow for that tag
after adding the secret (`release.replace_existing_artifacts` makes GoReleaser replace
the assets already uploaded; the builds are reproducible, so the bytes are identical).

## Notes

- Archives are reproducible (`-trimpath`, commit-time `mod_timestamp` and archive
  mtimes). The Chocolatey job rebuilds the Windows zips on a different runner and the
  package points at the zips released from ubuntu, so the job packs with `--skip=publish`, compares their SHA-256 to
  `checksums.txt`, and only then runs `choco push`; on a mismatch nothing is pushed.
- Homebrew ships a cask (`homebrew_casks`; GoReleaser deprecated formulas via `brews`).
  Since v1.0.1 the tap's `tap_migrations.json` maps the retired `Formula/lazyregex.rb` to
  the cask: on `brew update` formula users get a notice with the two switch commands
  (Homebrew does not switch them automatically). Casks work on Linux Homebrew too
  (verified in the `homebrew/brew` container) and, unlike formulas, don't require
  Xcode Command Line Tools on macOS.
- Homebrew 6+ requires users to trust third-party taps (`brew trust --cask
  dor1202/lazyregex/lazyregex`). Only acceptance into `homebrew/core` removes that step.
- The binary embeds all four engines, so archives are about 5-6 MB. Releases need no
  C toolchain and no Docker; the WASM engines are committed under `internal/engine/`
  (rebuild them with `make wasm`).
