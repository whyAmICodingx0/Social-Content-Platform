# ============================================
# 階段 1:建置前端
# ============================================
FROM node:22-alpine AS frontend

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# ============================================
# 階段 2:建置後端(把前端產物嵌進 binary)
# ============================================
FROM golang:1.26-alpine AS backend

WORKDIR /app/backend

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN rm -rf internal/web/dist && mkdir -p internal/web/dist
COPY --from=frontend /app/frontend/dist/ ./internal/web/dist/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# ============================================
# 階段 3:最終映像(只有 binary,沒有編譯工具)
# ============================================
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 10001 appuser
USER appuser

COPY --from=backend /server /server

EXPOSE 8080
ENTRYPOINT ["/server"]