# ==========================================
# Stage 1: Build Frontend Assets
# ==========================================
FROM node:22-alpine AS web-builder
WORKDIR /app/web

COPY web/package.json web/package-lock.json* ./
RUN npm install

COPY web/ ./
RUN npm run build

# ==========================================
# Stage 2: Build Host Server Binary
# ==========================================
FROM golang:1.22-alpine AS host-builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY shared/ ./shared/
COPY host/ ./host/

WORKDIR /app/host
ENV GOWORK=off
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/bin/justping-host ./cmd/server

# ==========================================
# Stage 3: Runtime Container
# ==========================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata bash curl iputils

WORKDIR /app

# Copy Host executable and scripts
COPY --from=host-builder /app/bin/justping-host /app/justping-host
COPY --from=web-builder /app/web/dist /app/web/dist
COPY scripts/install.sh /app/scripts/install.sh
RUN chmod +x /app/scripts/install.sh

ENV PORT=8080
ENV GIN_MODE=release

EXPOSE 8080

ENTRYPOINT ["/app/justping-host"]
