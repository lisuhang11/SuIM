FROM golang:1.24-alpine AS build
ENV GOWORK=off CGO_ENABLED=0 GOOS=linux
WORKDIR /src
COPY go.mod ./
COPY proto/ proto/
COPY pkg/ pkg/
COPY services/file/ services/file/
WORKDIR /src/services/file
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go mod tidy && go build -o /app/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /app/server .
ENTRYPOINT ["/app/server"]
