# Release builds

hos-code release archives are built, not published, by this repository's tooling. Publishing a GitHub release is a
separate, manual step.

## Build locally

```bash
make test
make dist        # dist/hos-code-<version>-<os>-<arch>.tar.gz (.zip on Windows) and dist/checksums.txt
```

Each archive contains `hos-code` (or `hos-code.exe`), `LICENSE`, `THIRD_PARTY_NOTICES.md` and `README.md`. No
PostHog key is compiled in.

## Build on GitHub

Run the **Release build** workflow by hand (Actions → Release build → Run workflow). It runs `./build.sh`, checks
that every archive carries both licence files, and keeps the archives as workflow artifacts for 14 days. It creates
no tag and no release.

## Version

`make publish <version>` writes `VERSION`, builds the archives, commits `VERSION` and tags `v<version>` locally. It
pushes nothing.
