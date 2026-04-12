APP       := album-store
BINARY    := server
IMAGE     ?= $(APP)
TAG       ?= latest
PORT      ?= 8080
DATABASE_URL ?= postgres://album:album@localhost:5432/albumdb

.PHONY: build test run migrate \
        docker-build docker-push \
        up down logs \
        deploy clean

# ── Local build ───────────────────────────────────────────────────────────────

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/server

test:
	go test ./... -v -count=1

run: build
	DATABASE_URL=$(DATABASE_URL) \
	S3_BUCKET=album-photos \
	AWS_ENDPOINT_URL=http://localhost:4566 \
	AWS_REGION=us-east-1 \
	PORT=$(PORT) \
	./$(BINARY)

migrate:
	psql "$(DATABASE_URL)" -f migrations/001_create_albums.sql
	psql "$(DATABASE_URL)" -f migrations/002_create_photos.sql

# ── Docker ────────────────────────────────────────────────────────────────────

docker-build:
	docker build -t $(IMAGE):$(TAG) .

docker-push:
	docker push $(IMAGE):$(TAG)

# ── Docker Compose (local dev) ────────────────────────────────────────────────

up:
	./scripts/run_local.sh

down:
	docker compose down

logs:
	docker compose logs -f app

# ── Terraform deploy ──────────────────────────────────────────────────────────

deploy:
	cd terraform && terraform apply -auto-approve

# ── Cleanup ───────────────────────────────────────────────────────────────────

clean:
	rm -f $(BINARY)
	docker compose down -v --remove-orphans
