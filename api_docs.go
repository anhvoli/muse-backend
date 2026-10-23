package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// handleAPIDocument phục vụ trang tài liệu HTML hoặc dữ liệu JSON mô tả API
func handleAPIDocument(rw http.ResponseWriter, r *http.Request) {
	// CORS Headers
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "*")
	rw.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

	if r.Method == http.MethodOptions {
		rw.WriteHeader(http.StatusOK)
		return
	}

	// Nếu client yêu cầu JSON (?format=json hoặc Accept: application/json)
	if r.URL.Query().Get("format") == "json" || (r.Header.Get("Accept") != "" && strings.Contains(r.Header.Get("Accept"), "application/json") && !strings.Contains(r.Header.Get("Accept"), "text/html")) {
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"name":        "Gmail Desktop Gateway API",
			"description": "Cổng API đọc thư Gmail trực tiếp từ ứng dụng Desktop qua giao thức IMAP",
			"version":     "1.0.0",
			"auth":        "None (Công khai không cần token)",
			"cors":        "Enabled (*)",
			"endpoints": []map[string]interface{}{
				{
					"path":        "/api/getemaillist",
					"method":      "GET",
					"description": "Lấy danh sách các tài khoản Gmail hiện có trong app",
					"parameters":  []interface{}{},
					"response_example": []string{
						"user1@gmail.com",
						"user2@gmail.com",
					},
				},
				{
					"path":        "/api/getemail",
					"methods":     []string{"POST", "GET"},
					"description": "Đọc các email mới nhất của tài khoản Gmail chỉ định",
					"parameters": []map[string]interface{}{
						{
							"name":        "email",
							"type":        "string",
							"required":    true,
							"description": "Địa chỉ Gmail cần đọc (chấp nhận alias 'data')",
						},
						{
							"name":        "limit",
							"type":        "integer",
							"required":    false,
							"default":     5,
							"description": "Số lượng thư mới nhất cần lấy (tối đa 50)",
						},
					},
					"response_example": []map[string]interface{}{
						{
							"id":           105,
							"from":         "Google Security <no-reply@accounts.google.com>",
							"to":           "user1@gmail.com",
							"subject":      "Cảnh báo bảo mật mới",
							"date":         "2026-10-22 23:15:00",
							"body_snippet": "Thiết bị mới vừa đăng nhập vào tài khoản của bạn...",
						},
					},
				},
				{
					"path":        "/api/status",
					"method":      "GET",
					"description": "Kiểm tra trạng thái server, Tailscale IP, Public URL",
				},
				{
					"path":        "/api/session/report",
					"method":      "POST",
					"description": "Báo cáo cập nhật tiến độ của phiên làm việc (id, name, current, total, status, message)",
				},
				{
					"path":        "/api/session/list",
					"method":      "GET",
					"description": "Lấy danh sách và tiến độ toàn bộ các phiên làm việc",
				},
				{
					"path":        "/api/selfies",
					"method":      "GET",
					"description": "Lấy danh sách tất cả các ảnh selfie có trong kho",
				},
				{
					"path":        "/api/selfie",
					"method":      "GET",
					"description": "Lấy ngẫu nhiên 1 ảnh selfie (dạng file ảnh thô hoặc JSON/Base64)",
				},
				{
					"path":        "/api/selfie/file",
					"method":      "GET",
					"description": "Tải hoặc hiển thị trực tiếp ảnh selfie theo tên file (?name=...)",
				},
				{
					"path":        "/api/selfie/download-url",
					"method":      "POST",
					"description": "Tự động tải ảnh selfie từ link URL trên internet về kho",
				},
				{
					"path":        "/api/selfie/upload",
					"methods":     []string{"POST", "PUT"},
					"description": "Thêm/tải ảnh mới lên kho selfie (hỗ trợ multipart/form-data, Base64 JSON, raw binary)",
				},
				{
					"path":        "/api/selfie/delete",
					"methods":     []string{"POST", "DELETE"},
					"description": "Xóa ảnh selfie khỏi kho theo tên hoặc xóa toàn bộ kho ảnh",
				},
				{
					"path":        "/api/document",
					"method":      "GET",
					"description": "Tài liệu API tương tác trực tiếp",
				},
			},
		})
		return
	}

	// Mặc định trả về giao diện Web HTML tài liệu tương tác
	port := "8088"
	if apiServer != nil && apiServer.Addr != "" {
		parts := strings.Split(apiServer.Addr, ":")
		if len(parts) > 1 {
			port = parts[len(parts)-1]
		}
	}

	lanIP := getLocalIP()
	tunnelMu.Lock()
	pubURL := publicURL
	if pubURL == "" && tsFunnelURL != "" {
		pubURL = tsFunnelURL
	}
	tunnelMu.Unlock()

	html := renderAPIDocumentHTML(port, lanIP, tsIP, tsDNSName, pubURL)
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Write([]byte(html))
}

func renderAPIDocumentHTML(port, lanIP, tsIP, tsDNS, pubURL string) string {
	if port == "" {
		port = "8088"
	}
	if lanIP == "" {
		lanIP = "127.0.0.1"
	}

	pubDisplay := pubURL
	if pubDisplay == "" {
		pubDisplay = "Chưa kích hoạt"
	}

	tsDisplay := tsIP
	if tsDisplay == "" {
		tsDisplay = "Chưa phát hiện Tailscale"
	} else {
		tsDisplay = fmt.Sprintf("http://%s:%s", tsIP, port)
	}

	return `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Tài Liệu & Cổng Kiểm Thử API Đọc Thư Gmail</title>
  <style>
    :root {
      --bg: #fff5f8;
      --card-bg: #ffffff;
      --card-border: #fce7f3;
      --card-hover: #fff0f5;
      --text: #1e293b;
      --text-muted: #64748b;
      --primary: #ec4899;
      --primary-hover: #db2777;
      --success: #10b981;
      --success-bg: rgba(16, 185, 129, 0.12);
      --warning: #f59e0b;
      --danger: #e11d48;
      --danger-bg: rgba(225, 29, 72, 0.12);
      --code-bg: #fff9fb;
      --pink-glow: rgba(236, 72, 153, 0.2);
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      background-color: var(--bg);
      color: var(--text);
      line-height: 1.6;
      padding-bottom: 80px;
    }
    a { color: var(--primary); text-decoration: none; }
    a:hover { text-decoration: underline; }
    
    .container {
      max-width: 1200px;
      margin: 0 auto;
      padding: 0 20px;
    }

    /* Navbar */
    header {
      background: rgba(255, 255, 255, 0.95);
      backdrop-filter: blur(12px);
      border-bottom: 1px solid var(--card-border);
      position: sticky;
      top: 0;
      z-index: 100;
      box-shadow: 0 2px 10px rgba(236, 72, 153, 0.05);
    }
    .header-content {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 70px;
    }
    .logo-area {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .logo-icon {
      font-size: 28px;
    }
    .logo-title {
      font-size: 20px;
      font-weight: 700;
      letter-spacing: -0.5px;
      background: linear-gradient(135deg, #ec4899, #f43f5e, #fb7185);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }
    .badges-area {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    /* Badges */
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 4px 10px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 600;
      border: 1px solid transparent;
    }
    .badge-success {
      background: var(--success-bg);
      color: #059669;
      border-color: rgba(5, 150, 105, 0.3);
    }
    .badge-primary {
      background: #fdf2f8;
      color: #db2777;
      border-color: #fbcfe8;
    }
    .badge-method-get {
      background: rgba(16, 185, 129, 0.12);
      color: #059669;
      border: 1px solid #10b981;
      font-family: monospace;
      font-weight: bold;
    }
    .badge-method-post {
      background: #fdf2f8;
      color: #db2777;
      border: 1px solid #f472b6;
      font-family: monospace;
      font-weight: bold;
    }

    /* Hero Section */
    .hero {
      padding: 40px 0 25px;
      text-align: center;
    }
    .hero h1 {
      font-size: 32px;
      font-weight: 800;
      margin-bottom: 12px;
      letter-spacing: -0.5px;
      color: #831843;
    }
    .hero p {
      font-size: 16px;
      color: var(--text-muted);
      max-width: 750px;
      margin: 0 auto;
    }

    /* Network Grid */
    .network-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
      gap: 16px;
      margin: 30px 0 40px;
    }
    .net-card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 12px;
      padding: 18px 20px;
      transition: border-color 0.2s, box-shadow 0.2s;
      box-shadow: 0 4px 15px rgba(236, 72, 153, 0.05);
    }
    .net-card:hover {
      border-color: var(--primary);
      box-shadow: 0 6px 20px rgba(236, 72, 153, 0.12);
    }
    .net-label {
      font-size: 13px;
      color: var(--text-muted);
      margin-bottom: 6px;
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .net-value {
      font-size: 15px;
      font-weight: 600;
      font-family: monospace;
      color: #0f172a;
      word-break: break-all;
    }

    /* Section Cards */
    .card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 14px;
      padding: 26px;
      margin-bottom: 30px;
      box-shadow: 0 6px 25px rgba(236, 72, 153, 0.06);
    }
    .card-title {
      font-size: 20px;
      font-weight: 700;
      margin-bottom: 8px;
      display: flex;
      align-items: center;
      gap: 10px;
      color: #831843;
    }
    .card-subtitle {
      font-size: 14px;
      color: var(--text-muted);
      margin-bottom: 22px;
    }

    /* Interactive Playground */
    .tester-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 24px;
    }
    @media (max-width: 900px) {
      .tester-grid { grid-template-columns: 1fr; }
    }
    .test-box {
      background: #fff8fa;
      border: 1px solid var(--card-border);
      border-radius: 10px;
      padding: 20px;
    }
    .form-group {
      margin-bottom: 16px;
    }
    .form-label {
      display: block;
      font-size: 13px;
      font-weight: 600;
      margin-bottom: 6px;
      color: #334155;
    }
    .form-control {
      width: 100%;
      background: #ffffff;
      border: 1px solid #fbcfe8;
      color: #1e293b;
      padding: 10px 14px;
      border-radius: 8px;
      font-size: 14px;
      outline: none;
      transition: border-color 0.2s, box-shadow 0.2s;
    }
    .form-control:focus {
      border-color: var(--primary);
      box-shadow: 0 0 0 3px rgba(236, 72, 153, 0.15);
    }
    select.form-control {
      cursor: pointer;
    }
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      background: linear-gradient(135deg, #ec4899, #db2777);
      color: #fff;
      padding: 10px 20px;
      border-radius: 8px;
      border: none;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.2s, transform 0.1s, box-shadow 0.2s;
      box-shadow: 0 4px 12px rgba(236, 72, 153, 0.25);
    }
    .btn:hover {
      background: linear-gradient(135deg, #f43f5e, #be185d);
      box-shadow: 0 6px 16px rgba(236, 72, 153, 0.35);
    }
    .btn:active {
      transform: scale(0.98);
    }
    .btn-secondary {
      background: #fdf2f8;
      color: #db2777;
      border: 1px solid #fbcfe8;
      box-shadow: none;
    }
    .btn-secondary:hover {
      background: #fce7f3;
      color: #9d174d;
      box-shadow: 0 2px 8px rgba(236, 72, 153, 0.15);
    }
    .btn-sm {
      padding: 6px 12px;
      font-size: 12px;
    }

    /* Output Console */
    .output-container {
      margin-top: 16px;
    }
    .output-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 8px 14px;
      background: #fdf2f8;
      border: 1px solid #fce7f3;
      border-bottom: none;
      border-radius: 8px 8px 0 0;
      font-size: 12px;
      color: var(--text-muted);
    }
    pre.code-block {
      background: var(--code-bg);
      border: 1px solid #fce7f3;
      border-radius: 0 0 8px 8px;
      padding: 14px;
      font-family: "Fira Code", Consolas, Monaco, monospace;
      font-size: 13px;
      color: #be185d;
      overflow-x: auto;
      max-height: 380px;
      white-space: pre-wrap;
      word-break: break-word;
    }

    /* Email Cards in Playground */
    .email-item-card {
      background: #ffffff;
      border: 1px solid #fce7f3;
      border-left: 4px solid var(--primary);
      border-radius: 8px;
      padding: 14px;
      margin-bottom: 12px;
      transition: transform 0.15s, border-color 0.15s, box-shadow 0.15s;
    }
    .email-item-card:hover {
      border-color: #ec4899;
      transform: translateY(-2px);
      box-shadow: 0 4px 15px rgba(236, 72, 153, 0.1);
    }
    .email-item-header {
      display: flex;
      justify-content: space-between;
      font-size: 12px;
      color: var(--text-muted);
      margin-bottom: 6px;
    }
    .email-item-from {
      font-weight: 700;
      color: #0f172a;
    }
    .email-item-subject {
      font-size: 15px;
      font-weight: 600;
      color: #be185d;
      margin-bottom: 6px;
    }
    .email-item-snippet {
      font-size: 13px;
      color: #475569;
      line-height: 1.4;
    }

    /* Tabs */
    .tabs-nav {
      display: flex;
      gap: 4px;
      border-bottom: 1px solid var(--card-border);
      margin-bottom: 16px;
    }
    .tab-btn {
      background: transparent;
      border: none;
      color: var(--text-muted);
      padding: 8px 16px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
      border-bottom: 2px solid transparent;
      transition: color 0.2s, border-color 0.2s;
    }
    .tab-btn.active {
      color: #db2777;
      border-bottom-color: var(--primary);
    }

    /* Endpoint Docs */
    .endpoint-header {
      display: flex;
      align-items: center;
      gap: 14px;
      margin-bottom: 12px;
      flex-wrap: wrap;
    }
    .endpoint-path {
      font-size: 18px;
      font-family: monospace;
      font-weight: 700;
      color: #831843;
    }
    .param-table {
      width: 100%;
      border-collapse: collapse;
      margin: 16px 0;
      font-size: 14px;
    }
    .param-table th, .param-table td {
      border: 1px solid var(--card-border);
      padding: 10px 14px;
      text-align: left;
    }
    .param-table th {
      background: #fdf2f8;
      color: #831843;
      font-weight: 600;
    }
    .param-table td {
      background: #ffffff;
      color: #334155;
    }
    .param-name {
      font-family: monospace;
      font-weight: 700;
      color: #db2777;
    }
    .param-type {
      font-family: monospace;
      font-size: 12px;
      color: #e11d48;
    }
    .param-req {
      color: var(--danger);
      font-size: 12px;
      font-weight: 600;
    }

    .toast {
      position: fixed;
      bottom: 20px;
      right: 20px;
      background: #ec4899;
      color: #fff;
      padding: 12px 20px;
      border-radius: 8px;
      font-weight: 600;
      box-shadow: 0 6px 18px rgba(236, 72, 153, 0.35);
      display: none;
      z-index: 1000;
    }
  </style>
</head>
<body>

  <!-- Toast Copy -->
  <div id="toast" class="toast">Đã sao chép vào bộ nhớ tạm!</div>

  <!-- Header -->
  <header>
    <div class="container header-content">
      <div class="logo-area">
        <span class="logo-icon">📬</span>
        <div>
          <div class="logo-title">Gmail Gateway API</div>
          <div style="font-size: 11px; color: var(--text-muted);">Cổng Đọc Thư IMAP Desktop & Tích Hợp Hệ Thống</div>
        </div>
      </div>
      <div class="badges-area">
        <span class="badge badge-success">● API Đang Chạy</span>
        <span class="badge badge-primary">🔓 Không Cần Token</span>
        <span class="badge badge-primary">⚡ CORS: *</span>
      </div>
    </div>
  </header>

  <div class="container">
    <!-- Hero -->
    <div class="hero">
      <h1>Tài Liệu Hướng Dẫn & Kiểm Thử API</h1>
      <p>Hệ thống cho phép các dịch vụ bên ngoài (Python, Node.js, Bot, CRM, Automation) đọc hộp thư Gmail trực tiếp thông qua ứng dụng máy tính này mà không cần cấu hình OAuth2 phức tạp.</p>
    </div>

    <!-- Network Addresses Grid -->
    <div class="network-grid">
      <div class="net-card">
        <div class="net-label">🏠 Máy Cục Bộ (Localhost)</div>
        <div class="net-value">http://localhost:` + port + `</div>
        <button class="btn btn-secondary btn-sm" style="margin-top: 8px;" onclick="copyText('http://localhost:` + port + `')">📋 Sao chép</button>
      </div>

      <div class="net-card">
        <div class="net-label">🌐 Mạng Nội Bộ (LAN IP)</div>
        <div class="net-value">http://` + lanIP + `:` + port + `</div>
        <button class="btn btn-secondary btn-sm" style="margin-top: 8px;" onclick="copyText('http://` + lanIP + `:` + port + `')">📋 Sao chép</button>
      </div>

      <div class="net-card">
        <div class="net-label">🦎 Mạng Lưới Tailscale (Mesh IP)</div>
        <div class="net-value">` + tsDisplay + `</div>
        <button class="btn btn-secondary btn-sm" style="margin-top: 8px;" onclick="copyText('` + tsDisplay + `')">📋 Sao chép</button>
      </div>

      <div class="net-card">
        <div class="net-label">🌍 Cổng Công Khai (Public Tunnel)</div>
        <div class="net-value">` + pubDisplay + `</div>
        <button class="btn btn-secondary btn-sm" style="margin-top: 8px;" onclick="copyText('` + pubDisplay + `')">📋 Sao chép</button>
      </div>
    </div>

    <!-- PLAYGROUND: KIỂM THỬ TRỰC TIẾP -->
    <div class="card" id="playground">
      <div class="card-title">
        <span>⚡</span> Trình Kiểm Thử API Trực Tiếp (Interactive Tester)
      </div>
      <div class="card-subtitle">Thử nghiệm gửi yêu cầu và nhận kết quả đọc thư từ ứng dụng ngay trên trình duyệt web:</div>

      <div class="tester-grid">
        <!-- Bước 1: Lấy danh sách email -->
        <div class="test-box">
          <div style="font-weight: 700; font-size: 15px; margin-bottom: 12px; color: #059669;">
            1. Lấy danh sách Email hiện có (/api/getemaillist)
          </div>
          <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 16px;">
            Gọi API lấy toàn bộ các tài khoản Gmail đã đăng nhập trong ứng dụng.
          </p>
          <button class="btn" onclick="runGetEmailList()">
            <span>🚀</span> Gửi Yêu Cầu /api/getemaillist
          </button>

          <div class="output-container">
            <div class="output-header">
              <span id="list-status">Chưa thực thi</span>
              <button class="btn btn-secondary btn-sm" onclick="copyResult('list-output')">Sao chép</button>
            </div>
            <pre class="code-block" id="list-output">// Kết quả JSON sẽ hiển thị ở đây...</pre>
          </div>
        </div>

        <!-- Bước 2: Đọc email cụ thể -->
        <div class="test-box">
          <div style="font-weight: 700; font-size: 15px; margin-bottom: 12px; color: #db2777;">
            2. Đọc thư từ một Email (/api/getemail)
          </div>

          <div class="form-group">
            <label class="form-label">Chọn email (từ danh sách) hoặc nhập:</label>
            <select class="form-control" id="target-select" onchange="onSelectEmailChange()">
              <option value="">-- Bấm bước 1 để tải danh sách --</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Hoặc nhập email trực tiếp:</label>
            <input type="email" class="form-control" id="target-email" placeholder="ví dụ: user@gmail.com" />
          </div>

          <div class="form-group">
            <label class="form-label">Số thư mới nhất cần lấy (Limit: 1 - 50):</label>
            <input type="number" class="form-control" id="target-limit" value="5" min="1" max="50" />
          </div>

          <div style="display: flex; gap: 10px; margin-top: 14px;">
            <button class="btn" onclick="runGetEmail('POST')">
              <span>📩</span> Gửi POST /api/getemail
            </button>
            <button class="btn btn-secondary" onclick="runGetEmail('GET')">
              <span>🔍</span> Gửi GET
            </button>
          </div>

          <div class="output-container">
            <div class="output-header">
              <span id="email-status">Chưa thực thi</span>
              <div style="display: flex; gap: 6px;">
                <button class="btn btn-secondary btn-sm" onclick="toggleResultView('cards')">Dạng Thẻ</button>
                <button class="btn btn-secondary btn-sm" onclick="toggleResultView('raw')">JSON Thô</button>
                <button class="btn btn-secondary btn-sm" onclick="copyResult('email-output')">Sao chép</button>
              </div>
            </div>

            <!-- Vùng hiển thị dạng thẻ thư -->
            <div id="email-cards-view" style="max-height: 380px; overflow-y: auto; background: var(--code-bg); border: 1px solid #fce7f3; border-radius: 0 0 8px 8px; padding: 12px; display: none;">
              <!-- Cards chèn vào đây -->
            </div>

            <!-- Vùng JSON thô -->
            <pre class="code-block" id="email-output">// Kết quả JSON đọc thư sẽ hiển thị ở đây...</pre>
          </div>
        </div>

        <!-- Bước 3: Mail Parser -->
        <div class="test-box" style="grid-column: 1 / -1; margin-top: 8px;">
          <div style="font-weight: 700; font-size: 15px; margin-bottom: 8px; color: #be185d;">
            3. Mail Parser - Bóc Tách Dữ Liệu Tự Động (/api/parsers & /api/parse)
          </div>
          <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 12px;">
            Mọi email khi đọc qua <code>/api/getemail</code> sẽ tự động đính kèm dữ liệu trích xuất (OTP, Link, Đơn hàng) nếu khớp với các Parser đã cấu hình trong app.
          </p>
          <div style="display: flex; gap: 10px; margin-bottom: 12px;">
            <button class="btn btn-secondary" onclick="runGetParsers()">
              <span>📑</span> Lấy Danh Sách Parser (/api/parsers)
            </button>
            <button class="btn btn-secondary" onclick="runTestParse()">
              <span>🧪</span> Thử Bóc Tách Văn Bản (/api/parse)
            </button>
          </div>
          <div class="output-container">
            <div class="output-header">
              <span id="parser-status">Sẵn sàng</span>
              <button class="btn btn-secondary btn-sm" onclick="copyResult('parser-output')">Sao chép</button>
            </div>
            <pre class="code-block" id="parser-output">// Kết quả danh sách Parser hoặc bóc tách sẽ hiển thị tại đây...</pre>
          </div>
        </div>

        <!-- Bước 4: Báo Cáo Tiến Độ Phiên (Hỗ trợ Live Image) -->
        <div class="test-box" style="grid-column: 1 / -1; margin-top: 14px; border-left: 4px solid #ec4899;">
          <div style="font-weight: 700; font-size: 15px; margin-bottom: 8px; color: #db2777;">
            4. Báo Cáo & Giám Sát Tiến Độ Phiên Kèm Live Image (/api/session/report & /api/session/image)
          </div>
          <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 12px;">
            Dành cho bot/tool ngoài liên tục báo cáo tiến độ các công việc (số bước, % hoàn thành, log và ảnh chụp màn hình / webcam live).
          </p>

          <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 10px; margin-bottom: 12px;">
            <div>
              <label class="form-label">Mã Phiên (ID):</label>
              <input type="text" class="form-control" id="sess-input-id" value="sess-bot-01" />
            </div>
            <div>
              <label class="form-label">Tên Phiên (Tác vụ):</label>
              <input type="text" class="form-control" id="sess-input-name" value="Bot Đăng Ký Tài Khoản" />
            </div>
            <div>
              <label class="form-label">Bước hiện tại (Current):</label>
              <input type="number" class="form-control" id="sess-input-current" value="35" />
            </div>
            <div>
              <label class="form-label">Tổng số bước (Total):</label>
              <input type="number" class="form-control" id="sess-input-total" value="100" />
            </div>
            <div>
              <label class="form-label">Thông báo (Message):</label>
              <input type="text" class="form-control" id="sess-input-msg" value="Đang đọc mã OTP cho user_35" />
            </div>
          </div>

          <!-- Nhập ảnh kèm theo -->
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 12px;">
            <div>
              <label class="form-label">🖼️ Đính kèm file ảnh (Upload):</label>
              <input type="file" id="sess-input-file" accept="image/*" class="form-control" style="background: #ffffff;" />
            </div>
            <div>
              <label class="form-label">🔗 Hoặc nhập Base64 / URL ảnh:</label>
              <input type="text" class="form-control" id="sess-input-img" placeholder="data:image/png;base64,... hoặc http://..." />
            </div>
          </div>

          <div style="display: flex; gap: 10px; margin-bottom: 12px; flex-wrap: wrap;">
            <button class="btn" onclick="runReportSession()">
              <span>🚀</span> Gửi Báo Cáo Tiến Độ (Kèm Ảnh)
            </button>
            <button class="btn btn-secondary" onclick="runGetSessions()">
              <span>📊</span> Xem Danh Sách Phiên (/api/session/list)
            </button>
            <button class="btn btn-secondary" onclick="runViewLatestSessionImage()">
              <span>🖼️</span> Xem Ảnh Live Mới Nhất (/api/session/image/latest)
            </button>
          </div>

          <!-- Khung xem Live Ảnh 3 Ô Trả Về Từ API -->
          <div id="sess-live-image-box" style="display: none; margin-bottom: 14px; background: #fff8fa; border: 1px solid #fce7f3; border-radius: 8px; padding: 14px;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; flex-wrap: wrap; gap: 8px;">
              <div style="font-size: 14px; font-weight: 700; color: #db2777;">
                🖼️ KHUNG XEM ẢNH LIVE (3 Ô: MỚI NHẤT ➔ TRƯỚC ĐÓ ➔ CŨ HƠN)
              </div>
              <div style="font-size: 12px; color: var(--text-muted); font-style: italic;">
                ⚡ Tự động: Ảnh mới nhất vào Ô 1 ➔ đẩy dần sang Ô 2 và Ô 3
              </div>
            </div>

            <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px;">
              <!-- Ô 1 -->
              <div style="background: #ffffff; border: 2px solid #ec4899; border-radius: 8px; padding: 10px; text-align: center; box-shadow: 0 4px 12px rgba(236,72,153,0.12);">
                <div style="font-weight: 700; font-size: 13px; color: #ec4899; margin-bottom: 6px;">🌟 Ô 1: MỚI NHẤT (#1)</div>
                <div style="font-size: 11px; color: #64748b; margin-bottom: 8px; min-height: 28px;" id="slot1-info">Chưa có ảnh</div>
                <img id="slot1-img" src="" alt="Slot 1" style="max-width: 100%; max-height: 180px; border-radius: 6px; border: 1px solid #fbcfe8; display: none;" />
                <div style="margin-top: 8px; display: flex; gap: 6px; justify-content: center;">
                  <a id="slot1-link" href="#" target="_blank" class="btn btn-secondary btn-sm" style="display: none;">Mở to</a>
                  <button id="slot1-copy" class="btn btn-secondary btn-sm" onclick="copySlotURL(0)" style="display: none;">Copy Link</button>
                </div>
              </div>

              <!-- Ô 2 -->
              <div style="background: #ffffff; border: 1px solid #fbcfe8; border-radius: 8px; padding: 10px; text-align: center;">
                <div style="font-weight: 700; font-size: 13px; color: #db2777; margin-bottom: 6px;">⏳ Ô 2: TRƯỚC ĐÓ (#2)</div>
                <div style="font-size: 11px; color: #64748b; margin-bottom: 8px; min-height: 28px;" id="slot2-info">Chưa có ảnh</div>
                <img id="slot2-img" src="" alt="Slot 2" style="max-width: 100%; max-height: 180px; border-radius: 6px; border: 1px solid #fbcfe8; display: none;" />
                <div style="margin-top: 8px; display: flex; gap: 6px; justify-content: center;">
                  <a id="slot2-link" href="#" target="_blank" class="btn btn-secondary btn-sm" style="display: none;">Mở to</a>
                  <button id="slot2-copy" class="btn btn-secondary btn-sm" onclick="copySlotURL(1)" style="display: none;">Copy Link</button>
                </div>
              </div>

              <!-- Ô 3 -->
              <div style="background: #ffffff; border: 1px solid #e2e8f0; border-radius: 8px; padding: 10px; text-align: center;">
                <div style="font-weight: 700; font-size: 13px; color: #64748b; margin-bottom: 6px;">📜 Ô 3: CŨ HƠN (#3)</div>
                <div style="font-size: 11px; color: #64748b; margin-bottom: 8px; min-height: 28px;" id="slot3-info">Chưa có ảnh</div>
                <img id="slot3-img" src="" alt="Slot 3" style="max-width: 100%; max-height: 180px; border-radius: 6px; border: 1px solid #e2e8f0; display: none;" />
                <div style="margin-top: 8px; display: flex; gap: 6px; justify-content: center;">
                  <a id="slot3-link" href="#" target="_blank" class="btn btn-secondary btn-sm" style="display: none;">Mở to</a>
                  <button id="slot3-copy" class="btn btn-secondary btn-sm" onclick="copySlotURL(2)" style="display: none;">Copy Link</button>
                </div>
              </div>
            </div>
          </div>

          <div class="output-container">
            <div class="output-header">
              <span id="session-status">Sẵn sàng</span>
              <button class="btn btn-secondary btn-sm" onclick="copyResult('session-output')">Sao chép</button>
            </div>
            <pre class="code-block" id="session-output">// Kết quả báo cáo tiến độ hoặc danh sách phiên sẽ hiển thị tại đây...</pre>
          </div>
        </div>

        <!-- Bước 5: Kho Ảnh Selfie -->
        <div class="test-box" style="grid-column: 1 / -1; margin-top: 14px; border-left: 4px solid #ec4899;">
          <div style="font-weight: 700; font-size: 15px; margin-bottom: 8px; color: #ec4899;">
            5. Kho Ảnh Selfie (/api/selfies & /api/selfie)
          </div>
          <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 12px;">
            Lấy ảnh selfie phục vụ KYC, avatar, xác minh khuôn mặt. Hỗ trợ xem trực tiếp dạng file ảnh, tải về hoặc trả về JSON / Base64.
          </p>

          <div style="display: flex; gap: 10px; margin-bottom: 12px; flex-wrap: wrap;">
            <button class="btn" onclick="runGetSelfies()">
              <span>📸</span> Xem Danh Sách Ảnh (/api/selfies)
            </button>
            <button class="btn btn-secondary" onclick="runGetRandomSelfie()">
              <span>🎲</span> Lấy 1 Ảnh Ngẫu Nhiên (/api/selfie)
            </button>
            <input type="file" id="selfie-file-input" accept="image/*" style="display: none;" onchange="runUploadSelfieFile(this)" />
            <button class="btn btn-secondary" onclick="document.getElementById('selfie-file-input').click()" style="background: rgba(16, 185, 129, 0.12); border-color: #10b981; color: #059669;">
              <span>📤</span> Tải Ảnh Từ Máy Lên (/api/selfie/upload)
            </button>
          </div>

          <div style="display: flex; gap: 8px; margin-bottom: 12px;">
            <input type="text" class="form-control" id="selfie-url-input" placeholder="Dán link ảnh tại đây (https://.../photo.jpg)" style="flex: 1;" />
            <button class="btn" onclick="runDownloadSelfieFromURL()">
              <span>📥</span> Tải Về Kho
            </button>
          </div>

          <div style="display: flex; gap: 8px; margin-bottom: 12px;">
            <input type="text" class="form-control" id="selfie-delete-input" placeholder="Tên file cần xóa (vd: selfie_sample_01.png)" style="flex: 1;" />
            <button class="btn" onclick="runDeleteSelfie()" style="background: #e11d48; border-color: #e11d48; box-shadow: none;">
              <span>🗑️</span> Xóa Ảnh
            </button>
            <button class="btn btn-secondary" onclick="runClearAllSelfies()" title="Xóa toàn bộ kho ảnh selfie">
              <span>🧹</span> Xóa Hết
            </button>
          </div>

          <div id="selfie-preview-box" style="display: none; margin-bottom: 12px; background: #fff8fa; border: 1px solid #fce7f3; border-radius: 8px; padding: 14px; text-align: center;">
            <div style="font-size: 13px; font-weight: 600; color: #475569; margin-bottom: 8px;" id="selfie-preview-title">Ảnh xem trước</div>
            <img id="selfie-preview-img" src="" alt="Selfie preview" style="max-width: 240px; max-height: 240px; border-radius: 8px; border: 2px solid #ec4899;" />
          </div>

          <div class="output-container">
            <div class="output-header">
              <span id="selfie-status">Sẵn sàng</span>
              <button class="btn btn-secondary btn-sm" onclick="copyResult('selfie-output')">Sao chép</button>
            </div>
            </div>
            <pre class="code-block" id="selfie-output">// Kết quả danh sách ảnh hoặc thông tin ảnh sẽ hiển thị tại đây...</pre>
          </div>
        </div>
      </div>
    </div>

    <!-- ĐẶC TẢ CHI TIẾT CÁC ENDPOINT -->
    <div class="card">
      <div class="card-title">
        <span>📖</span> Danh Mục & Đặc Tả Chi Tiết Các Endpoint
      </div>
      <div class="card-subtitle">Chi tiết cách tích hợp vào các dự án phần mềm và ngôn ngữ lập trình:</div>

      <!-- ENDPOINT 1 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/getemaillist</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/getemaillist')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Lấy danh sách tất cả các tài khoản Gmail hiện đang được cấu hình và sẵn sàng đọc trong ứng dụng desktop.
        </p>

        <table class="param-table">
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td colspan="4" style="text-align: center; color: var(--text-muted); font-style: italic;">
                Không có tham số yêu cầu. Header Authorization không cần thiết.
              </td>
            </tr>
          </tbody>
        </table>

        <div style="margin-top: 14px;">
          <div class="tabs-nav" id="tabs-list">
            <button class="tab-btn active" onclick="switchSnippet('list', 'curl')">cURL</button>
            <button class="tab-btn" onclick="switchSnippet('list', 'py')">Python</button>
            <button class="tab-btn" onclick="switchSnippet('list', 'js')">JavaScript</button>
            <button class="tab-btn" onclick="switchSnippet('list', 'php')">PHP</button>
            <button class="tab-btn" onclick="switchSnippet('list', 'go')">Go</button>
          </div>

          <div id="snippet-list-curl">
            <pre class="code-block">curl http://localhost:` + port + `/api/getemaillist</pre>
          </div>
          <div id="snippet-list-py" style="display: none;">
            <pre class="code-block">import requests

res = requests.get("http://localhost:` + port + `/api/getemaillist")
emails = res.json()
print("Danh sách email:", emails)</pre>
          </div>
          <div id="snippet-list-js" style="display: none;">
            <pre class="code-block">fetch("http://localhost:` + port + `/api/getemaillist")
  .then(res => res.json())
  .then(data => console.log("Danh sách email:", data));</pre>
          </div>
          <div id="snippet-list-php" style="display: none;">
            <pre class="code-block">&lt;?php
$json = file_get_contents("http://localhost:` + port + `/api/getemaillist");
$emails = json_decode($json, true);
print_r($emails);
?&gt;</pre>
          </div>
          <div id="snippet-list-go" style="display: none;">
            <pre class="code-block">resp, err := http.Get("http://localhost:` + port + `/api/getemaillist")
if err == nil {
    defer resp.Body.Close()
    var emails []string
    json.NewDecoder(resp.Body).Decode(&emails)
    fmt.Println("Emails:", emails)
}</pre>
          </div>
        </div>
      </div>

      <!-- ENDPOINT 2 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-post">POST</span>
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/getemail</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/getemail')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Kết nối trực tiếp tới máy chủ IMAP của Google và đọc các thư mới nhất từ hộp thư đến (INBOX) của địa chỉ email được truyền vào.
        </p>

        <table class="param-table">
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Vị trí</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><span class="param-name">email</span></td>
              <td>Body JSON hoặc Query</td>
              <td><span class="param-type">string</span></td>
              <td><span class="param-req">Bắt buộc</span></td>
              <td>Địa chỉ Gmail cần đọc thư (chấp nhận cả alias <code>data</code>)</td>
            </tr>
            <tr>
              <td><span class="param-name">limit</span></td>
              <td>Body JSON hoặc Query</td>
              <td><span class="param-type">integer</span></td>
              <td><span style="color: var(--text-muted);">Tuỳ chọn</span></td>
              <td>Số lượng thư mới nhất cần lấy (Mặc định: 5, Tối đa: 50)</td>
            </tr>
          </tbody>
        </table>

        <div style="margin-top: 14px;">
          <div class="tabs-nav" id="tabs-mail">
            <button class="tab-btn active" onclick="switchSnippet('mail', 'curl-post')">cURL (POST JSON)</button>
            <button class="tab-btn" onclick="switchSnippet('mail', 'curl-get')">cURL (GET Query)</button>
            <button class="tab-btn" onclick="switchSnippet('mail', 'py')">Python</button>
            <button class="tab-btn" onclick="switchSnippet('mail', 'js')">JavaScript</button>
            <button class="tab-btn" onclick="switchSnippet('mail', 'php')">PHP</button>
          </div>

          <div id="snippet-mail-curl-post">
            <pre class="code-block">curl -X POST http://localhost:` + port + `/api/getemail \
  -H "Content-Type: application/json" \
  -d '{"email": "user@gmail.com", "limit": 5}'</pre>
          </div>
          <div id="snippet-mail-curl-get" style="display: none;">
            <pre class="code-block">curl "http://localhost:` + port + `/api/getemail?email=user@gmail.com&limit=5"</pre>
          </div>
          <div id="snippet-mail-py" style="display: none;">
            <pre class="code-block">import requests

payload = {"email": "user@gmail.com", "limit": 5}
res = requests.post("http://localhost:` + port + `/api/getemail", json=payload)
emails = res.json()

for mail in emails:
    print(f"[{mail['date']}] {mail['from']} -> {mail['subject']}")
    print("Nội dung:", mail['body_snippet'][:100], "...\n")</pre>
          </div>
          <div id="snippet-mail-js" style="display: none;">
            <pre class="code-block">const res = await fetch("http://localhost:` + port + `/api/getemail", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email: "user@gmail.com", limit: 5 })
});
const emails = await res.json();
console.log(emails);</pre>
          </div>
          <div id="snippet-mail-php" style="display: none;">
            <pre class="code-block">&lt;?php
$ch = curl_init("http://localhost:` + port + `/api/getemail");
$data = json_encode(["email" => "user@gmail.com", "limit" => 5]);
curl_setopt($ch, CURLOPT_POSTFIELDS, $data);
curl_setopt($ch, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
$response = curl_exec($ch);
curl_close($ch);

$emails = json_decode($response, true);
print_r($emails);
?&gt;</pre>
          </div>
        </div>
      </div>

      <!-- ENDPOINT 3 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/status</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/status')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Kiểm tra tình trạng hoạt động của Gateway, số lượng tài khoản kết nối, IP Tailscale và Public URL.
        </p>
        <pre class="code-block">curl http://localhost:` + port + `/api/status</pre>
      </div>

      <!-- ENDPOINT 4 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/parsers</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/parsers')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Lấy danh sách các bộ Mail Parser đang kích hoạt trong ứng dụng (kèm toàn bộ danh sách quy tắc trích xuất).
        </p>
        <pre class="code-block">curl http://localhost:` + port + `/api/parsers</pre>
      </div>

      <!-- ENDPOINT 5 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-post">POST</span>
          <span class="endpoint-path">/api/parse</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/parse')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Bóc tách văn bản thô theo toàn bộ các quy tắc Mail Parser đã cấu hình trong ứng dụng.
        </p>
        <pre class="code-block">curl -X POST http://localhost:` + port + `/api/parse \
  -H "Content-Type: application/json" \
  -d '{"text": "Mã xác thực của bạn là: 894102. Hết hạn trong 5 phút."}'</pre>
      </div>

      <!-- ENDPOINT 6 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-post">POST</span>
          <span class="endpoint-path">/api/session/report</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/session/report')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Báo cáo hoặc cập nhật tiến độ của một phiên làm việc (session). Hỗ trợ alias <code>/api/progress</code>.
        </p>
        <table class="param-table">
          <thead>
            <tr><th>Trường</th><th>Kiểu</th><th>Mô tả</th></tr>
          </thead>
          <tbody>
            <tr><td><code>id</code></td><td>string</td><td>Mã phiên (tự sinh nếu để trống)</td></tr>
            <tr><td><code>name</code></td><td>string</td><td>Tên công việc / tác vụ</td></tr>
            <tr><td><code>current</code></td><td>integer</td><td>Bước hiện tại (vd: 45)</td></tr>
            <tr><td><code>total</code></td><td>integer</td><td>Tổng số bước (vd: 100)</td></tr>
            <tr><td><code>status</code></td><td>string</td><td>"running", "completed", "failed", "paused"</td></tr>
            <tr><td><code>message</code></td><td>string</td><td>Thông báo log mới nhất</td></tr>
            <tr><td><code>image</code></td><td>string / file</td><td>Ảnh đính kèm: Base64 (data:image/...) hoặc URL hoặc file upload (Multipart)</td></tr>
          </tbody>
        </table>
        <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 8px;">Ví dụ 1: Gửi JSON kèm chuỗi Base64 / URL ảnh:</p>
        <pre class="code-block">curl -X POST http://localhost:` + port + `/api/session/report \
  -H "Content-Type: application/json" \
  -d '{"id": "bot-01", "name": "Đăng ký tài khoản", "current": 45, "total": 100, "message": "Đang đọc mã OTP", "image": "data:image/png;base64,..."}'</pre>
        <p style="font-size: 13px; color: var(--text-muted); margin: 10px 0 8px;">Ví dụ 2: Gửi file ảnh chụp màn hình (Multipart/Form-Data):</p>
        <pre class="code-block">curl -X POST http://localhost:` + port + `/api/session/report \
  -F "id=bot-01" \
  -F "name=Bot Đăng Ký" \
  -F "current=45" \
  -F "total=100" \
  -F "message=Chụp màn hình bước 45" \
  -F "image=@screenshot.png"</pre>
      </div>

      <!-- ENDPOINT 7: LIVE SESSION IMAGE -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/session/image</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/session/image/latest')">Copy URL Mới Nhất</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Xem trực tiếp ảnh live trả về từ API của một phiên cụ thể (<code>?id=...</code>) hoặc ảnh của phiên mới nhất (<code>?latest=1</code> hoặc <code>/api/session/image/latest</code>).
        </p>
        <pre class="code-block"># Xem ảnh phiên cụ thể:
curl http://localhost:` + port + `/api/session/image?id=sess-bot-01

# Xem ảnh live mới nhất của toàn hệ thống:
curl http://localhost:` + port + `/api/session/image/latest</pre>
      </div>

      <!-- ENDPOINT 8: SESSION LIST -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/session/list</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/session/list')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Lấy toàn bộ danh sách các phiên làm việc, tiến độ % chi tiết và đường dẫn ảnh live (<code>imageUrl</code>). Hỗ trợ alias <code>/api/sessions</code>.
        </p>
        <pre class="code-block">curl http://localhost:` + port + `/api/session/list</pre>
      </div>

      <!-- ENDPOINT 8 -->
      <div style="margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/selfies</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/selfies')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Lấy danh sách toàn bộ các file ảnh selfie trong kho (kèm độ phân giải, dung lượng, đường dẫn xem/tải ảnh).
        </p>
        <pre class="code-block">curl http://localhost:` + port + `/api/selfies</pre>
      </div>

      <!-- ENDPOINT 9 -->
      <div>
        <div class="endpoint-header">
          <span class="badge badge-method-get">GET</span>
          <span class="endpoint-path">/api/selfie</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/selfie')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Lấy ngẫu nhiên 1 ảnh selfie (hoặc lấy theo tên bằng <code>?name=...</code>). Mặc định trả về trực tiếp file ảnh (dùng được trong thẻ <code>&lt;img src="..."&gt;</code>). Nếu muốn nhận JSON metadata, thêm <code>?format=json</code> hoặc truyền header <code>Accept: application/json</code>. Thêm <code>&amp;base64=true</code> để nhận chuỗi Data URI.
        </p>
        <pre class="code-block"># Lấy trực tiếp file ảnh:
curl http://localhost:` + port + `/api/selfie -o random_selfie.png

# Lấy thông tin JSON:
curl "http://localhost:` + port + `/api/selfie?format=json"

# Lấy theo tên cụ thể:
curl "http://localhost:` + port + `/api/selfie/file?name=selfie_sample_01.png"</pre>
      </div>

      <!-- ENDPOINT 10 -->
      <div style="margin-top: 30px; margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-post">POST</span>
          <span class="endpoint-path">/api/selfie/download-url</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/selfie/download-url')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Tải ảnh selfie từ một đường link URL bất kỳ trên internet và tự động lưu vào kho selfie của máy chủ. Hỗ trợ alias <code>/api/selfie/download</code>.
        </p>
        <pre class="code-block">curl -X POST http://localhost:` + port + `/api/selfie/download-url \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/photo.jpg", "filename": "avatar_01.jpg"}'</pre>
      </div>

      <!-- ENDPOINT 11 -->
      <div style="margin-top: 30px; margin-bottom: 40px; padding-bottom: 30px; border-bottom: 1px solid var(--card-border);">
        <div class="endpoint-header">
          <span class="badge badge-method-post">POST</span>
          <span class="badge badge-method-get" style="background: rgba(225, 29, 72, 0.12); color: #e11d48; border-color: rgba(225, 29, 72, 0.3);">DELETE</span>
          <span class="endpoint-path">/api/selfie/delete</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/selfie/delete')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Xóa một hoặc nhiều ảnh selfie khỏi kho lưu trữ. Hỗ trợ alias <code>/api/selfie/remove</code>. Chấp nhận phương thức <code>POST</code> hoặc <code>DELETE</code>.
        </p>

        <table class="param-table">
          <thead>
            <tr><th>Tham số</th><th>Vị trí</th><th>Kiểu</th><th>Mô tả</th></tr>
          </thead>
          <tbody>
            <tr><td><code>name</code> / <code>filename</code></td><td>JSON / Query</td><td>string</td><td>Tên file ảnh cần xóa (vd: "selfie_sample_01.png")</td></tr>
            <tr><td><code>names</code> / <code>filenames</code></td><td>JSON</td><td>array[string]</td><td>Danh sách nhiều tên file cần xóa hàng loạt</td></tr>
            <tr><td><code>action</code></td><td>JSON / Query</td><td>string</td><td>Đặt <code>"clear_all"</code> để xóa sạch toàn bộ ảnh trong kho</td></tr>
          </tbody>
        </table>

        <div style="margin-top: 14px;">
          <div class="tabs-nav" id="tabs-delete">
            <button class="tab-btn active" onclick="switchSnippet('delete', 'curl')">cURL</button>
            <button class="tab-btn" onclick="switchSnippet('delete', 'py')">Python</button>
            <button class="tab-btn" onclick="switchSnippet('delete', 'js')">JavaScript</button>
          </div>

          <div id="snippet-delete-curl">
            <pre class="code-block"># 1. Xóa 1 ảnh (POST JSON):
curl -X POST http://localhost:` + port + `/api/selfie/delete \
  -H "Content-Type: application/json" \
  -d '{"name": "photo_01.jpg"}'

# 2. Xóa 1 ảnh (DELETE query):
curl -X DELETE "http://localhost:` + port + `/api/selfie/delete?name=photo_01.jpg"

# 3. Xóa nhiều ảnh cùng lúc:
curl -X POST http://localhost:` + port + `/api/selfie/delete \
  -H "Content-Type: application/json" \
  -d '{"names": ["photo_01.jpg", "photo_02.png"]}'

# 4. Xóa sạch toàn bộ ảnh:
curl -X POST http://localhost:` + port + `/api/selfie/delete \
  -H "Content-Type: application/json" \
  -d '{"action": "clear_all"}'</pre>
          </div>
          <div id="snippet-delete-py" style="display: none;">
            <pre class="code-block">import requests

# Xóa 1 ảnh
res = requests.post("http://localhost:` + port + `/api/selfie/delete", json={"name": "photo_01.jpg"})
print(res.json())

# Xóa nhiều ảnh
res = requests.post("http://localhost:` + port + `/api/selfie/delete", json={"names": ["a.jpg", "b.png"]})
print(res.json())</pre>
          </div>
          <div id="snippet-delete-js" style="display: none;">
            <pre class="code-block">const res = await fetch("http://localhost:` + port + `/api/selfie/delete", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ name: "photo_01.jpg" })
});
const data = await res.json();
console.log(data);</pre>
          </div>
        </div>
      </div>

      <!-- ENDPOINT 12 -->
      <div style="margin-top: 30px;">
        <div class="endpoint-header">
          <span class="badge badge-method-post">POST</span>
          <span class="badge badge-method-post" style="background: #fdf2f8; color: #db2777; border-color: #fbcfe8;">PUT</span>
          <span class="endpoint-path">/api/selfie/upload</span>
          <button class="btn btn-secondary btn-sm" onclick="copyText(window.location.origin + '/api/selfie/upload')">Copy URL</button>
        </div>
        <p style="color: var(--text-muted); margin-bottom: 14px;">
          Thêm hoặc tải ảnh selfie mới trực tiếp lên kho lưu trữ qua API. Hỗ trợ alias <code>/api/selfie/add</code> và <code>/api/selfie/create</code>. Hỗ trợ 3 định dạng linh hoạt: <b>Multipart Form File</b>, <b>Base64 JSON</b> hoặc <b>Raw Binary Data</b>.
        </p>

        <table class="param-table">
          <thead>
            <tr><th>Hình thức</th><th>Định dạng / Trường</th><th>Mô tả</th></tr>
          </thead>
          <tbody>
            <tr><td><b>Multipart Form</b></td><td><code>file</code> hoặc <code>image</code></td><td>Upload trực tiếp file ảnh (.jpg, .png, .webp, .bmp). Hỗ trợ upload nhiều file cùng lúc.</td></tr>
            <tr><td><b>Base64 JSON</b></td><td><code>{"image": "...", "filename": "..."}</code></td><td>Chuỗi Base64 ảnh (hỗ trợ cả Data URI <code>data:image/jpeg;base64,...</code> hoặc chuỗi base64 thuần).</td></tr>
            <tr><td><b>Raw Binary</b></td><td>Body nhị phân + Query <code>?filename=...</code></td><td>Gửi trực tiếp byte ảnh trong POST body kèm tên file tùy chọn.</td></tr>
          </tbody>
        </table>

        <div style="margin-top: 14px;">
          <div class="tabs-nav" id="tabs-upload">
            <button class="tab-btn active" onclick="switchSnippet('upload', 'curl-form')">cURL (File Form)</button>
            <button class="tab-btn" onclick="switchSnippet('upload', 'curl-b64')">cURL (Base64 JSON)</button>
            <button class="tab-btn" onclick="switchSnippet('upload', 'py')">Python (File & Base64)</button>
            <button class="tab-btn" onclick="switchSnippet('upload', 'js')">JavaScript</button>
          </div>

          <div id="snippet-upload-curl-form">
            <pre class="code-block"># 1. Tải lên file ảnh qua Multipart Form:
curl -X POST http://localhost:` + port + `/api/selfie/upload \
  -F "file=@my_selfie.jpg"

# Tải lên nhiều ảnh cùng lúc:
curl -X POST http://localhost:` + port + `/api/selfie/upload \
  -F "file1=@face1.jpg" \
  -F "file2=@face2.png"</pre>
          </div>

          <div id="snippet-upload-curl-b64" style="display: none;">
            <pre class="code-block"># 2. Thêm ảnh qua chuỗi Base64 JSON:
curl -X POST http://localhost:` + port + `/api/selfie/upload \
  -H "Content-Type: application/json" \
  -d '{"image": "data:image/jpeg;base64,/9j/4AAQSkZJRg...", "filename": "selfie_kyc.jpg"}'</pre>
          </div>

          <div id="snippet-upload-py" style="display: none;">
            <pre class="code-block">import requests

SERVER = "http://localhost:` + port + `"

# Cách 1: Upload file ảnh (Multipart)
with open("my_photo.jpg", "rb") as f:
    res = requests.post(f"{SERVER}/api/selfie/upload", files={"file": f})
    print(res.json())

# Cách 2: Upload qua chuỗi Base64
payload = {
    "image": "data:image/png;base64,iVBORw0KGgoAAAANSU...",
    "filename": "avatar_01.png"
}
res = requests.post(f"{SERVER}/api/selfie/upload", json=payload)
print(res.json())</pre>
          </div>

          <div id="snippet-upload-js" style="display: none;">
            <pre class="code-block">// Cách 1: FormData (File upload)
const formData = new FormData();
formData.append("file", fileInput.files[0]);

const res = await fetch("http://localhost:` + port + `/api/selfie/upload", {
  method: "POST",
  body: formData
});
const data = await res.json();
console.log(data);</pre>
          </div>
        </div>
      </div>

    </div>
  </div>

  <script>
    function copyText(txt) {
      navigator.clipboard.writeText(txt).then(function() {
        showToast("Đã sao chép: " + txt);
      });
    }

    function copyResult(id) {
      var el = document.getElementById(id);
      if (el) {
        navigator.clipboard.writeText(el.textContent).then(function() {
          showToast("Đã sao chép nội dung!");
        });
      }
    }

    function showToast(msg) {
      var t = document.getElementById("toast");
      t.textContent = msg;
      t.style.display = "block";
      setTimeout(function() { t.style.display = "none"; }, 2500);
    }

    function switchSnippet(group, lang) {
      var container = document.getElementById("tabs-" + group);
      var btns = container.querySelectorAll(".tab-btn");
      btns.forEach(function(b) { b.classList.remove("active"); });
      event.target.classList.add("active");

      var prefix = "snippet-" + group + "-";
      var allSnippets = document.querySelectorAll("[id^='" + prefix + "']");
      allSnippets.forEach(function(s) { s.style.display = "none"; });

      var target = document.getElementById(prefix + lang);
      if (target) target.style.display = "block";
    }

    // Interactive 1: Get email list
    function runGetEmailList() {
      var statusEl = document.getElementById("list-status");
      var outEl = document.getElementById("list-output");
      statusEl.textContent = "⏳ Đang gọi API...";
      outEl.textContent = "Đang tải dữ liệu...";

      fetch("/api/getemaillist")
        .then(function(res) {
          statusEl.textContent = "Status: " + res.status + " " + res.statusText;
          return res.json();
        })
        .then(function(data) {
          outEl.textContent = JSON.stringify(data, null, 2);
          // Điền vào select box
          var sel = document.getElementById("target-select");
          sel.innerHTML = "";
          if (Array.isArray(data) && data.length > 0) {
            data.forEach(function(em) {
              var opt = document.createElement("option");
              opt.value = em;
              opt.textContent = em;
              sel.appendChild(opt);
            });
            document.getElementById("target-email").value = data[0];
          } else {
            var opt = document.createElement("option");
            opt.value = "";
            opt.textContent = "Chưa có email nào trong app (hãy thêm trong tab 1)";
            sel.appendChild(opt);
          }
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi khi gọi API: " + err;
        });
    }

    function onSelectEmailChange() {
      var sel = document.getElementById("target-select");
      if (sel.value) {
        document.getElementById("target-email").value = sel.value;
      }
    }

    var cachedEmailsData = [];

    // Interactive 2: Get emails
    function runGetEmail(method) {
      var email = document.getElementById("target-email").value.trim();
      var limit = parseInt(document.getElementById("target-limit").value) || 5;
      var statusEl = document.getElementById("email-status");
      var outEl = document.getElementById("email-output");
      var cardsEl = document.getElementById("email-cards-view");

      if (!email) {
        alert("Vui lòng nhập hoặc chọn một địa chỉ Gmail!");
        return;
      }

      statusEl.textContent = "⏳ Đang kết nối IMAP đọc thư từ " + email + "...";
      outEl.textContent = "Đang đọc thư...";

      var url = "/api/getemail";
      var options = {};

      if (method === "POST") {
        options = {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email: email, limit: limit })
        };
      } else {
        url += "?email=" + encodeURIComponent(email) + "&limit=" + limit;
      }

      var tStart = performance.now();
      fetch(url, options)
        .then(function(res) {
          var dur = Math.round(performance.now() - tStart);
          statusEl.textContent = res.status + " " + res.statusText + " (" + dur + "ms)";
          return res.json().then(function(data) {
            return { ok: res.ok, data: data };
          });
        })
        .then(function(resObj) {
          var data = resObj.data;
          cachedEmailsData = Array.isArray(data) ? data : [];
          outEl.textContent = JSON.stringify(data, null, 2);

          if (resObj.ok && Array.isArray(data)) {
            renderEmailCards(data);
            toggleResultView("cards");
          } else {
            toggleResultView("raw");
          }
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi kết nối: " + err;
          outEl.textContent = "Lỗi: " + err;
          toggleResultView("raw");
        });
    }

    function renderEmailCards(list) {
      var cardsEl = document.getElementById("email-cards-view");
      cardsEl.innerHTML = "";

      if (list.length === 0) {
        cardsEl.innerHTML = '<div style="color: var(--text-muted); text-align: center; padding: 20px;">Hộp thư trống hoặc chưa có thư mới.</div>';
        return;
      }

      list.forEach(function(m) {
        var card = document.createElement("div");
        card.className = "email-item-card";

        var header = document.createElement("div");
        header.className = "email-item-header";
        header.innerHTML = '<span class="email-item-from">' + escapeHtml(m.from || "") + '</span><span>' + escapeHtml(m.date || "") + '</span>';

        var subj = document.createElement("div");
        subj.className = "email-item-subject";
        subj.textContent = m.subject || "(Không có tiêu đề)";

        var snip = document.createElement("div");
        snip.className = "email-item-snippet";
        snip.textContent = (m.body_snippet || "(Nội dung trống)").substring(0, 150) + "...";

        card.appendChild(header);
        card.appendChild(subj);
        card.appendChild(snip);

        if (m.parsed && Object.keys(m.parsed).length > 0) {
          var parsedDiv = document.createElement("div");
          parsedDiv.style.marginTop = "8px";
          parsedDiv.style.padding = "6px 10px";
          parsedDiv.style.background = "#fdf2f8";
          parsedDiv.style.border = "1px solid #fbcfe8";
          parsedDiv.style.borderRadius = "6px";
          parsedDiv.style.fontSize = "12px";
          parsedDiv.style.color = "#db2777";
          var parsedList = [];
          for (var k in m.parsed) {
            parsedList.push("<strong>" + escapeHtml(k) + ":</strong> " + escapeHtml(String(m.parsed[k])));
          }
          parsedDiv.innerHTML = "🎯 <strong>Dữ liệu trích xuất (Mail Parser):</strong> [ " + parsedList.join(" | ") + " ]";
          card.appendChild(parsedDiv);
        }

        cardsEl.appendChild(card);
      });
    }

    function runGetParsers() {
      var statusEl = document.getElementById("parser-status");
      var outEl = document.getElementById("parser-output");
      statusEl.textContent = "⏳ Đang lấy danh sách parser...";
      fetch("/api/parsers")
        .then(function(res) { return res.json(); })
        .then(function(data) {
          statusEl.textContent = "✔ 200 OK (" + data.length + " parsers)";
          outEl.textContent = JSON.stringify(data, null, 2);
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function runTestParse() {
      var sample = prompt("Nhập văn bản email mẫu để thử nghiệm bóc tách:", "Mã xác thực OTP của bạn là: 894102. Hết hạn trong 5 phút.");
      if (!sample) return;
      var statusEl = document.getElementById("parser-status");
      var outEl = document.getElementById("parser-output");
      statusEl.textContent = "⏳ Đang bóc tách văn bản...";
      fetch("/api/parse", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text: sample })
      })
        .then(function(res) { return res.json(); })
        .then(function(data) {
          statusEl.textContent = "✔ 200 OK";
          outEl.textContent = JSON.stringify(data, null, 2);
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    // 4. Session Reporting Functions (Hỗ trợ kèm ảnh & Live View 3 Ô)
    var recentImageSlots = [];

    function pushImageToLiveSlots(imgUrl, sessionName, step, sessionId) {
      if (!imgUrl) return;
      var fullUrl = imgUrl.indexOf("http") === 0 ? imgUrl : window.location.origin + imgUrl;
      var now = new Date().toLocaleTimeString();
      var infoText = (sessionName || sessionId) + (step > 0 ? " (Bước " + step + ")" : "") + "\n" + now;

      // Đẩy vào đầu danh sách (Ô 1), các ảnh cũ lùi sang Ô 2 và Ô 3
      recentImageSlots.unshift({
        url: fullUrl,
        info: infoText,
        time: now
      });
      if (recentImageSlots.length > 3) {
        recentImageSlots = recentImageSlots.slice(0, 3);
      }

      renderLiveImageSlots();
    }

    function renderLiveImageSlots() {
      var liveBox = document.getElementById("sess-live-image-box");
      if (recentImageSlots.length > 0) {
        liveBox.style.display = "block";
      }

      for (var i = 1; i <= 3; i++) {
        var item = recentImageSlots[i - 1];
        var imgEl = document.getElementById("slot" + i + "-img");
        var infoEl = document.getElementById("slot" + i + "-info");
        var linkEl = document.getElementById("slot" + i + "-link");
        var copyEl = document.getElementById("slot" + i + "-copy");

        if (item) {
          imgEl.src = item.url + (item.url.indexOf("?") >= 0 ? "&" : "?") + "t=" + Date.now();
          imgEl.style.display = "inline-block";
          infoEl.textContent = item.info;
          linkEl.href = item.url;
          linkEl.style.display = "inline-flex";
          copyEl.style.display = "inline-flex";
        } else {
          imgEl.style.display = "none";
          infoEl.textContent = "Chưa có ảnh ở vị trí này";
          linkEl.style.display = "none";
          copyEl.style.display = "none";
        }
      }
    }

    function copySlotURL(idx) {
      if (idx >= 0 && idx < recentImageSlots.length) {
        copyText(recentImageSlots[idx].url);
      }
    }

    function runReportSession() {
      var id = document.getElementById("sess-input-id").value.trim();
      var name = document.getElementById("sess-input-name").value.trim();
      var current = parseInt(document.getElementById("sess-input-current").value) || 0;
      var total = parseInt(document.getElementById("sess-input-total").value) || 100;
      var msg = document.getElementById("sess-input-msg").value.trim();
      var imgInput = document.getElementById("sess-input-img").value.trim();
      var fileInput = document.getElementById("sess-input-file");

      var statusEl = document.getElementById("session-status");
      var outEl = document.getElementById("session-output");

      statusEl.textContent = "⏳ Đang gửi báo cáo tiến độ...";

      // Nếu có chọn file upload -> Gửi multipart/form-data
      if (fileInput && fileInput.files && fileInput.files.length > 0) {
        var formData = new FormData();
        formData.append("id", id);
        formData.append("name", name);
        formData.append("current", current);
        formData.append("total", total);
        formData.append("message", msg);
        formData.append("image", fileInput.files[0]);

        fetch("/api/session/report", {
          method: "POST",
          body: formData
        })
          .then(function(res) { return res.json(); })
          .then(function(data) {
            handleSessionReportSuccess(data, statusEl, outEl);
            fileInput.value = "";
          })
          .catch(function(err) {
            statusEl.textContent = "❌ Lỗi: " + err;
            outEl.textContent = "Lỗi: " + err;
          });
      } else {
        // Gửi qua JSON Payload
        var payload = {
          id: id,
          name: name,
          current: current,
          total: total,
          message: msg,
          image: imgInput
        };

        fetch("/api/session/report", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload)
        })
          .then(function(res) { return res.json(); })
          .then(function(data) {
            handleSessionReportSuccess(data, statusEl, outEl);
          })
          .catch(function(err) {
            statusEl.textContent = "❌ Lỗi: " + err;
            outEl.textContent = "Lỗi: " + err;
          });
      }
    }

    function handleSessionReportSuccess(data, statusEl, outEl) {
      statusEl.textContent = "✔ Đã cập nhật thành công!";
      outEl.textContent = JSON.stringify(data, null, 2);

      var imgUrl = data.imageUrl || (data.session && data.session.imageUrl);
      if (imgUrl) {
        var sName = data.session ? data.session.name : "";
        var sStep = data.session ? data.session.current : 0;
        var sId = data.session ? data.session.id : "";
        pushImageToLiveSlots(imgUrl, sName, sStep, sId);
      }
    }

    function runViewLatestSessionImage() {
      var statusEl = document.getElementById("session-status");
      statusEl.textContent = "⏳ Đang lấy 3 ảnh live gần nhất...";

      fetch("/api/session/images")
        .then(function(res) { return res.json(); })
        .then(function(items) {
          if (items && items.length > 0) {
            recentImageSlots = [];
            for (var i = 0; i < items.length; i++) {
              recentImageSlots.push({
                url: window.location.origin + "/api/session/image?index=" + i,
                info: items[i].name + "\n" + (items[i].step > 0 ? "Bước " + items[i].step + " | " : "") + items[i].timestamp,
                time: items[i].timestamp
              });
            }
            renderLiveImageSlots();
            statusEl.textContent = "✔ Đã hiển thị " + items.length + " ảnh live gần nhất!";
          } else {
            pushImageToLiveSlots("/api/session/image/latest", "Ảnh Live Mới Nhất", 0, "latest");
            statusEl.textContent = "✔ Đã hiển thị ảnh live mới nhất!";
          }
        })
        .catch(function(err) {
          pushImageToLiveSlots("/api/session/image/latest", "Ảnh Live Mới Nhất", 0, "latest");
        });
    }

    function runGetSessions() {
      var statusEl = document.getElementById("session-status");
      var outEl = document.getElementById("session-output");
      statusEl.textContent = "⏳ Đang lấy danh sách các phiên...";

      fetch("/api/session/list")
        .then(function(res) { return res.json(); })
        .then(function(data) {
          statusEl.textContent = "✔ 200 OK (" + data.length + " phiên)";
          outEl.textContent = JSON.stringify(data, null, 2);
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    // 5. Selfie Functions
    function runGetSelfies() {
      var statusEl = document.getElementById("selfie-status");
      var outEl = document.getElementById("selfie-output");
      var previewBox = document.getElementById("selfie-preview-box");
      statusEl.textContent = "⏳ Đang lấy danh sách ảnh selfie...";

      fetch("/api/selfies")
        .then(function(res) { return res.json(); })
        .then(function(data) {
          statusEl.textContent = "✔ 200 OK (" + data.length + " ảnh)";
          outEl.textContent = JSON.stringify(data, null, 2);

          if (Array.isArray(data) && data.length > 0) {
            previewBox.style.display = "block";
            document.getElementById("selfie-preview-title").textContent = "Ảnh: " + data[0].filename + " (" + data[0].sizeHuman + ")";
            document.getElementById("selfie-preview-img").src = "/api/selfie/file?name=" + encodeURIComponent(data[0].filename);
          }
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function runGetRandomSelfie() {
      var statusEl = document.getElementById("selfie-status");
      var outEl = document.getElementById("selfie-output");
      var previewBox = document.getElementById("selfie-preview-box");
      statusEl.textContent = "⏳ Đang lấy 1 ảnh ngẫu nhiên...";

      // Lấy metadata JSON trước
      fetch("/api/selfie?format=json")
        .then(function(res) { return res.json(); })
        .then(function(item) {
          statusEl.textContent = "✔ 200 OK (Ảnh: " + item.filename + ")";
          outEl.textContent = JSON.stringify(item, null, 2);

          previewBox.style.display = "block";
          document.getElementById("selfie-preview-title").textContent = "Ảnh ngẫu nhiên: " + item.filename + " (" + item.sizeHuman + ")";
          // Thêm timestamp để tránh cache trình duyệt
          document.getElementById("selfie-preview-img").src = "/api/selfie/file?name=" + encodeURIComponent(item.filename) + "&t=" + Date.now();
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function runDownloadSelfieFromURL() {
      var urlInput = document.getElementById("selfie-url-input");
      var rawURL = urlInput.value.trim();
      if (!rawURL) {
        alert("Vui lòng dán link ảnh cần tải!");
        return;
      }
      var statusEl = document.getElementById("selfie-status");
      var outEl = document.getElementById("selfie-output");
      var previewBox = document.getElementById("selfie-preview-box");
      statusEl.textContent = "⏳ Đang tải ảnh từ URL: " + rawURL + "...";

      fetch("/api/selfie/download-url", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: rawURL })
      })
        .then(function(res) { return res.json(); })
        .then(function(resObj) {
          if (resObj.success && resObj.selfie) {
            statusEl.textContent = "✔ Đã tải về kho thành công: " + resObj.selfie.filename;
            outEl.textContent = JSON.stringify(resObj, null, 2);

            previewBox.style.display = "block";
            document.getElementById("selfie-preview-title").textContent = "Ảnh vừa tải: " + resObj.selfie.filename + " (" + resObj.selfie.sizeHuman + ")";
            document.getElementById("selfie-preview-img").src = "/api/selfie/file?name=" + encodeURIComponent(resObj.selfie.filename) + "&t=" + Date.now();
            urlInput.value = "";
          } else {
            statusEl.textContent = "❌ Lỗi: " + (resObj.error || "Không thể tải ảnh");
            outEl.textContent = JSON.stringify(resObj, null, 2);
          }
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi kết nối: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function runDeleteSelfie() {
      var input = document.getElementById("selfie-delete-input");
      var name = input.value.trim();
      if (!name) {
        alert("Vui lòng nhập tên file ảnh cần xóa (vd: selfie_sample_01.png)!");
        return;
      }
      if (!confirm("Bạn có chắc chắn muốn xóa ảnh '" + name + "' không?")) {
        return;
      }
      var statusEl = document.getElementById("selfie-status");
      var outEl = document.getElementById("selfie-output");
      var previewBox = document.getElementById("selfie-preview-box");
      statusEl.textContent = "⏳ Đang gửi yêu cầu xóa ảnh " + name + "...";

      fetch("/api/selfie/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: name })
      })
        .then(function(res) { return res.json().then(function(d) { return { ok: res.ok, data: d }; }); })
        .then(function(resObj) {
          if (resObj.ok && resObj.data.success) {
            statusEl.textContent = "✔ " + resObj.data.message;
            outEl.textContent = JSON.stringify(resObj.data, null, 2);
            previewBox.style.display = "none";
            input.value = "";
          } else {
            statusEl.textContent = "❌ Lỗi: " + (resObj.data.errors ? resObj.data.errors.join(", ") : resObj.data.error || "Không thể xóa");
            outEl.textContent = JSON.stringify(resObj.data, null, 2);
          }
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi kết nối: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function runClearAllSelfies() {
      if (!confirm("⚠️ CẢNH BÁO: Bạn có chắc chắn muốn XÓA TOÀN BỘ tất cả ảnh trong kho selfie không?")) {
        return;
      }
      var statusEl = document.getElementById("selfie-status");
      var outEl = document.getElementById("selfie-output");
      var previewBox = document.getElementById("selfie-preview-box");
      statusEl.textContent = "⏳ Đang xóa toàn bộ kho ảnh...";

      fetch("/api/selfie/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "clear_all" })
      })
        .then(function(res) { return res.json(); })
        .then(function(data) {
          statusEl.textContent = "✔ " + (data.message || "Đã xóa toàn bộ kho ảnh");
          outEl.textContent = JSON.stringify(data, null, 2);
          previewBox.style.display = "none";
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function runUploadSelfieFile(input) {
      if (!input.files || input.files.length === 0) return;
      var file = input.files[0];
      var statusEl = document.getElementById("selfie-status");
      var outEl = document.getElementById("selfie-output");
      var previewBox = document.getElementById("selfie-preview-box");
      statusEl.textContent = "⏳ Đang tải lên ảnh: " + file.name + "...";

      var formData = new FormData();
      formData.append("file", file);

      fetch("/api/selfie/upload", {
        method: "POST",
        body: formData
      })
        .then(function(res) { return res.json(); })
        .then(function(resObj) {
          if (resObj.success && (resObj.selfie || (resObj.selfies && resObj.selfies.length > 0))) {
            var s = resObj.selfie || resObj.selfies[0];
            statusEl.textContent = "✔ Đã tải lên kho thành công: " + s.filename;
            outEl.textContent = JSON.stringify(resObj, null, 2);

            previewBox.style.display = "block";
            document.getElementById("selfie-preview-title").textContent = "Ảnh vừa tải lên: " + s.filename + " (" + s.sizeHuman + ")";
            document.getElementById("selfie-preview-img").src = "/api/selfie/file?name=" + encodeURIComponent(s.filename) + "&t=" + Date.now();
            input.value = "";
          } else {
            statusEl.textContent = "❌ Lỗi: " + (resObj.errors ? resObj.errors.join(", ") : resObj.error || "Lỗi tải ảnh");
            outEl.textContent = JSON.stringify(resObj, null, 2);
          }
        })
        .catch(function(err) {
          statusEl.textContent = "❌ Lỗi kết nối: " + err;
          outEl.textContent = "Lỗi: " + err;
        });
    }

    function toggleResultView(type) {
      var cardsEl = document.getElementById("email-cards-view");
      var rawEl = document.getElementById("email-output");
      if (type === "cards") {
        cardsEl.style.display = "block";
        rawEl.style.display = "none";
      } else {
        cardsEl.style.display = "none";
        rawEl.style.display = "block";
      }
    }

    function escapeHtml(text) {
      var div = document.createElement("div");
      div.textContent = text;
      return div.innerHTML;
    }

    // Tự động tải danh sách email khi mở trang
    window.addEventListener("DOMContentLoaded", function() {
      runGetEmailList();
    });
  </script>
</body>
</html>`
}
