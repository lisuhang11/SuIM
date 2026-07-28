# Multi-stage Dockerfile for SuIM services (Go monorepo)
# Usage: docker build --build-arg SERVICE=user -t suim-user .
ARG GO_VERSION=1.24

# ============================================================
# Stage 1: Resolve dependencies (cached)
# ============================================================
FROM golang:${GO_VERSION}-alpine AS deps
WORKDIR /build
COPY go.work go.work.sum ./
# Copy go.mod/go.sum for all services
COPY services/user/go.mod services/user/go.sum services/user/
COPY services/relation/go.mod services/relation/go.sum services/relation/
COPY services/group/go.mod services/group/go.sum services/group/
COPY services/conversation/go.mod services/conversation/go.sum services/conversation/
COPY services/message/go.mod services/message/go.sum services/message/
COPY services/msggateway/go.mod services/msggateway/go.sum services/msggateway/
COPY services/push/go.mod services/push/go.sum services/push/
COPY services/apigateway/go.mod services/apigateway/go.sum services/apigateway/
# Copy root go.mod for proto imports
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go work sync

# ============================================================
# Stage 2: Build
# ============================================================
FROM deps AS build
ARG SERVICE
WORKDIR /build
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /app/server ./services/${SERVICE}/cmd/server

# ============================================================
# Stage 3: Runtime
# ============================================================
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /app/server .
ENTRYPOINT ["/app/server"]
