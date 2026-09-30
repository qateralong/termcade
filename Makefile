.PHONY: run build build-linux test vet fmt docker

run:
	go run ./cmd/termcade

build:
	go build -trimpath -ldflags="-s -w" -o termcade ./cmd/termcade

# Static binary for a typical Linux VPS.
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/termcade-linux-amd64 ./cmd/termcade

test:
	go test -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

docker:
	docker build -t termcade .
