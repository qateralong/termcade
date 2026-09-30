#!/usr/bin/env bash
# Publishes a new version: runs the checks, commits everything with the
# version as the message, tags it and pushes. GitHub Actions then builds the
# release, and the server picks it up with: sudo termcade update
#
# Usage: scripts/release.sh 0.0.3
set -euo pipefail

die() { printf '\n  \033[1;31m✗\033[0m %s\n\n' "$*" >&2; exit 1; }
say() { printf '  \033[1;35m◆\033[0m %s\n' "$*"; }

VERSION="${1:-}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "Usage: scripts/release.sh X.Y.Z"
TAG="v$VERSION"

cd "$(git rev-parse --show-toplevel)"
[ "$(git branch --show-current)" = "main" ] || die "Releases are cut from main"
git rev-parse -q --verify "refs/tags/$TAG" >/dev/null && die "$TAG already exists"

LAST="$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo v0.0.0)"
newest="$(printf '%s\n%s\n' "${LAST#v}" "$VERSION" | sort -V | tail -1)"
[ "$newest" = "$VERSION" ] && [ "$LAST" != "$TAG" ] || die "$TAG is not newer than $LAST"

echo
say "Checking formatting, vet and tests"
[ -z "$(gofmt -l .)" ] || die "Unformatted files:$(printf ' %s' $(gofmt -l .))"
go vet ./...
go test -count=1 ./... >/dev/null || die "Tests failed; run go test ./..."

if [ -n "$(git status --porcelain)" ]; then
  say "Committing changes as $VERSION"
  git add -A
  git commit -q -m "$VERSION"
else
  say "Nothing to commit; tagging the current commit"
fi

git tag "$TAG"
say "Pushing main and $TAG"
git push -q origin main "$TAG"

echo
say "Done. GitHub is building the release:"
echo "    https://github.com/$(git remote get-url origin | sed -E 's#.*github.com[:/](.*)\.git$#\1#; s#.*github.com[:/](.*)$#\1#')/actions"
echo
echo "  When it finishes, update the server with:  sudo termcade update"
echo
