SHELL := /bin/bash

.PHONY: web-dev web-build api worker fmt

web-dev:
	cd apps/web && npm run dev

web-build:
	cd apps/web && npm run build

api:
	cd apps/backend && go run ./cmd/api

worker:
	cd apps/backend && go run ./cmd/worker

fmt:
	cd apps/backend && go fmt ./...
