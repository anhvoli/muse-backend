# 🚀 HƯỚNG DẪN TRIỂN KHAI (DEPLOY) GMAIL HUB & SELFIE VAULT

Ứng dụng gồm **Backend Go** và **Frontend React (Vite + Tailwind CSS)** được thiết kế đồng bộ với tone màu **Trắng & Hồng Pastel**. Hệ thống hỗ trợ 2 màn hình chính:
1. **Màn 1: Hộp Thư Đến** - Quản lý tài khoản Gmail, kết nối IMAP, đọc thư, tự động bóc tách OTP & Link kích hoạt.
2. **Màn 2 (Màn 3 cũ): Kho Ảnh Selfie** - Quản lý ảnh selfie, upload kéo thả, xem kích thước thật, copy chuỗi Base64 Data URI, tải về.

---

## 🌟 TỔNG QUAN KIẾN TRÚC & TÍNH NĂNG ĐỒNG BỘ
- **Cơ chế Single-Port / Single-Binary:** Backend Go tự động phục vụ các API `/api/*` đồng thời phục vụ luôn file tĩnh của React (`frontend/dist`) trên cùng cổng (mặc định `:8088`). Bạn chỉ cần mở `http://<IP-hoặc-Domain>:8088/` là truy cập ngay toàn bộ giao diện Web React!
- **Hỗ trợ Headless Mode:** Backend có sẵn cờ `--server` và nhận diện biến môi trường `HEADLESS=true`, `PORT=...` giúp chạy mượt mà trên môi trường Server Linux / Docker mà không cần card màn hình hay X11 display.
- **Dữ liệu bền vững:** File `accounts.json` lưu tài khoản Gmail và thư mục `selfies/` lưu kho ảnh được mount độc lập, không bị mất khi cập nhật mã nguồn.

---

## 🛠 CÁC HƯỚNG TRIỂN KHAI (DEPLOYMENT OPTIONS)

### 📌 HƯỚNG 1: DOCKER & DOCKER COMPOSE (KHUYÊN DÙNG NHẤT CHO VPS)
Phù hợp với mọi VPS Linux (Ubuntu, Debian, CentOS) hoặc máy chủ Windows/Mac có cài Docker.

#### Bước 1: Chuẩn bị VPS
Cài đặt Docker & Docker Compose trên Ubuntu:
```bash
sudo apt update && sudo apt install -y docker.io docker-compose
```

#### Bước 2: Clone hoặc tải thư mục mã nguồn lên VPS
```bash
git clone <repo-url> /opt/gmail-app
cd /opt/gmail-app
```

#### Bước 3: Khởi chạy bằng 1 lệnh duy nhất
```bash
docker-compose up -d --build
```
- Docker sẽ tự động build React Frontend (`frontend/dist`), build Go binary và khởi chạy container.
- Truy cập vào: `http://<IP-VPS>:8088`

---

### 📌 HƯỚNG 2: CLOUD PAAS (RENDER / RAILWAY / FLY.IO)
Không cần mua VPS, không cần cài đặt hệ điều hành, cấu hình CI/CD tự động từ GitHub.

#### Cách 1: Render.com (Miễn phí)
1. Đăng ký tài khoản tại [Render.com](https://render.com).
2. Chọn **New +** -> **Web Service**.
3. Kết nối với Repository GitHub của dự án.
4. Cấu hình:
   - **Environment:** `Docker`
   - **Dockerfile Path:** `./Dockerfile`
   - **Port:** `8088`
5. Bấm **Deploy Web Service**. Render sẽ cấp một tên miền HTTPS miễn phí (ví dụ: `https://my-gmail-app.onrender.com`).

#### Cách 2: Railway.app
1. Truy cập [Railway.app](https://railway.app), tạo New Project -> **Deploy from GitHub repo**.
2. Railway tự động phát hiện `Dockerfile` và deploy.
3. Trong mục **Settings**, thêm Public Domain để nhận đường link truy cập.

---

### 📌 HƯỚNG 3: VPS UBUNTU CHẠY TRỰC TIẾP (SYSTEMD + NGINX HTTPS)
Tối ưu hiệu năng tối đa (native performance) và cài đặt tên miền riêng với chứng chỉ SSL Let's Encrypt miễn phí.

#### Bước 1: Build Frontend & Backend
Trên máy build hoặc ngay trên VPS:
```bash
# 1. Build React Frontend
cd frontend
npm install
npm run build
cd ..

# 2. Build Go Binary
go build -ldflags="-s -w" -o GmailApp .
```

#### Bước 2: Tạo Systemd Service
Tạo file dịch vụ chạy nền:
```bash
sudo nano /etc/systemd/system/gmail-app.service
```
Nội dung file:
```ini
[Unit]
Description=Gmail Hub & Selfie Vault Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gmail-app
Environment=HEADLESS=true
Environment=PORT=8088
ExecStart=/opt/gmail-app/GmailApp --server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```
Kích hoạt service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gmail-app
sudo systemctl status gmail-app
```

#### Bước 3: Cấu hình Nginx Reverse Proxy & SSL Let's Encrypt
```nginx
server {
    server_name mail.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:8088;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
Kích hoạt SSL miễn phí:
```bash
sudo certbot --nginx -d mail.yourdomain.com
```

---

### 📌 HƯỚNG 4: DEPLOY TÁCH RỜI (FRONTEND VERCEL + BACKEND VPS)
Phương án cho tốc độ tải trang toàn cầu nhanh nhất:
- **Frontend:** Đẩy thư mục `frontend/` lên **Vercel** hoặc **Netlify** (miễn phí, CDN toàn cầu).
- **Backend:** Chạy Go backend trên VPS hoặc Render.
- **Kết nối:** Người dùng mở Web trên Vercel, bấm biểu tượng **Cài đặt (bánh răng)** ở góc phải trên cùng và nhập URL của Backend (ví dụ: `https://api.yourdomain.com`). Cấu hình này được lưu trên trình duyệt của người dùng.

---

### 📌 HƯỚNG 5: ZERO-COST VỚI CLOUDFLARE TUNNEL (CHẠY TẠI MÁY NHÀ/PC)
Nếu bạn không muốn thuê VPS:
1. Chạy `GmailApp.exe` trên máy tính cá nhân của bạn.
2. Cài đặt **Cloudflare Tunnel** (`cloudflared`):
   ```bash
   cloudflared tunnel --url http://localhost:8088
   ```
3. Cloudflare sẽ cấp ngay cho bạn một link HTTPS công khai (ví dụ: `https://xyz-abc.trycloudflare.com`). Bất kỳ ai trên thế giới cũng có thể truy cập được mà bạn không cần mở port modem (NAT port).

---

## 🔒 LƯU Ý BẢO MẬT KHI DEPLOY PRODUCTION
1. **Mật khẩu ứng dụng Gmail (App Password):** Các mật khẩu 16 ký tự được lưu trữ trong `accounts.json`. Hãy đảm bảo phân quyền file chỉ định (`chmod 600 accounts.json`) và không commit file này lên public Git repository.
2. **Kho ảnh Selfie:** Được lưu trong thư mục `selfies/`. Khi backup hệ thống, chỉ cần sao lưu thư mục `selfies/` và file `accounts.json`.
3. **CORS:** Backend đã được mở cấu hình CORS `*` để linh hoạt khi deploy Frontend và Backend ở các domain khác nhau.
