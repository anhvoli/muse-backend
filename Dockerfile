# ==========================================
# STAGE 1: Build Go Backend
# ==========================================
FROM golang:1.23-bookworm AS backend-builder
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1-mesa-dev \
    xorg-dev \
    gcc \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# ==========================================
# STAGE 2: Production Runtime
# ==========================================
FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libgl1 \
    libx11-6 \
    libxcursor1 \
    libxrandr2 \
    libxinerama1 \
    libxi6 \
    libxxf86vm1 \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend-builder /app/server /app/server

# Tạo thư mục dữ liệu
RUN mkdir -p /app/selfies /app/parsers

ENV PORT=8088
ENV HEADLESS=true
EXPOSE 8088

# Khởi chạy ở chế độ Server Headless
CMD ["/app/server", "--server"]
