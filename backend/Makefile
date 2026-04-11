SHELL := /bin/bash

.PHONY: run test

run:
	@set -a; \
	if [ -f .env ]; then source .env; fi; \
	set +a; \
	go run ./cmd/api

test:
	go test ./...
