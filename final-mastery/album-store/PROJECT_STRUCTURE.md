# Album Store — Project Structure

```
album-store/
├── cmd/
│   └── server/
│       └── main.go              # Entry point — starts HTTP server + worker pool
│
├── internal/
│   ├── handler/
│   │   ├── health.go            # GET /health
│   │   ├── album.go             # PUT /albums/:id, GET /albums/:id, GET /albums
│   │   └── photo.go             # POST /albums/:id/photos, GET .../photos/:id, DELETE
│   │
│   ├── model/
│   │   └── models.go            # Structs: Album, Photo, HealthResponse, ErrorResponse
│   │
│   ├── db/
│   │   ├── postgres.go          # Connection pool setup (pgxpool)
│   │   ├── album_repo.go        # Album CRUD queries
│   │   └── photo_repo.go        # Photo CRUD + atomic seq counter queries
│   │
│   ├── worker/
│   │   └── processor.go         # Background goroutine pool — reads from channel,
│   │                            #   uploads to S3, updates status to "completed"
│   │
│   └── storage/
│       └── s3.go                # S3 upload / delete / presigned URL generation
│
├── migrations/
│   ├── 001_create_albums.sql    # Albums table
│   └── 002_create_photos.sql    # Photos table with seq counter
│
├── scripts/
│   ├── run_local.sh             # Docker Compose up for local dev
│   ├── test_local.sh            # Curl-based smoke tests against localhost
│   └── submit.sh                # Submit to ChaosArena
│
├── terraform/
│   ├── main.tf                  # Provider, VPC, subnets
│   ├── ecs.tf                   # ECS Fargate task + service
│   ├── rds.tf                   # PostgreSQL RDS instance
│   ├── s3.tf                    # S3 bucket for photo storage
│   ├── alb.tf                   # Application Load Balancer
│   ├── security_groups.tf       # SG rules
│   ├── variables.tf             # Configurable vars
│   └── outputs.tf               # Base URL, DB endpoint, bucket name
│
├── .github/
│   └── workflows/
│       └── ci.yml               # Build, test, push Docker image
│
├── Dockerfile                   # Multi-stage: build Go binary, copy to scratch/alpine
├── docker-compose.yml           # Local dev: app + postgres + localstack (S3)
├── go.mod
├── go.sum
├── Makefile                     # build, test, docker-build, deploy shortcuts
└── README.md
```

## Architecture Overview

```
                    ┌──────────────┐
   Internet ───────►│     ALB      │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  ECS Fargate  │  (Go HTTP server + worker goroutines)
                    │  (N tasks)    │
                    └──┬────────┬──┘
                       │        │
              ┌────────▼──┐  ┌──▼────────┐
              │ PostgreSQL │  │  S3 Bucket │
              │   (RDS)    │  │  (photos)  │
              └────────────┘  └───────────┘
```

## Key Design Decisions

### Why Go?
- Native concurrency (goroutines) for the async worker pool
- Low memory footprint → cheaper Fargate tasks
- Fast cold starts, great p95 latency under load
- You already used Go for HW6 load testing

### Why PostgreSQL?
- ACID guarantees for the atomic `seq` counter (SELECT FOR UPDATE or serial column)
- Reliable under concurrent writes
- RDS handles backups/failover

### Why in-process worker (goroutines + channel) instead of SQS/Lambda?
- Simpler architecture, fewer moving parts
- No cross-service latency for the "POST→completed" time measured in S12/S15
- Tradeoff: if the container dies, in-flight jobs are lost. Mitigate with a
  "reprocess stale" sweep on startup (find photos stuck in "processing" > 60s)

### Atomic seq assignment
- Use a DB transaction in the POST handler:
  ```sql
  BEGIN;
  SELECT COALESCE(MAX(seq), 0) + 1 FROM photos WHERE album_id = $1 FOR UPDATE;
  INSERT INTO photos (photo_id, album_id, seq, status) VALUES (...);
  COMMIT;
  ```
- The `FOR UPDATE` lock on the album's rows prevents concurrent uploads from
  getting the same seq number.
- Alternative: an `album_seq_counter` column on the albums table, incremented
  atomically with `UPDATE albums SET seq_counter = seq_counter + 1 ... RETURNING seq_counter`

### Photo storage
- Upload raw bytes to S3: `s3://<bucket>/albums/<album_id>/photos/<photo_id>`
- Return the S3 public URL or presigned URL as the `url` field
- On DELETE: remove S3 object + delete DB row

## Database Schema (Preview)

```sql
-- migrations/001_create_albums.sql
CREATE TABLE IF NOT EXISTS albums (
    album_id    UUID PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner       TEXT NOT NULL,
    seq_counter INT  NOT NULL DEFAULT 0,     -- atomic per-album counter
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- migrations/002_create_photos.sql
CREATE TABLE IF NOT EXISTS photos (
    photo_id   UUID PRIMARY KEY,
    album_id   UUID NOT NULL REFERENCES albums(album_id),
    seq        INT  NOT NULL,
    status     TEXT NOT NULL DEFAULT 'processing',  -- processing | completed | failed
    url        TEXT,
    s3_key     TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (album_id, seq)
);

CREATE INDEX idx_photos_album_id ON photos(album_id);
CREATE INDEX idx_photos_status ON photos(status);
```

## Local Development

```bash
# Start postgres + localstack
docker-compose up -d

# Run migrations
psql $DATABASE_URL -f migrations/001_create_albums.sql
psql $DATABASE_URL -f migrations/002_create_photos.sql

# Run server
go run cmd/server/main.go

# Smoke test
./scripts/test_local.sh
```

## Deployment Checklist

1. `terraform apply` in terraform/
2. Build & push Docker image to ECR
3. Update ECS service to use new image
4. Run `./scripts/submit.sh` to trigger ChaosArena

## Performance Tips for Load Tests

- **Connection pooling**: pgxpool with 20-50 max connections
- **Worker pool size**: 10-20 goroutines processing photos concurrently
- **S3 uploads**: use multipart upload for large files (S15)
- **Keep-alive**: ensure HTTP keep-alive is enabled (default in Go)
- **ALB health check**: point at /health with short interval (5s)
- **Horizontal scaling**: 2-3 ECS tasks behind ALB for load tests
  - BUT: if using in-process channels, seq counter is already in DB so
    multiple instances work fine. The channel is per-instance; each instance
    has its own worker pool pulling from its own channel.
- **Stale processing recovery**: on startup, sweep photos with status='processing'
  older than 60s and re-queue them