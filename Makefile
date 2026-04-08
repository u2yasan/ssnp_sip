SHELL := /bin/sh

.PHONY: test build smoke testnet-local

test:
	cd agent_py && python3 -m unittest discover -s tests -v
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./...
	cd probe && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./...

build:
	cd agent_py && python3 -m compileall ssnp_agent
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go build ./...
	cd probe && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go build ./...

smoke:
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./internal/server -run TestSmokeE2E -count=1

testnet-local:
	cd portal && env GOCACHE=$$PWD/.cache/go-build GOMODCACHE=$$PWD/.cache/go-mod go test ./internal/server -run TestTestnetOperableE2E -count=1
