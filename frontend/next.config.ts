import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  outputFileTracingRoot: process.cwd(),
  images: {
    formats: ["image/webp"],
  },
  // 将前端发往 /api/ 的请求反向代理到 Docker 中的 API Gateway
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:9000/api/:path*",
      },
    ];
  },
};

export default nextConfig;
