SHELL := /bin/sh

.PHONY: test build smoke

test:
	cd agent && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./...
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./...

build:
	cd agent && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go build ./...
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go build ./...

smoke:
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./internal/server -run TestSmokeE2E -count=1
