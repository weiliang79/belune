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
    -o /paas ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata git
COPY --from=backend /paas /usr/local/bin/paas
EXPOSE 8080
ENTRYPOINT ["paas"]
