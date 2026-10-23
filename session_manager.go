package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ==================== CẤU TRÚC DỮ LIỆU TIẾN ĐỘ PHIÊN ====================

const sessionImagesDir = "session_images"
const maxLiveImageSlots = 3

type SessionLogItem struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Step      int    `json:"step"`
}

type SessionProgress struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Status       string                 `json:"status"` // "running", "completed", "failed", "paused"
	Current      int                    `json:"current"`
	Total        int                    `json:"total"`
	Percent      float64                `json:"percent"`
	Message      string                 `json:"message"`
	ClientIP     string                 `json:"clientIp"`
	StartedAt    string                 `json:"startedAt"`
	UpdatedAt    string                 `json:"updatedAt"`
	Image        string                 `json:"image,omitempty"`        // Base64 hoặc URL đầu vào từ client
	ImageURL     string                 `json:"imageUrl,omitempty"`     // Cổng HTTP phục vụ ảnh (/api/session/image?id=...)
	ImagePath    string                 `json:"imagePath,omitempty"`    // Đường dẫn lưu trữ file trên ổ cứng
	Data         map[string]interface{} `json:"data,omitempty"`
	Logs         []SessionLogItem       `json:"logs,omitempty"`
	RecentImages []LiveImageItem        `json:"recentImages,omitempty"` // Tối đa 3 ảnh gần nhất của riêng phiên này
}

// Cấu trúc cho mỗi ảnh trong hàng đợi 3 ô
type LiveImageItem struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Step      int    `json:"step"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Path      string `json:"path"`
	ImageURL  string `json:"imageUrl"`
}

var (
	sessionsFile      = "sessions.json"
	recentImagesFile  = "recent_images.json"
	sessionsMap       = make(map[string]*SessionProgress)
	sessionsOrder     []string
	sessionsMu        sync.RWMutex
	sessionListWidget *widget.List
	sessionDetailView *fyne.Container

	// Labels chi tiết phiên đang chọn
	lblDetailSessionName   *widget.Label
	lblDetailSessionStatus *widget.Label
	lblDetailSessionID     *widget.Label
	lblDetailSessionProg   *widget.Label
	lblDetailSessionTime   *widget.Label
	barDetailSession       *widget.ProgressBar
	entryDetailSessionData *widget.Entry
	entryDetailSessionLogs *widget.Entry
	selectedSessionID      string

	// Quản lý hàng đợi 3 ô ảnh live (Mới nhất Ô 1 -> dịch chuyển sang Ô 2 và Ô 3)
	recentImagesMu   sync.RWMutex
	recentImagesList []LiveImageItem

	// 3 Khung Live Image UI
	imgSlots      [maxLiveImageSlots]*canvas.Image
	lblSlotTitles [maxLiveImageSlots]*widget.Label
	lblSlotInfos  [maxLiveImageSlots]*widget.Label
	btnSlotOpens  [maxLiveImageSlots]*widget.Button
	btnSlotCopys  [maxLiveImageSlots]*widget.Button
	slotPaths     [maxLiveImageSlots]string
	lblLiveImageSubtitle *widget.Label
)

// ==================== QUẢN LÝ ẢNH BÁO CÁO ====================

func initSessionImagesDir() {
	_ = os.MkdirAll(sessionImagesDir, 0755)
}

func sessionFileExists(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func sanitizeSessionFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if len(res) > 60 {
		res = res[:60]
	}
	return res
}

func SaveSessionImage(sessionID string, step int, imgInput string, rawBytes []byte) (string, error) {
	initSessionImagesDir()
	safeID := sanitizeSessionFilename(sessionID)
	if safeID == "" {
		safeID = fmt.Sprintf("sess_%d", time.Now().UnixNano()/1e6)
	}

	var data []byte
	ext := ".png"

	if len(rawBytes) > 0 {
		data = rawBytes
		ct := http.DetectContentType(data)
		if strings.Contains(ct, "jpeg") {
			ext = ".jpg"
		} else if strings.Contains(ct, "webp") {
			ext = ".webp"
		} else if strings.Contains(ct, "gif") {
			ext = ".gif"
		}
	} else if strings.HasPrefix(imgInput, "data:image/") {
		parts := strings.SplitN(imgInput, ",", 2)
		if len(parts) == 2 {
			if strings.Contains(parts[0], "jpeg") || strings.Contains(parts[0], "jpg") {
				ext = ".jpg"
			} else if strings.Contains(parts[0], "webp") {
				ext = ".webp"
			} else if strings.Contains(parts[0], "gif") {
				ext = ".gif"
			}
			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err == nil {
				data = decoded
			}
		}
	} else if strings.HasPrefix(imgInput, "http://") || strings.HasPrefix(imgInput, "https://") {
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Get(imgInput)
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			data = b
			ct := http.DetectContentType(data)
			if strings.Contains(ct, "jpeg") {
				ext = ".jpg"
			} else if strings.Contains(ct, "webp") {
				ext = ".webp"
			}
		}
	} else if len(imgInput) > 100 {
		decoded, err := base64.StdEncoding.DecodeString(imgInput)
		if err == nil {
			data = decoded
		}
	}

	if len(data) == 0 {
		return "", fmt.Errorf("không có dữ liệu ảnh hợp lệ")
	}

	// 1. Lưu file theo từng bước nếu có step để bảo toàn lịch sử không bị ghi đè
	var filename string
	if step > 0 {
		filename = fmt.Sprintf("%s_step%d%s", safeID, step, ext)
	} else {
		filename = safeID + ext
	}
	filePath := filepath.Join(sessionImagesDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", err
	}

	// 2. Luôn cập nhật/sao chép ảnh mới nhất vào file đại diện chính <safeID>.<ext>
	repPath := filepath.Join(sessionImagesDir, safeID+ext)
	_ = os.WriteFile(repPath, data, 0644)

	return filePath, nil
}

func createSampleSessionImage(filename string, title string) string {
	initSessionImagesDir()
	outPath := filepath.Join(sessionImagesDir, filename)
	if sessionFileExists(outPath) {
		return outPath
	}
	const w, h = 480, 320
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// Nền trắng hồng nhẹ
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 245, B: 248, A: 255}}, image.Point{}, draw.Src)

	// Khung viền hồng
	for x := 0; x < w; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 236, G: 72, B: 153, A: 255})
			img.Set(x, h-1-y, color.RGBA{R: 236, G: 72, B: 153, A: 255})
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 236, G: 72, B: 153, A: 255})
			img.Set(w-1-x, y, color.RGBA{R: 236, G: 72, B: 153, A: 255})
		}
	}

	// Biểu tượng camera ở giữa
	cx, cy := 240, 160
	r := 60
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, color.RGBA{R: 244, G: 114, B: 182, A: 255})
			}
			if dx*dx+dy*dy <= (r-14)*(r-14) {
				img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
			if dx*dx+dy*dy <= (r-32)*(r-32) {
				img.Set(x, y, color.RGBA{R: 236, G: 72, B: 153, A: 255})
			}
		}
	}

	f, err := os.Create(outPath)
	if err == nil {
		defer f.Close()
		_ = png.Encode(f, img)
	}
	return outPath
}

// ==================== QUẢN LÝ HÀNG ĐỢI 3 Ô ẢNH LIVE ====================

func saveRecentImagesLocked() {
	data, err := json.MarshalIndent(recentImagesList, "", "  ")
	if err == nil {
		_ = os.WriteFile(recentImagesFile, data, 0644)
	}
}

func loadRecentImages() {
	recentImagesMu.Lock()
	defer recentImagesMu.Unlock()

	data, err := os.ReadFile(recentImagesFile)
	if err == nil && len(data) > 0 {
		var list []LiveImageItem
		if json.Unmarshal(data, &list) == nil {
			recentImagesList = nil
			for _, it := range list {
				if sessionFileExists(it.Path) {
					recentImagesList = append(recentImagesList, it)
				}
			}
			if len(recentImagesList) > maxLiveImageSlots {
				recentImagesList = recentImagesList[:maxLiveImageSlots]
			}
		}
	}
}

// Lưu tối đa 3 ảnh gần nhất của RIÊNG phiên này
func pushSessionRecentImage(s *SessionProgress, item LiveImageItem) {
	if s == nil || item.Path == "" {
		return
	}
	// Khử trùng lặp: Nếu bước này đã có trong RecentImages của phiên thì cập nhật tại chỗ
	for i, it := range s.RecentImages {
		if it.Step == item.Step {
			s.RecentImages[i] = item
			return
		}
	}
	// Chèn lên đầu (Ô 1 - mới nhất của phiên này)
	s.RecentImages = append([]LiveImageItem{item}, s.RecentImages...)
	if len(s.RecentImages) > maxLiveImageSlots {
		s.RecentImages = s.RecentImages[:maxLiveImageSlots]
	}
}

func pushGlobalLiveImage(item LiveImageItem) {
	recentImagesMu.Lock()
	defer recentImagesMu.Unlock()

	// Khử trùng lặp: Nếu (SessionID, Step) đã tồn tại thì cập nhật tại chỗ
	for i, it := range recentImagesList {
		if it.SessionID == item.SessionID && it.Step == item.Step {
			recentImagesList[i] = item
			saveRecentImagesLocked()
			return
		}
	}
	// Prepend vào đầu danh sách (Ô 1), tối đa lưu 3 ảnh
	recentImagesList = append([]LiveImageItem{item}, recentImagesList...)
	if len(recentImagesList) > maxLiveImageSlots {
		recentImagesList = recentImagesList[:maxLiveImageSlots]
	}
	saveRecentImagesLocked()
}

// Đẩy ảnh mới vào hàng đợi ảnh toàn cục (kèm timestamp thực tế & khử trùng lặp)
func PushLiveImage(sessionID string, name string, step int, message string, path string, imageURL string, timestamp string) {
	if path == "" || !sessionFileExists(path) {
		return
	}
	if timestamp == "" {
		timestamp = time.Now().Format("15:04:05")
	}

	item := LiveImageItem{
		SessionID: sessionID,
		Name:      name,
		Step:      step,
		Message:   message,
		Timestamp: timestamp,
		Path:      path,
		ImageURL:  imageURL,
	}

	pushGlobalLiveImage(item)
	refreshActiveImageSlots()
}

func GetSessionImages(sessionID string) []LiveImageItem {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()

	var target *SessionProgress
	if sessionID != "" {
		target = sessionsMap[sessionID]
	}
	if target == nil && selectedSessionID != "" {
		target = sessionsMap[selectedSessionID]
	}
	if target == nil && len(sessionsOrder) > 0 {
		target = sessionsMap[sessionsOrder[0]]
	}

	if target != nil {
		res := make([]LiveImageItem, len(target.RecentImages))
		copy(res, target.RecentImages)
		return res
	}
	return nil
}

func GetRecentLiveImages() []LiveImageItem {
	return GetSessionImages("")
}

func GetSessionImagePathByIndex(sessionID string, idx int) string {
	items := GetSessionImages(sessionID)
	if idx >= 0 && idx < len(items) {
		return items[idx].Path
	}
	return ""
}

func GetLiveImagePathByIndex(idx int) string {
	return GetSessionImagePathByIndex("", idx)
}

// Làm mới 3 ô ảnh: TẤT CẢ 3 Ô BẮT BUỘC PHẢI THUỘC VỀ CÙNG 1 PHIÊN
func refreshActiveImageSlots() {
	sessionsMu.RLock()
	var activeItems []LiveImageItem
	var activeSessionName string

	var target *SessionProgress
	if selectedSessionID != "" {
		target = sessionsMap[selectedSessionID]
	}
	if target == nil && len(sessionsOrder) > 0 {
		target = sessionsMap[sessionsOrder[0]]
	}

	if target != nil {
		activeSessionName = target.Name
		activeItems = make([]LiveImageItem, len(target.RecentImages))
		copy(activeItems, target.RecentImages)
	}
	sessionsMu.RUnlock()

	// Hiển thị ảnh của riêng phiên này - TUYỆT ĐỐI KHÔNG mượn ảnh của phiên khác
	renderImageSlots(activeItems, activeSessionName)
}

func renderImageSlots(items []LiveImageItem, sessionTitle string) {
	if lblLiveImageSubtitle != nil {
		if sessionTitle != "" {
			lblLiveImageSubtitle.SetText(fmt.Sprintf("⚡ 3 ảnh của phiên: %s", sessionTitle))
		} else {
			lblLiveImageSubtitle.SetText("⚡ Luồng Live: Ảnh mới nhất vào Ô 1 ➔ đẩy sang Ô 2 và Ô 3")
		}
	}

	slotNames := []string{
		"🌟 Ô 1: ẢNH MỚI NHẤT (#1)",
		"⏳ Ô 2: ẢNH TRƯỚC ĐÓ (#2)",
		"📜 Ô 3: ẢNH CŨ HƠN (#3)",
	}

	for i := 0; i < maxLiveImageSlots; i++ {
		if imgSlots[i] == nil {
			continue
		}

		if i < len(items) && items[i].Path != "" && sessionFileExists(items[i].Path) {
			slotPaths[i] = items[i].Path
			imgSlots[i].File = items[i].Path
			imgSlots[i].Resource = nil
			imgSlots[i].Refresh()

			if lblSlotTitles[i] != nil {
				lblSlotTitles[i].SetText(slotNames[i])
			}
			if lblSlotInfos[i] != nil {
				fi, _ := os.Stat(items[i].Path)
				sizeStr := ""
				if fi != nil {
					sizeStr = formatFileSize(fi.Size())
				}
				stepInfo := ""
				if items[i].Step > 0 {
					stepInfo = fmt.Sprintf("Bước %d | ", items[i].Step)
				}
				displayName := items[i].Name
				if displayName == "" && sessionTitle != "" {
					displayName = sessionTitle
				}
				lblSlotInfos[i].SetText(fmt.Sprintf("%s\n%s%s | %s", displayName, stepInfo, items[i].Timestamp, sizeStr))
			}
			if btnSlotOpens[i] != nil {
				btnSlotOpens[i].Enable()
			}
			if btnSlotCopys[i] != nil {
				btnSlotCopys[i].Enable()
			}
		} else {
			slotPaths[i] = ""
			imgSlots[i].File = ""
			imgSlots[i].Resource = theme.FileImageIcon()
			imgSlots[i].Refresh()

			if lblSlotTitles[i] != nil {
				lblSlotTitles[i].SetText(slotNames[i])
			}
			if lblSlotInfos[i] != nil {
				if sessionTitle != "" {
					lblSlotInfos[i].SetText("Chưa có ảnh ở bước này (Cùng phiên)")
				} else {
					lblSlotInfos[i].SetText("Chưa có ảnh ở vị trí này")
				}
			}
			if btnSlotOpens[i] != nil {
				btnSlotOpens[i].Disable()
			}
			if btnSlotCopys[i] != nil {
				btnSlotCopys[i].Disable()
			}
		}
	}
}

func updateAllLiveImageSlots() {
	refreshActiveImageSlots()
}

// ==================== TẢI & LƯU TRỮ PHIÊN ====================

func loadSessions() {
	initSessionImagesDir()
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	data, err := os.ReadFile(sessionsFile)
	if err == nil && len(data) > 0 {
		var list []SessionProgress
		if json.Unmarshal(data, &list) == nil {
			sessionsMap = make(map[string]*SessionProgress)
			sessionsOrder = nil
			for i := range list {
				s := list[i]
				// Kiểm tra ảnh trên đĩa nếu ImagePath rỗng
				if s.ImagePath == "" {
					safeID := sanitizeSessionFilename(s.ID)
					for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
						p := filepath.Join(sessionImagesDir, safeID+ext)
						if sessionFileExists(p) {
							s.ImagePath = p
							s.ImageURL = "/api/session/image?id=" + s.ID
							break
						}
					}
				}

				// Nếu phiên chưa có RecentImages nhưng có ảnh, nạp ảnh vào RecentImages của phiên
				if len(s.RecentImages) == 0 && s.ImagePath != "" && sessionFileExists(s.ImagePath) {
					ts := ""
					timeParts := strings.Split(s.UpdatedAt, " ")
					if len(timeParts) > 1 {
						ts = timeParts[1]
					} else {
						ts = s.UpdatedAt
					}
					s.RecentImages = []LiveImageItem{
						{
							SessionID: s.ID,
							Name:      s.Name,
							Step:      s.Current,
							Message:   s.Message,
							Timestamp: ts,
							Path:      s.ImagePath,
							ImageURL:  s.ImageURL,
						},
					}
				}

				sessionsMap[s.ID] = &s
				sessionsOrder = append(sessionsOrder, s.ID)
			}

			// Tải hàng đợi 3 ô từ file recent_images.json độc lập
			loadRecentImages()

			// Nếu recent_images.json chưa có, nạp đúng thứ tự thời gian từ các phiên mới nhất
			recentImagesMu.Lock()
			if len(recentImagesList) == 0 {
				for _, id := range sessionsOrder {
					sess := sessionsMap[id]
					if sess != nil && len(sess.RecentImages) > 0 {
						for _, it := range sess.RecentImages {
							if sessionFileExists(it.Path) {
								recentImagesList = append(recentImagesList, it)
								if len(recentImagesList) >= maxLiveImageSlots {
									break
								}
							}
						}
					}
					if len(recentImagesList) >= maxLiveImageSlots {
						break
					}
				}
				saveRecentImagesLocked()
			}
			recentImagesMu.Unlock()
			return
		}
	}

	// Mẫu phiên khởi đầu nếu file rỗng
	demoImgPath := createSampleSessionImage("demo-session-01.png", "System Ready")
	demoSession := SessionProgress{
		ID:        "demo-session-01",
		Name:      "Phiên mẫu: Khởi động hệ thống Gateway",
		Status:    "completed",
		Current:   100,
		Total:     100,
		Percent:   100.0,
		Message:   "Hệ thống đã sẵn sàng nhận báo cáo tiến độ và ảnh từ các bot/tool ngoài.",
		ClientIP:  "127.0.0.1",
		StartedAt: time.Now().Add(-10 * time.Minute).Format("2006-01-02 15:04:05"),
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		ImageURL:  "/api/session/image?id=demo-session-01",
		ImagePath: demoImgPath,
		Logs: []SessionLogItem{
			{Timestamp: time.Now().Add(-10 * time.Minute).Format("15:04:05"), Step: 1, Message: "Khởi tạo tiến trình"},
			{Timestamp: time.Now().Format("15:04:05"), Step: 100, Message: "Hoàn tất kiểm tra camera / màn hình"},
		},
		RecentImages: []LiveImageItem{
			{
				SessionID: "demo-session-01",
				Name:      "Phiên mẫu: Khởi động hệ thống Gateway",
				Step:      100,
				Message:   "Hoàn tất kiểm tra camera / màn hình",
				Timestamp: "15:04:05",
				Path:      demoImgPath,
				ImageURL:  "/api/session/image?id=demo-session-01",
			},
		},
	}
	sessionsMap[demoSession.ID] = &demoSession
	sessionsOrder = append(sessionsOrder, demoSession.ID)

	PushLiveImage(demoSession.ID, demoSession.Name, demoSession.Current, demoSession.Message, demoImgPath, demoSession.ImageURL, "15:04:05")

	saveSessionsLocked()
}

func saveSessions() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	saveSessionsLocked()
}

func saveSessionsLocked() {
	var list []SessionProgress
	for _, id := range sessionsOrder {
		if s, ok := sessionsMap[id]; ok {
			list = append(list, *s)
		}
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err == nil {
		_ = os.WriteFile(sessionsFile, data, 0644)
	}
}

// ==================== THAO TÁC QUẢN LÝ TIẾN ĐỘ ====================

func CreateOrUpdateSession(req SessionProgress, clientIP string) *SessionProgress {
	sessionsMu.Lock()

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = fmt.Sprintf("sess-%d", time.Now().UnixNano()/1000000)
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	nowTimeOnly := time.Now().Format("15:04:05")

	s, exists := sessionsMap[id]
	if !exists {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "Phiên làm việc " + id
		}
		s = &SessionProgress{
			ID:        id,
			Name:      name,
			StartedAt: nowStr,
			ClientIP:  clientIP,
			Status:    "running",
		}
		sessionsMap[id] = s
		// Thêm vào đầu danh sách hiển thị
		sessionsOrder = append([]string{id}, sessionsOrder...)
	}

	if req.Name != "" {
		s.Name = req.Name
	}
	if clientIP != "" {
		s.ClientIP = clientIP
	}

	s.Current = req.Current
	if req.Total > 0 {
		s.Total = req.Total
	}

	// Tính toán phần trăm tiến độ
	if s.Total > 0 {
		pct := (float64(s.Current) / float64(s.Total)) * 100.0
		if pct < 0 {
			pct = 0
		} else if pct > 100 {
			pct = 100
		}
		s.Percent = pct
	} else if req.Percent > 0 {
		s.Percent = req.Percent
	}

	// Xác định trạng thái
	if req.Status != "" {
		s.Status = strings.ToLower(req.Status)
	} else if s.Total > 0 && s.Current >= s.Total {
		s.Status = "completed"
	}

	if req.Message != "" {
		s.Message = req.Message
		s.Logs = append(s.Logs, SessionLogItem{
			Timestamp: nowTimeOnly,
			Step:      s.Current,
			Message:   req.Message,
		})
		if len(s.Logs) > 60 {
			s.Logs = s.Logs[len(s.Logs)-60:]
		}
	}

	if req.Data != nil {
		s.Data = req.Data
	}

	// Xử lý ảnh đính kèm từ request
	var incomingImagePath string
	if req.ImagePath != "" && sessionFileExists(req.ImagePath) {
		incomingImagePath = req.ImagePath
	} else if req.Image != "" {
		filePath, err := SaveSessionImage(s.ID, s.Current, req.Image, nil)
		if err == nil && filePath != "" {
			incomingImagePath = filePath
		}
	}

	if incomingImagePath != "" {
		s.ImagePath = incomingImagePath
		s.ImageURL = "/api/session/image?id=" + s.ID
		s.Image = s.ImageURL

		liveItem := LiveImageItem{
			SessionID: s.ID,
			Name:      s.Name,
			Step:      s.Current,
			Message:   s.Message,
			Timestamp: nowTimeOnly,
			Path:      incomingImagePath,
			ImageURL:  s.ImageURL,
		}

		// 1. Lưu vào danh sách 3 ảnh của RIÊNG phiên này (bảo toàn lịch sử các bước trong phiên)
		pushSessionRecentImage(s, liveItem)

		// 2. Đẩy vào hàng đợi 3 ảnh toàn cục
		pushGlobalLiveImage(liveItem)
	}

	s.UpdatedAt = nowStr

	// Giới hạn số lượng phiên trong bộ nhớ (tối đa 150 phiên)
	if len(sessionsOrder) > 150 {
		removeID := sessionsOrder[len(sessionsOrder)-1]
		delete(sessionsMap, removeID)
		sessionsOrder = sessionsOrder[:len(sessionsOrder)-1]
	}

	saveSessionsLocked()
	sessionsMu.Unlock()

	// Làm mới giao diện Desktop
	refreshSessionUI()

	return s
}

func GetSessionList() []SessionProgress {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()

	var list []SessionProgress
	for _, id := range sessionsOrder {
		if s, ok := sessionsMap[id]; ok {
			list = append(list, *s)
		}
	}
	return list
}

func GetSessionByID(id string) (*SessionProgress, bool) {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()

	s, ok := sessionsMap[id]
	if !ok {
		return nil, false
	}
	copyS := *s
	return &copyS, true
}

func DeleteSession(id string) bool {
	sessionsMu.Lock()

	s, ok := sessionsMap[id]
	if !ok {
		sessionsMu.Unlock()
		return false
	}
	for _, it := range s.RecentImages {
		if it.Path != "" && sessionFileExists(it.Path) {
			_ = os.Remove(it.Path)
		}
	}
	if s.ImagePath != "" && sessionFileExists(s.ImagePath) {
		_ = os.Remove(s.ImagePath)
	}
	delete(sessionsMap, id)

	var newOrder []string
	for _, oid := range sessionsOrder {
		if oid != id {
			newOrder = append(newOrder, oid)
		}
	}
	sessionsOrder = newOrder
	saveSessionsLocked()
	sessionsMu.Unlock()

	// Dọn khỏi recentImagesList
	recentImagesMu.Lock()
	var newRecent []LiveImageItem
	for _, it := range recentImagesList {
		if it.SessionID != id {
			newRecent = append(newRecent, it)
		}
	}
	recentImagesList = newRecent
	saveRecentImagesLocked()
	recentImagesMu.Unlock()

	refreshSessionUI()
	return true
}

func ClearCompletedSessions() int {
	sessionsMu.Lock()

	var newOrder []string
	var deletedIDs []string
	cleared := 0
	for _, id := range sessionsOrder {
		s := sessionsMap[id]
		if s != nil && (s.Status == "completed" || s.Status == "failed") {
			for _, it := range s.RecentImages {
				if it.Path != "" && sessionFileExists(it.Path) {
					_ = os.Remove(it.Path)
				}
			}
			if s.ImagePath != "" && sessionFileExists(s.ImagePath) {
				_ = os.Remove(s.ImagePath)
			}
			delete(sessionsMap, id)
			deletedIDs = append(deletedIDs, id)
			cleared++
		} else {
			newOrder = append(newOrder, id)
		}
	}
	sessionsOrder = newOrder
	saveSessionsLocked()
	sessionsMu.Unlock()

	if len(deletedIDs) > 0 {
		recentImagesMu.Lock()
		var newRecent []LiveImageItem
		for _, it := range recentImagesList {
			del := false
			for _, did := range deletedIDs {
				if it.SessionID == did {
					del = true
					break
				}
			}
			if !del {
				newRecent = append(newRecent, it)
			}
		}
		recentImagesList = newRecent
		saveRecentImagesLocked()
		recentImagesMu.Unlock()
	}

	refreshSessionUI()
	return cleared
}

func refreshSessionUI() {
	if sessionListWidget != nil {
		sessionListWidget.Refresh()
	}

	sessionsMu.RLock()
	defer sessionsMu.RUnlock()

	if selectedSessionID != "" {
		s, ok := sessionsMap[selectedSessionID]
		if ok && s != nil {
			loadSessionDetail(s)
		}
	} else if len(sessionsOrder) > 0 {
		firstID := sessionsOrder[0]
		if s, ok := sessionsMap[firstID]; ok && s != nil {
			loadSessionDetail(s)
		}
	}
}

// ==================== TẠO KHUNG ĐƠN CHO TỪNG Ô TRONG 3 Ô ====================

func createSingleImageSlot(index int, title string, w fyne.Window) fyne.CanvasObject {
	imgSlots[index] = canvas.NewImageFromResource(theme.FileImageIcon())
	imgSlots[index].FillMode = canvas.ImageFillContain
	imgSlots[index].SetMinSize(fyne.NewSize(160, 110))

	lblSlotTitles[index] = widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	lblSlotInfos[index] = widget.NewLabel("Chưa có ảnh ở ô này")
	lblSlotInfos[index].Alignment = fyne.TextAlignCenter
	lblSlotInfos[index].Wrapping = fyne.TextWrapWord
	lblSlotInfos[index].TextStyle.Italic = true

	idx := index
	btnSlotOpens[index] = widget.NewButtonWithIcon("Mở to", theme.NavigateNextIcon(), func() {
		p := slotPaths[idx]
		if p != "" && sessionFileExists(p) {
			absPath, err := filepath.Abs(p)
			if err == nil {
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", absPath).Start()
			}
		}
	})
	btnSlotOpens[index].Disable()

	btnSlotCopys[index] = widget.NewButtonWithIcon("Copy Base64", theme.ContentPasteIcon(), func() {
		p := slotPaths[idx]
		if p != "" && sessionFileExists(p) {
			data, err := os.ReadFile(p)
			if err == nil {
				b64 := base64.StdEncoding.EncodeToString(data)
				w.Clipboard().SetContent(fmt.Sprintf("data:image/png;base64,%s", b64))
				dialog.ShowInformation("Đã sao chép", fmt.Sprintf("Đã chép Base64 Data-URI của Ô %d vào bộ nhớ tạm!", idx+1), w)
			}
		}
	})
	btnSlotCopys[index].Disable()

	btnRow := container.NewGridWithColumns(2, btnSlotOpens[index], btnSlotCopys[index])

	cardHeader := container.NewVBox(
		lblSlotTitles[index],
		lblSlotInfos[index],
		widget.NewSeparator(),
	)

	return container.NewBorder(
		cardHeader,
		btnRow,
		nil, nil,
		container.NewPadded(imgSlots[index]),
	)
}

// ==================== GIAO DIỆN TAB TIẾN ĐỘ PHIÊN (DESKTOP UI) ====================

func buildSessionTab(w fyne.Window) fyne.CanvasObject {
	// Danh sách phiên bên trái
	sessionListWidget = widget.NewList(
		func() int {
			sessionsMu.RLock()
			defer sessionsMu.RUnlock()
			return len(sessionsOrder)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.DocumentIcon())

			nameLbl := widget.NewLabelWithStyle("Tên Phiên", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			nameLbl.Truncation = fyne.TextTruncateEllipsis

			statusBadge := widget.NewLabelWithStyle("[Đang chạy]", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})

			progLbl := widget.NewLabel("0% (0/0)")
			progLbl.TextStyle.Italic = true
			progLbl.Truncation = fyne.TextTruncateEllipsis

			timeLbl := widget.NewLabel("00:00:00")

			topRow := container.NewBorder(nil, nil, nil, statusBadge, nameLbl)
			midRow := container.NewBorder(nil, nil, nil, timeLbl, progLbl)

			vbox := container.NewVBox(topRow, midRow)
			card := container.NewBorder(nil, nil, icon, nil, vbox)
			return container.NewPadded(card)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			sessionsMu.RLock()
			if id < 0 || id >= len(sessionsOrder) {
				sessionsMu.RUnlock()
				return
			}
			sID := sessionsOrder[id]
			s, ok := sessionsMap[sID]
			sessionsMu.RUnlock()
			if !ok || s == nil {
				return
			}

			padded, ok := obj.(*fyne.Container)
			if !ok || len(padded.Objects) == 0 {
				return
			}
			card, ok := padded.Objects[0].(*fyne.Container)
			if !ok || len(card.Objects) < 2 {
				return
			}
			vbox, ok := card.Objects[0].(*fyne.Container)
			icon, _ := card.Objects[1].(*widget.Icon)
			if !ok || len(vbox.Objects) < 2 {
				return
			}

			topRow, okTop := vbox.Objects[0].(*fyne.Container)
			midRow, okMid := vbox.Objects[1].(*fyne.Container)
			if !okTop || !okMid || len(topRow.Objects) < 2 || len(midRow.Objects) < 2 {
				return
			}

			nameLbl, _ := topRow.Objects[0].(*widget.Label)
			statusBadge, _ := topRow.Objects[1].(*widget.Label)

			progLbl, _ := midRow.Objects[0].(*widget.Label)
			timeLbl, _ := midRow.Objects[1].(*widget.Label)

			if nameLbl != nil {
				nameLbl.SetText(s.Name)
			}

			if statusBadge != nil {
				switch strings.ToLower(s.Status) {
				case "completed":
					statusBadge.SetText("✓ Hoàn thành")
					if icon != nil {
						icon.SetResource(theme.ConfirmIcon())
					}
				case "failed":
					statusBadge.SetText("✖ Thất bại")
					if icon != nil {
						icon.SetResource(theme.ErrorIcon())
					}
				case "paused":
					statusBadge.SetText("⏸ Tạm dừng")
					if icon != nil {
						icon.SetResource(theme.MediaPauseIcon())
					}
				default:
					statusBadge.SetText("▶ Đang chạy")
					if icon != nil {
						icon.SetResource(theme.MediaPlayIcon())
					}
				}
			}

			if progLbl != nil {
				if s.Total > 0 {
					progLbl.SetText(fmt.Sprintf("%.1f%% (%d/%d)", s.Percent, s.Current, s.Total))
				} else {
					progLbl.SetText(fmt.Sprintf("Bước: %d", s.Current))
				}
			}

			if timeLbl != nil {
				timeParts := strings.Split(s.UpdatedAt, " ")
				if len(timeParts) > 1 {
					timeLbl.SetText(timeParts[1])
				} else {
					timeLbl.SetText(s.UpdatedAt)
				}
			}
		},
	)

	sessionListWidget.OnSelected = func(id widget.ListItemID) {
		sessionsMu.RLock()
		if id < 0 || id >= len(sessionsOrder) {
			sessionsMu.RUnlock()
			return
		}
		sID := sessionsOrder[id]
		s := sessionsMap[sID]
		sessionsMu.RUnlock()

		if s != nil {
			selectedSessionID = sID
			loadSessionDetail(s)
		}
	}

	// Toolbar bên trái
	btnClearCompleted := widget.NewButtonWithIcon("Dọn phiên xong", theme.DeleteIcon(), func() {
		cleared := ClearCompletedSessions()
		dialog.ShowInformation("Thông báo", fmt.Sprintf("Đã dọn dẹp %d phiên đã hoàn thành/thất bại.", cleared), w)
	})

	btnRefreshList := widget.NewButtonWithIcon("Làm mới", theme.ViewRefreshIcon(), func() {
		sessionListWidget.Refresh()
		updateAllLiveImageSlots()
	})

	leftHeader := container.NewVBox(
		widget.NewLabelWithStyle("📊 TIẾN ĐỘ PHIÊN", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, btnRefreshList, btnClearCompleted),
		widget.NewSeparator(),
	)

	leftPanel := container.NewBorder(
		leftHeader,
		nil, nil, nil,
		sessionListWidget,
	)

	// ==================== BÊN PHẢI: CHI TIẾT & NHẬT KÝ CHIẾM TOÀN BỘ MÀN HÌNH ====================
	lblDetailSessionName = widget.NewLabelWithStyle("Chọn một phiên bên trái để xem tiến độ chi tiết", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	lblDetailSessionStatus = widget.NewLabel("")
	lblDetailSessionID = widget.NewLabel("")
	lblDetailSessionProg = widget.NewLabel("")
	lblDetailSessionTime = widget.NewLabel("")
	barDetailSession = widget.NewProgressBar()
	barDetailSession.SetValue(0)

	// Khởi tạo các khung nhập liệu / nhật ký
	entryDetailSessionLogs = widget.NewMultiLineEntry()
	entryDetailSessionLogs.Wrapping = fyne.TextWrapWord
	entryDetailSessionLogs.TextStyle = fyne.TextStyle{Monospace: true}
	entryDetailSessionLogs.Disable()

	entryDetailSessionData = widget.NewMultiLineEntry()
	entryDetailSessionData.Wrapping = fyne.TextWrapWord
	entryDetailSessionData.TextStyle = fyne.TextStyle{Monospace: true}
	entryDetailSessionData.Disable()

	btnDeleteSelected := widget.NewButtonWithIcon("Xóa Phiên Này", theme.DeleteIcon(), func() {
		if selectedSessionID == "" {
			return
		}
		dialog.ShowConfirm("Xác nhận", "Bạn có chắc chắn muốn xóa phiên đang chọn?", func(ok bool) {
			if ok {
				DeleteSession(selectedSessionID)
				selectedSessionID = ""
				clearSessionDetail()
			}
		}, w)
	})
	btnDeleteSelected.Importance = widget.DangerImportance

	lblDetailSessionName.Truncation = fyne.TextTruncateEllipsis
	lblDetailSessionID.Truncation = fyne.TextTruncateEllipsis
	lblDetailSessionProg.Truncation = fyne.TextTruncateEllipsis

	rowDetail1 := container.NewBorder(nil, nil, nil, lblDetailSessionStatus, lblDetailSessionName)
	rowDetail2 := container.NewBorder(nil, nil, nil, lblDetailSessionTime, lblDetailSessionID)
	rowDetail3 := container.NewBorder(nil, nil, nil, btnDeleteSelected, lblDetailSessionProg)

	infoCard := container.NewVBox(
		rowDetail1,
		rowDetail2,
		rowDetail3,
		barDetailSession,
		widget.NewSeparator(),
	)

	// Khung Nhật Ký Tiến Trình chiếm toàn bộ bề ngang
	logsBox := container.NewBorder(
		nil, nil, nil, nil,
		entryDetailSessionLogs,
	)

	dataBox := container.NewBorder(
		nil, nil, nil, nil,
		entryDetailSessionData,
	)

	upperTabs := container.NewAppTabs(
		container.NewTabItemWithIcon("📜 Nhật Ký Tiến Trình Chi Tiết (Realtime Logs)", theme.DocumentIcon(), logsBox),
		container.NewTabItemWithIcon("📦 Dữ Liệu Payload (JSON)", theme.FolderIcon(), dataBox),
	)
	upperTabs.SetTabLocation(container.TabLocationTop)

	upperSection := container.NewBorder(
		infoCard,
		nil, nil, nil,
		upperTabs,
	)

	// ==================== PHẦN DƯỚI: KHUNG XEM ẢNH CHIA THÀNH 3 Ô ====================
	slot1 := createSingleImageSlot(0, "🌟 Ô 1: ẢNH MỚI NHẤT (#1)", w)
	slot2 := createSingleImageSlot(1, "⏳ Ô 2: ẢNH TRƯỚC ĐÓ (#2)", w)
	slot3 := createSingleImageSlot(2, "📜 Ô 3: ẢNH CŨ HƠN (#3)", w)

	grid3Slots := container.NewGridWithColumns(3, slot1, slot2, slot3)

	hdrTitle := widget.NewLabelWithStyle("🖼️ KHUNG XEM ẢNH (3 ẢNH GẦN NHẤT)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	lblLiveImageSubtitle = widget.NewLabelWithStyle("⚡ Tự động: Ảnh mới nhất vào Ô 1 ➔ đẩy dần sang Ô 2 và Ô 3", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true})
	lblLiveImageSubtitle.Truncation = fyne.TextTruncateEllipsis
	bottomHeader := container.NewBorder(nil, nil, nil, lblLiveImageSubtitle, hdrTitle)

	bottomImagesBox := container.NewBorder(
		container.NewVBox(bottomHeader, widget.NewSeparator()),
		nil, nil, nil,
		grid3Slots,
	)

	// Chia trên / dưới: Nhật ký chiếm toàn bộ phần trên, 3 ô ảnh ở dưới
	splitUpperLower := container.NewVSplit(upperSection, bottomImagesBox)
	splitUpperLower.SetOffset(0.50) // 50% nhật ký bên trên, 50% 3 ô ảnh bên dưới

	sessionDetailView = container.NewBorder(
		nil, nil, nil, nil,
		splitUpperLower,
	)

	mainSplit := container.NewHSplit(leftPanel, sessionDetailView)
	mainSplit.SetOffset(0.30)

	// Khởi tạo hiển thị cho 3 ô ảnh
	updateAllLiveImageSlots()

	// Chọn phiên đầu tiên nếu có
	sessionsMu.RLock()
	hasSessions := len(sessionsOrder) > 0
	sessionsMu.RUnlock()
	if hasSessions {
		sessionListWidget.Select(0)
	}

	return mainSplit
}

func loadSessionDetail(s *SessionProgress) {
	if s == nil {
		return
	}
	selectedSessionID = s.ID
	if lblDetailSessionName != nil {
		lblDetailSessionName.SetText(fmt.Sprintf("📌 %s", s.Name))
	}
	if lblDetailSessionID != nil {
		lblDetailSessionID.SetText(fmt.Sprintf("Mã ID: %s | Client IP: %s", s.ID, s.ClientIP))
	}
	if lblDetailSessionTime != nil {
		lblDetailSessionTime.SetText(fmt.Sprintf("Bắt đầu: %s | Cập nhật: %s", s.StartedAt, s.UpdatedAt))
	}

	if lblDetailSessionStatus != nil {
		switch strings.ToLower(s.Status) {
		case "completed":
			lblDetailSessionStatus.SetText("🟢 TRẠNG THÁI: HOÀN THÀNH")
		case "failed":
			lblDetailSessionStatus.SetText("🔴 TRẠNG THÁI: THẤT BÀI")
		case "paused":
			lblDetailSessionStatus.SetText("🟡 TRẠNG THÁI: TẠM DỪNG")
		default:
			lblDetailSessionStatus.SetText("🔵 TRẠNG THÁI: ĐANG CHẠY")
		}
	}

	if barDetailSession != nil {
		if s.Total > 0 {
			if lblDetailSessionProg != nil {
				lblDetailSessionProg.SetText(fmt.Sprintf("Tiến độ: %d / %d (%.1f%%) — %s", s.Current, s.Total, s.Percent, s.Message))
			}
			barDetailSession.SetValue(s.Percent / 100.0)
		} else {
			if lblDetailSessionProg != nil {
				lblDetailSessionProg.SetText(fmt.Sprintf("Bước hiện tại: %d — %s", s.Current, s.Message))
			}
			barDetailSession.SetValue(1.0)
		}
	}

	// Hiển thị 3 ảnh gần nhất của riêng phiên đang chọn
	refreshActiveImageSlots()

	// Hiển thị logs
	if entryDetailSessionLogs != nil {
		var logLines []string
		for _, l := range s.Logs {
			logLines = append(logLines, fmt.Sprintf("[%s] (Bước %d) %s", l.Timestamp, l.Step, l.Message))
		}
		if len(logLines) == 0 && s.Message != "" {
			logLines = append(logLines, fmt.Sprintf("[%s] %s", s.UpdatedAt, s.Message))
		}
		entryDetailSessionLogs.SetText(strings.Join(logLines, "\n"))
	}

	// Hiển thị data
	if entryDetailSessionData != nil {
		if s.Data != nil && len(s.Data) > 0 {
			b, _ := json.MarshalIndent(s.Data, "", "  ")
			entryDetailSessionData.SetText(string(b))
		} else {
			entryDetailSessionData.SetText("// Không có dữ liệu đính kèm (data rỗng)")
		}
	}
}

func clearSessionDetail() {
	selectedSessionID = ""
	refreshActiveImageSlots()
	if lblDetailSessionName != nil {
		lblDetailSessionName.SetText("Chọn một phiên bên trái để xem tiến độ chi tiết")
	}
	if lblDetailSessionStatus != nil {
		lblDetailSessionStatus.SetText("")
	}
	if lblDetailSessionID != nil {
		lblDetailSessionID.SetText("")
	}
	if lblDetailSessionProg != nil {
		lblDetailSessionProg.SetText("")
	}
	if lblDetailSessionTime != nil {
		lblDetailSessionTime.SetText("")
	}
	if barDetailSession != nil {
		barDetailSession.SetValue(0)
	}
	if entryDetailSessionLogs != nil {
		entryDetailSessionLogs.SetText("")
	}
	if entryDetailSessionData != nil {
		entryDetailSessionData.SetText("")
	}
}
