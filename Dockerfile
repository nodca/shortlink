# syntax=docker/dockerfile:1

# ============================================
# Stage 1: Build frontend (Astro + Tailwind)
# ============================================
FROM node:20-alpine AS frontend

WORKDIR /web

# Copy package files
COPY internal/app/shortlink/web/package*.json ./

# Install dependencies
RUN npm ci

# Copy source files
COPY internal/app/shortlink/web/ ./

# Build static files
RUN npm run build

# ============================================
# Stage 2: Build backend (Go)
# ============================================
FROM golang:1.24 AS build

WORKDIR /src

# Allow overriding module proxy/sumdb during docker build.
# Examples:
#   docker build --build-arg GOPROXY=https://goproxy.cn,direct -t <img> .
#   docker build --build-arg GOSUMDB=off -t <img> .   (last resort)
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.org
ENV GOPROXY=$GOPROXY
ENV GOSUMDB=$GOSUMDB

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go mod download

COPY . .

# Copy frontend build output to static directory for embedding
COPY --from=frontend /web/dist/ ./internal/app/shortlink/httpapi/static/

RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ============================================
# Stage 3: Final image
# ============================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/api /app/api

USER app

EXPOSE 9999
EXPOSE 6060

ENTRYPOINT ["/app/api"]
