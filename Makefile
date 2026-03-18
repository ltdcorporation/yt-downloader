SHELL := /bin/bash

.PHONY: web-dev web-build backend-build backend-test api worker fmt smoke

web-dev:
	cd apps/web && npm run dev

web-build:
	cd apps/web && npm run build

backend-build:
	mkdir -p bin
	cd apps/backend && go build -o ../../bin/ytd-api ./cmd/api
	cd apps/backend && go build -o ../../bin/ytd-worker ./cmd/worker

backend-test:
	./scripts/test-backend.sh

api:
	cd apps/backend && go run ./cmd/api

worker:
	cd apps/backend && go run ./cmd/worker

fmt:
	cd apps/backend && go fmt ./...

smoke:
	./scripts/smoke-mvp.sh
