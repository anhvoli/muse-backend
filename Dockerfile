FROM golang:1.23-bookworm

WORKDIR /app

# Cài đặt đầy đủ các thư viện C/X11/OpenGL cần thiết
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1-mesa-dev \
    xorg-dev \
    gcc \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy và tải dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy mã nguồn và biên dịch binary Go
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /app/server .
RUN chmod +x /app/server

# Tạo các thư mục lưu trữ dữ liệu
RUN mkdir -p /app/selfies /app/parsers

ENV PORT=8088
ENV HEADLESS=true
EXPOSE 8088

# Khởi chạy ở chế độ Server Headless
CMD ["/app/server", "--server"]
