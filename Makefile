BIN      := bin/worker
LINUX   := bin/worker-linux-amd64

.PHONY: build linux test clean

## Build for the local machine (dev/testing).
build:
	go build -o $(BIN) ./cmd/worker

## Cross-compile the static binary to ship on Linux VPS workers.
linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(LINUX) ./cmd/worker

test:
	go test ./...

clean:
	rm -rf bin
