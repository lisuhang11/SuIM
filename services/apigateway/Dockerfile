# SuIM apigateway service — 独立构建，不依赖 go.work
# 构建上下文必须是仓库根目录（需要 proto/）：
#   docker build -f services/apigateway/Dockerfile -t suim-apigateway .
FROM golang:1.24-alpine AS build
ENV GOWORK=off CGO_ENABLED=0 GOOS=linux
WORKDIR /src

# 根模块（SuIM，提供 proto 生成代码）
COPY go.mod ./
COPY proto/ proto/
COPY pkg/ pkg/

# 本服务源码
COPY services/apigateway/ services/apigateway/

WORKDIR /src/services/apigateway
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod tidy && go build -o /app/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /app/server .
ENTRYPOINT ["/app/server"]
