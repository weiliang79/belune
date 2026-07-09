# Stage 1: Build the React frontend
FROM node:22-alpine AS frontend
WORKDIR /web
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build

# Stage 2: Build the Go backend (with embedded frontend)
FROM golang:1.24-alpine AS backend
WORKDIR /app
# Install git for go mod download (private modules)
RUN apk add --no-cache git
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
# Copy compiled frontend into the Go embed directory
COPY --from=frontend /web/build ./web/dist/
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION:-dev}" \
    -o /belune ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata git && \
    addgroup -S belune && adduser -S -G belune belune
COPY --from=backend /belune /usr/local/bin/belune
# Writable, persistable location for managed-database logical dumps. The default
# DatabaseBackupDir (/opt/belune/backups/databases) is not creatable by the non-root
# belune user, so point it at a dir we own here. Mount a volume on /data in
# docker-compose.prod.yml so dumps survive container recreation (needed for restore).
RUN mkdir -p /data/backups/databases && chown -R belune:belune /data
ENV DATABASE_BACKUP_DIR=/data/backups/databases
USER belune
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["belune"]
