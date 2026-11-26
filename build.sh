#!/usr/bin/env sh
# Build hos-code for every supported platform into dist/, one archive per platform with the licences beside the binary.
set -eu

VERSION=$(cat VERSION | tr -d ' \r\n')
OUT=${OUT:-dist}
LDFLAGS="-s -w"

./scripts/build-web.js

TARGETS="
linux/amd64 linux/arm64 linux/arm linux/386 linux/riscv64
darwin/amd64 darwin/arm64
windows/amd64 windows/arm64 windows/386
freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64
"

rm -rf "$OUT"
mkdir -p "$OUT"

for t in $TARGETS; do
  os=${t%/*}
  arch=${t#*/}
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  name="hos-code-$VERSION-$os-$arch"
  stage="$OUT/$name"
  mkdir -p "$stage"
  printf '  %-34s' "$name"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="$LDFLAGS" -o "$stage/hos-code$ext" .
  cp LICENSE THIRD_PARTY_NOTICES.md README.md "$stage/"
  if [ "$os" = "windows" ]; then
    (cd "$OUT" && zip -qr "$name.zip" "$name")
    archive="$name.zip"
  else
    tar -C "$OUT" -czf "$OUT/$name.tar.gz" "$name"
    archive="$name.tar.gz"
  fi
  rm -rf "$stage"
  printf '%s\n' "$(du -h "$OUT/$archive" | cut -f1)"
done

(cd "$OUT" && if command -v sha256sum >/dev/null 2>&1; then sha256sum hos-code-*; else shasum -a 256 hos-code-*; fi > checksums.txt)
echo "built $(ls -1 "$OUT" | grep -c '^hos-code-') archives in $OUT/ (plus checksums.txt)"
