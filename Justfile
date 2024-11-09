_help:
  @just -l

# Tag a new release
tag:
  #!/bin/bash
  set -euxo pipefail
  version=$(svu next)
  (
    cd extensions/javascript
    go mod edit \
      -require=github.com/block/scaffolder@"$version" \
      -replace=github.com/block/scaffolder=../..
    go mod tidy
    go mod edit \
      -dropreplace=github.com/block/scaffolder
  )
  (
    cd cmd/scaffolder
    go mod edit \
      -require=github.com/block/scaffolder@"$version" \
      -replace=github.com/block/scaffolder=../.. \
      -require=github.com/block/scaffolder/extensions/javascript@"$version" \
      -replace=github.com/block/scaffolder/extensions/javascript=../../extensions/javascript
    go mod tidy
    go mod edit \
      -dropreplace=github.com/block/scaffolder \
      -dropreplace=github.com/block/scaffolder/extensions/javascript
  )
  git add cmd/scaffolder/{go.mod,go.sum} extensions/javascript/{go.mod,go.sum}
  git diff-files --quiet ||  { echo "error: uncommitted changes"; exit 1;}
  git diff-index --quiet HEAD -- || git commit -m "chore: bump version to $version"
  git tag -f "$version"
  git tag -f "cmd/scaffolder/$version"
  git tag -f "extensions/javascript/$version"
  echo "use 'git push && git push --tags' to push the tags to the remote"

release:
  #!/bin/bash
  set -euxo pipefail
  cd cmd/scaffolder
  version=$(svu current)
  go build -ldflags "-X main.version=$version" -o scaffolder .
