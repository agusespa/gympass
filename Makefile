.PHONY: hooks verify dev run web build test lint e2e docker

IMAGE ?= ghcr.io/agusespa/gympass:latest
PLATFORMS ?= linux/amd64,linux/arm64

hooks:
	git config core.hooksPath scripts

verify:
	./scripts/verify.sh

# Go API on :8080 plus the Vite dev server on :5173 (proxies /api to Go).
dev:
	@trap 'kill 0' INT TERM EXIT; \
	go run ./cmd/gym-tracker & \
	npm --prefix web run dev -- --port 5173

# Build the frontend and run the production server on :8080.
run: web
	go run ./cmd/gym-tracker

# Build the frontend and copy it where the Go binary embeds it from.
web:
	npm --prefix web ci
	npm --prefix web run build
	find internal/ui/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
	cp -R web/build/. internal/ui/dist/

build: web
	CGO_ENABLED=0 go build -o bin/gym-tracker ./cmd/gym-tracker

test:
	go test -v -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

e2e: build
	npm ci
	npx playwright install chromium
	npm run test:e2e

docker:
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE) .
