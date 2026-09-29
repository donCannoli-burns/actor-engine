.PHONY: test smoke build

test:
	go test ./...

build:
	go build -o bin/kol-actor-engine ./cmd/kol-actor-engine

smoke:
	./scripts/smoke.sh
