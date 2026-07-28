# SuIM frontend — Next.js 多阶段构建
# 构建上下文必须是仓库根目录：
#   docker build -f frontend/Dockerfile -t suim-frontend .
FROM node:22-alpine AS build
WORKDIR /app

# 安装依赖（使用国内镜像加速）
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com && npm ci

# 复制源码并构建
COPY frontend/ ./
RUN mkdir -p /app/public && npm run build

# 生产运行阶段
FROM node:22-alpine AS runtime
RUN apk add --no-cache tzdata
WORKDIR /app
ENV NODE_ENV=production

COPY --from=build /app/public ./public
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static

EXPOSE 3000
ENTRYPOINT ["node", "server.js"]
