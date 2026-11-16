ARG GO_IMAGE=golang:1.26-alpine

FROM ${GO_IMAGE} AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg/ pkg/
COPY proto/ proto/
COPY services/ services/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/suim ./services/${SERVICE}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && mkdir -p /app/migrations
COPY --from=build /out/suim /app/suim
COPY migrations/ /app/migrations/
WORKDIR /app
ENTRYPOINT ["/app/suim"]
