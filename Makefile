VERSION ?= dev
LDFLAGS := -s -w -X termcade/internal/version.Version=$(VERSION)

.PHONY: run build build-linux test vet fmt docker release

run:
	go run ./cmd/termcade

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o termcade ./cmd/termcade

# Static binary for a typical Linux VPS.
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/termcade-linux-amd64 ./cmd/termcade

test:
	go test -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

docker:
	docker build --build-arg VERSION=$(VERSION) -t termcade .

# make release VERSION=0.0.3
release:
	./scripts/release.sh $(VERSION)
