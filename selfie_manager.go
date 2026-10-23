package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ==================== CẤU TRÚC DỮ LIỆU ẢNH SELFIE ====================

type SelfieItem struct {
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"sizeHuman"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	ModTime     string `json:"modTime"`
	URL         string `json:"url"`         // Đường dẫn API xem ảnh trực tiếp
	DownloadURL string `json:"downloadUrl"` // Đường dẫn API tải về máy
}

var (
	selfiesDir         = "selfies"
	selfiesList        []SelfieItem
	selfiesMu          sync.RWMutex
	selfieListWidget   *widget.List
	selectedSelfieIdx  = -1
	selectedSelfieItem *SelfieItem

	// Preview UI components
	lblSelfieHeader *widget.Label
	lblSelfieDetail *widget.Label
	lblSelfieAPIURL *widget.Label
	imgSelfieLarge  *canvas.Image
	boxSelfieDetail *fyne.Container
)

// ==================== KHỞI TẠO & QUÉT THƯ MỤC SELFIE ====================

func initSelfiesDir() {
	if err := os.MkdirAll(selfiesDir, 0755); err != nil {
		return
	}

	// Quét xem đã có ảnh chưa
	scanSelfies()

	// Nếu thư mục hoàn toàn rỗng, tự động tạo 3 ảnh chân dung mẫu chất lượng cao
	selfiesMu.RLock()
	count := len(selfiesList)
	selfiesMu.RUnlock()

	if count == 0 {
		createSampleSelfie("selfie_sample_01.png", color.RGBA{R: 236, G: 72, B: 153, A: 255}, "User Alpha") // Pink 500
		createSampleSelfie("selfie_sample_02.png", color.RGBA{R: 244, G: 114, B: 182, A: 255}, "User Beta")  // Pink 400
		createSampleSelfie("selfie_sample_03.png", color.RGBA{R: 219, G: 39, B: 119, A: 255}, "User Gamma") // Pink 600
		scanSelfies()
	}
}

func scanSelfies() {
	entries, err := os.ReadDir(selfiesDir)
	if err != nil {
		selfiesMu.Lock()
		selfiesList = nil
		selfiesMu.Unlock()
		if selfieListWidget != nil {
			selfieListWidget.Refresh()
		}
		return
	}

	var list []SelfieItem
	validExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".bmp": true,
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !validExts[ext] {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(selfiesDir, e.Name())

		// Đọc kích thước ảnh (Resolution)
		width, height := 0, 0
		f, err := os.Open(fullPath)
		if err == nil {
			cfg, _, err := image.DecodeConfig(f)
			f.Close()
			if err == nil {
				width = cfg.Width
				height = cfg.Height
			}
		}

		item := SelfieItem{
			Filename:    e.Name(),
			Path:        fullPath,
			Size:        info.Size(),
			SizeHuman:   formatFileSize(info.Size()),
			Width:       width,
			Height:      height,
			ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
			URL:         fmt.Sprintf("/api/selfie/file?name=%s", e.Name()),
			DownloadURL: fmt.Sprintf("/api/selfie/file?name=%s&download=true", e.Name()),
		}
		list = append(list, item)
	}

	selfiesMu.Lock()
	selfiesList = list
	selfiesMu.Unlock()

	if selfieListWidget != nil {
		selfieListWidget.Refresh()
	}
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Tạo ảnh mẫu chân dung dạng PNG vector avatar nếu thư mục rỗng
func createSampleSelfie(filename string, bgCol color.RGBA, label string) {
	const w, h = 400, 400
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// Nền trắng hồng nhẹ nhàng
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 245, B: 248, A: 255}}, image.Point{}, draw.Src)

	// Vẽ khung hình tròn nền màu
	cx, cy := 200, 200
	radius := 160
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, bgCol)
			}
		}
	}

	// Vẽ hình đầu (Head silhouette)
	headRadius := 60
	headCY := 160
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := x - cx
			dy := y - headCY
			if dx*dx+dy*dy <= headRadius*headRadius {
				img.Set(x, y, color.RGBA{R: 241, G: 245, B: 249, A: 255})
			}
		}
	}

	// Vẽ vai/thân (Shoulders silhouette)
	shoulderRadius := 105
	shoulderCY := 310
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := x - cx
			dy := y - shoulderCY
			if dx*dx+dy*dy <= shoulderRadius*shoulderRadius && y >= 220 {
				// Chỉ trong phạm vi khung tròn ngoài
				odx := x - cx
				ody := y - cy
				if odx*odx+ody*ody <= radius*radius {
					img.Set(x, y, color.RGBA{R: 226, G: 232, B: 240, A: 255})
				}
			}
		}
	}

	outPath := filepath.Join(selfiesDir, filename)
	outFile, err := os.Create(outPath)
	if err == nil {
		defer outFile.Close()
		_ = png.Encode(outFile, img)
	}
}

// ==================== CÁC HÀM TRUY VẤN DỮ LIỆU API ====================

func GetSelfiesList() []SelfieItem {
	selfiesMu.RLock()
	defer selfiesMu.RUnlock()
	list := make([]SelfieItem, len(selfiesList))
	copy(list, selfiesList)
	return list
}

func GetRandomSelfie() (*SelfieItem, error) {
	selfiesMu.RLock()
	defer selfiesMu.RUnlock()

	if len(selfiesList) == 0 {
		return nil, fmt.Errorf("kho ảnh selfie đang trống")
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := r.Intn(len(selfiesList))
	item := selfiesList[idx]
	return &item, nil
}

func GetSelfieByName(name string) (*SelfieItem, error) {
	selfiesMu.RLock()
	defer selfiesMu.RUnlock()

	cleanName := filepath.Base(name)
	for _, item := range selfiesList {
		if strings.EqualFold(item.Filename, cleanName) {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("không tìm thấy ảnh selfie '%s'", name)
}

func DeleteSelfieFile(name string) error {
	cleanName := filepath.Base(name)
	targetPath := filepath.Join(selfiesDir, cleanName)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return fmt.Errorf("không tìm thấy file '%s' trong kho", cleanName)
	}

	// Nếu ảnh đang xem trùng với ảnh cần xóa, xóa preview trước để giải phóng tài nguyên
	if selectedSelfieItem != nil && strings.EqualFold(selectedSelfieItem.Filename, cleanName) {
		clearSelfiePreview()
	}

	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = os.Remove(targetPath)
		if err == nil {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}

	scanSelfies()
	return err
}

// SaveSelfieBytes lưu mảng bytes ảnh vào thư mục selfies và trả về thông tin SelfieItem
func SaveSelfieBytes(data []byte, customName string) (*SelfieItem, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("dữ liệu ảnh trống")
	}

	// Xác định Content-Type từ bytes
	contentType := http.DetectContentType(data)
	ext := ".jpg"
	switch {
	case strings.Contains(contentType, "png"):
		ext = ".png"
	case strings.Contains(contentType, "webp"):
		ext = ".webp"
	case strings.Contains(contentType, "bmp"):
		ext = ".bmp"
	case strings.Contains(contentType, "jpeg"):
		ext = ".jpg"
	default:
		// Kiểm tra magic bytes cơ bản nếu mime detect không rõ
		if len(data) >= 8 && string(data[1:4]) == "PNG" {
			ext = ".png"
		} else if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			ext = ".jpg"
		} else if len(data) >= 2 && string(data[:2]) == "BM" {
			ext = ".bmp"
		} else if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			ext = ".webp"
		}
	}

	// Đặt tên file
	var finalFilename string
	if customName != "" {
		finalFilename = filepath.Base(strings.TrimSpace(customName))
		if filepath.Ext(finalFilename) == "" {
			finalFilename += ext
		}
	} else {
		finalFilename = fmt.Sprintf("selfie_%s_%04d%s", time.Now().Format("20060102_150405"), rand.Intn(10000), ext)
	}

	// Kiểm tra phần mở rộng
	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".bmp": true}
	curExt := strings.ToLower(filepath.Ext(finalFilename))
	if !validExts[curExt] {
		finalFilename += ext
	}

	// Tránh ghi đè file cũ nếu trùng tên
	targetPath := filepath.Join(selfiesDir, finalFilename)
	if _, err := os.Stat(targetPath); err == nil {
		baseWithoutExt := strings.TrimSuffix(finalFilename, filepath.Ext(finalFilename))
		finalFilename = fmt.Sprintf("%s_%d%s", baseWithoutExt, time.Now().UnixNano()%100000, filepath.Ext(finalFilename))
		targetPath = filepath.Join(selfiesDir, finalFilename)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return nil, fmt.Errorf("lỗi lưu file ảnh: %v", err)
	}

	scanSelfies()
	return GetSelfieByName(finalFilename)
}

// decodeBase64Image giải mã chuỗi base64 (hỗ trợ cả Data URI header data:image/...;base64, và raw base64)
func decodeBase64Image(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, ";base64,"); idx != -1 {
		raw = raw[idx+8:]
	} else if idx := strings.Index(raw, ","); idx != -1 && strings.HasPrefix(raw, "data:") {
		raw = raw[idx+1:]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("chuỗi base64 rỗng")
	}

	b, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return b, nil
	}
	if b, errURL := base64.URLEncoding.DecodeString(raw); errURL == nil {
		return b, nil
	}
	if b, errRaw := base64.RawStdEncoding.DecodeString(raw); errRaw == nil {
		return b, nil
	}
	return nil, err
}

// DownloadSelfieFromURL tải ảnh từ đường dẫn URL trên internet và lưu vào kho selfie
func DownloadSelfieFromURL(rawURL, customName string) (*SelfieItem, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") && !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return nil, fmt.Errorf("link không hợp lệ (phải bắt đầu bằng http:// hoặc https://)")
	}

	client := &http.Client{
		Timeout: 25 * time.Second,
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối tới link: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("máy chủ ảnh trả về mã lỗi HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(bodyBytes) == 0 {
		return nil, fmt.Errorf("không nhận được dữ liệu ảnh từ link")
	}

	if customName == "" {
		u, err := url.Parse(rawURL)
		if err == nil && u.Path != "" {
			urlBase := filepath.Base(u.Path)
			urlExt := strings.ToLower(filepath.Ext(urlBase))
			if urlExt == ".jpg" || urlExt == ".jpeg" || urlExt == ".png" || urlExt == ".webp" || urlExt == ".bmp" {
				customName = urlBase
			}
		}
	}

	return SaveSelfieBytes(bodyBytes, customName)
}

// ==================== GIAO DIỆN TAB KHO ẢNH SELFIE ====================

func buildSelfieTab(w fyne.Window) fyne.CanvasObject {
	// Danh sách ảnh bên trái
	selfieListWidget = widget.NewList(
		func() int {
			selfiesMu.RLock()
			defer selfiesMu.RUnlock()
			return len(selfiesList)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.FileImageIcon())
			nameLbl := widget.NewLabelWithStyle("selfie_01.jpg", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			detailLbl := widget.NewLabel("800x800 • 150 KB")
			detailLbl.TextStyle.Italic = true

			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			delBtn.Importance = widget.LowImportance

			vbox := container.NewVBox(nameLbl, detailLbl)
			return container.NewBorder(nil, nil, icon, delBtn, vbox)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			selfiesMu.RLock()
			if id < 0 || id >= len(selfiesList) {
				selfiesMu.RUnlock()
				return
			}
			item := selfiesList[id]
			selfiesMu.RUnlock()

			border := obj.(*fyne.Container)
			vbox := border.Objects[0].(*fyne.Container)
			nameLbl := vbox.Objects[0].(*widget.Label)
			detailLbl := vbox.Objects[1].(*widget.Label)
			delBtn := border.Objects[2].(*widget.Button)

			nameLbl.SetText(item.Filename)
			if item.Width > 0 && item.Height > 0 {
				detailLbl.SetText(fmt.Sprintf("%dx%d • %s", item.Width, item.Height, item.SizeHuman))
			} else {
				detailLbl.SetText(fmt.Sprintf("%s", item.SizeHuman))
			}

			targetName := item.Filename
			delBtn.OnTapped = func() {
				dialog.ShowConfirm("Xác nhận", fmt.Sprintf("Bạn có chắc chắn muốn xóa ảnh '%s'?", targetName), func(ok bool) {
					if ok {
						if err := DeleteSelfieFile(targetName); err != nil {
							dialog.ShowError(fmt.Errorf("Không thể xóa ảnh '%s': %v", targetName, err), w)
							return
						}
						selfiesMu.RLock()
						total := len(selfiesList)
						selfiesMu.RUnlock()

						selfieListWidget.UnselectAll()
						if total > 0 {
							newIdx := selectedSelfieIdx
							if newIdx >= total {
								newIdx = total - 1
							}
							if newIdx < 0 {
								newIdx = 0
							}
							selfieListWidget.Select(newIdx)
						} else {
							clearSelfiePreview()
						}
					}
				}, w)
			}
		},
	)

	selfieListWidget.OnSelected = func(id widget.ListItemID) {
		selfiesMu.RLock()
		if id < 0 || id >= len(selfiesList) {
			selfiesMu.RUnlock()
			return
		}
		item := selfiesList[id]
		selfiesMu.RUnlock()

		selectedSelfieIdx = id
		selectedSelfieItem = &item
		loadSelfiePreview(&item, w)
	}

	// Toolbar bên trái
	btnImport := widget.NewButtonWithIcon("Thêm Ảnh Selfie", theme.ContentAddIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			srcName := reader.URI().Name()
			dstPath := filepath.Join(selfiesDir, filepath.Base(srcName))

			dstFile, err := os.Create(dstPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Không thể lưu ảnh: %v", err), w)
				return
			}
			defer dstFile.Close()

			if _, err := io.Copy(dstFile, reader); err != nil {
				dialog.ShowError(fmt.Errorf("Lỗi sao chép ảnh: %v", err), w)
				return
			}

			scanSelfies()
			dialog.ShowInformation("Thành công", fmt.Sprintf("Đã nhập ảnh '%s' vào kho selfie!", srcName), w)
		}, w)

		fd.SetFilter(storage.NewExtensionFileFilter([]string{".jpg", ".jpeg", ".png", ".webp", ".bmp"}))
		fd.Show()
	})
	btnImport.Importance = widget.HighImportance

	btnDownloadURL := widget.NewButtonWithIcon("Dán Link Tải Ảnh", theme.DownloadIcon(), func() {
		entryURL := widget.NewEntry()
		entryURL.SetPlaceHolder("Dán đường link ảnh tại đây (https://.../photo.jpg)")

		entryCustomName := widget.NewEntry()
		entryCustomName.SetPlaceHolder("Tên file tuỳ chọn (ví dụ: avatar.jpg, để trống tự đặt)")

		statusLbl := widget.NewLabel("Nhập link ảnh (JPEG, PNG, WebP) và nhấn 'Bắt Đầu Tải'")
		statusLbl.TextStyle.Italic = true

		var dlg dialog.Dialog
		btnStartDownload := widget.NewButtonWithIcon("Bắt Đầu Tải Về", theme.DownloadIcon(), func() {
			rawURL := strings.TrimSpace(entryURL.Text)
			if rawURL == "" {
				statusLbl.SetText("❌ Vui lòng dán link ảnh!")
				return
			}
			customName := strings.TrimSpace(entryCustomName.Text)
			statusLbl.SetText("⏳ Đang tải ảnh từ internet...")

			go func() {
				item, err := DownloadSelfieFromURL(rawURL, customName)
				if err != nil {
					statusLbl.SetText(fmt.Sprintf("❌ Lỗi: %v", err))
					return
				}
				dlg.Hide()
				dialog.ShowInformation("Thành công", fmt.Sprintf("Đã tải xong ảnh: %s (%s)", item.Filename, item.SizeHuman), w)

				selfiesMu.RLock()
				targetIdx := -1
				for i, s := range selfiesList {
					if s.Filename == item.Filename {
						targetIdx = i
						break
					}
				}
				selfiesMu.RUnlock()

				if targetIdx >= 0 {
					selfieListWidget.UnselectAll()
					selfieListWidget.Select(targetIdx)
				}
			}()
		})
		btnStartDownload.Importance = widget.HighImportance

		content := container.NewVBox(
			widget.NewLabelWithStyle("🌐 Tải Ảnh Selfie Từ Đường Link (URL)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel("Link ảnh (URL):"),
			entryURL,
			widget.NewLabel("Tên file lưu trữ (tuỳ chọn):"),
			entryCustomName,
			statusLbl,
			btnStartDownload,
		)

		dlg = dialog.NewCustom("Tải Ảnh Từ Link", "Đóng", container.NewPadded(content), w)
		dlg.Resize(fyne.NewSize(500, 280))
		dlg.Show()
	})
	btnDownloadURL.Importance = widget.HighImportance

	btnOpenFolder := widget.NewButtonWithIcon("Mở Thư Mục", theme.FolderOpenIcon(), func() {
		absPath, err := filepath.Abs(selfiesDir)
		if err == nil {
			_ = exec.Command("explorer.exe", absPath).Start()
		}
	})

	btnRefresh := widget.NewButtonWithIcon("Làm Mới", theme.ViewRefreshIcon(), func() {
		scanSelfies()
	})

	leftHeader := container.NewVBox(
		widget.NewLabelWithStyle("📸 KHO ẢNH SELFIE", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, btnImport, btnDownloadURL, btnOpenFolder, btnRefresh),
		widget.NewSeparator(),
	)

	leftPanel := container.NewBorder(
		leftHeader,
		nil, nil, nil,
		selfieListWidget,
	)

	// BÊN PHẢI: Khung Xem Ảnh & Thao Tác
	lblSelfieHeader = widget.NewLabelWithStyle("Chọn một ảnh bên trái để xem trước", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	lblSelfieDetail = widget.NewLabel("")
	lblSelfieAPIURL = widget.NewLabel("")

	imgSelfieLarge = canvas.NewImageFromResource(theme.FileImageIcon())
	imgSelfieLarge.FillMode = canvas.ImageFillContain
	imgSelfieLarge.SetMinSize(fyne.NewSize(380, 380))

	btnCopyURL := widget.NewButtonWithIcon("Sao chép Link API", theme.ContentCopyIcon(), func() {
		if selectedSelfieItem != nil {
			port := "8088"
			if portEntry != nil && portEntry.Text != "" {
				port = portEntry.Text
			}
			fullURL := fmt.Sprintf("http://localhost:%s/api/selfie/file?name=%s", port, selectedSelfieItem.Filename)
			w.Clipboard().SetContent(fullURL)
			dialog.ShowInformation("Đã sao chép", "Đã chép link API xem ảnh:\n"+fullURL, w)
		}
	})

	btnCopyBase64 := widget.NewButtonWithIcon("Sao chép Base64", theme.ContentPasteIcon(), func() {
		if selectedSelfieItem != nil {
			data, err := os.ReadFile(selectedSelfieItem.Path)
			if err == nil {
				b64 := base64.StdEncoding.EncodeToString(data)
				dataURI := fmt.Sprintf("data:image/png;base64,%s", b64)
				w.Clipboard().SetContent(dataURI)
				dialog.ShowInformation("Đã sao chép", "Đã chép chuỗi Data-URI Base64 của ảnh vào bộ nhớ tạm!", w)
			}
		}
	})

	btnOpenExternal := widget.NewButtonWithIcon("Mở bằng App Ngoài", theme.NavigateNextIcon(), func() {
		if selectedSelfieItem != nil {
			absPath, err := filepath.Abs(selectedSelfieItem.Path)
			if err == nil {
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", absPath).Start()
			}
		}
	})

	btnDeleteSelected := widget.NewButtonWithIcon("Xóa Ảnh", theme.DeleteIcon(), func() {
		if selectedSelfieItem != nil {
			targetName := selectedSelfieItem.Filename
			dialog.ShowConfirm("Xác nhận xóa", fmt.Sprintf("Bạn có chắc chắn muốn xóa vĩnh viễn '%s'?", targetName), func(ok bool) {
				if ok {
					if err := DeleteSelfieFile(targetName); err != nil {
						dialog.ShowError(fmt.Errorf("Không thể xóa ảnh '%s': %v", targetName, err), w)
						return
					}
					selfiesMu.RLock()
					total := len(selfiesList)
					selfiesMu.RUnlock()

					selfieListWidget.UnselectAll()
					if total > 0 {
						newIdx := selectedSelfieIdx
						if newIdx >= total {
							newIdx = total - 1
						}
						if newIdx < 0 {
							newIdx = 0
						}
						selfieListWidget.Select(newIdx)
					} else {
						clearSelfiePreview()
					}
				}
			}, w)
		} else {
			dialog.ShowInformation("Thông báo", "Vui lòng chọn một ảnh để xóa!", w)
		}
	})
	btnDeleteSelected.Importance = widget.DangerImportance

	actionsBar := container.NewHBox(
		btnCopyURL,
		btnCopyBase64,
		btnOpenExternal,
		layout.NewSpacer(),
		btnDeleteSelected,
	)

	headerInfo := container.NewVBox(
		lblSelfieHeader,
		lblSelfieDetail,
		lblSelfieAPIURL,
		actionsBar,
		widget.NewSeparator(),
	)

	imageContainer := container.NewPadded(imgSelfieLarge)

	boxSelfieDetail = container.NewBorder(
		headerInfo,
		nil, nil, nil,
		imageContainer,
	)

	mainSplit := container.NewHSplit(leftPanel, boxSelfieDetail)
	mainSplit.SetOffset(0.32)

	// Chọn ảnh đầu tiên nếu có
	selfiesMu.RLock()
	hasImages := len(selfiesList) > 0
	selfiesMu.RUnlock()
	if hasImages {
		selfieListWidget.Select(0)
	}

	return mainSplit
}

func loadSelfiePreview(item *SelfieItem, w fyne.Window) {
	if item == nil {
		return
	}
	lblSelfieHeader.SetText(fmt.Sprintf("🖼️ %s", item.Filename))
	if item.Width > 0 && item.Height > 0 {
		lblSelfieDetail.SetText(fmt.Sprintf("📏 Độ phân giải: %dx%d px  |  💾 Dung lượng: %s  |  ⏱️ Ngày thêm: %s", item.Width, item.Height, item.SizeHuman, item.ModTime))
	} else {
		lblSelfieDetail.SetText(fmt.Sprintf("💾 Dung lượng: %s  |  ⏱️ Ngày thêm: %s", item.SizeHuman, item.ModTime))
	}

	port := "8088"
	if portEntry != nil && portEntry.Text != "" {
		port = portEntry.Text
	}
	lblSelfieAPIURL.SetText(fmt.Sprintf("🌐 API: http://localhost:%s%s", port, item.URL))

	imgBytes, err := os.ReadFile(item.Path)
	if err == nil {
		imgSelfieLarge.File = ""
		imgSelfieLarge.Resource = fyne.NewStaticResource(item.Filename, imgBytes)
		imgSelfieLarge.Refresh()
	} else {
		imgSelfieLarge.File = ""
		imgSelfieLarge.Resource = theme.FileImageIcon()
		imgSelfieLarge.Refresh()
	}
}

func clearSelfiePreview() {
	lblSelfieHeader.SetText("Chọn một ảnh bên trái để xem trước")
	lblSelfieDetail.SetText("")
	lblSelfieAPIURL.SetText("")
	imgSelfieLarge.File = ""
	imgSelfieLarge.Resource = theme.FileImageIcon()
	imgSelfieLarge.Refresh()
	selectedSelfieItem = nil
	selectedSelfieIdx = -1
}

// ==================== CÁC ROUTE API KHO ẢNH SELFIE ====================

func registerSelfieAPIRoutes(mux *http.ServeMux, setCors func(http.ResponseWriter)) {
	// 1. GET /api/selfies hoặc /api/selfie/list -> Danh sách toàn bộ ảnh selfie
	handleSelfiesList := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		list := GetSelfiesList()
		appendAPILog("📸 200 OK: /api/selfies - Trả về %d ảnh selfie cho %s", len(list), r.RemoteAddr)

		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(rw).Encode(list)
	}

	mux.HandleFunc("/api/selfies", handleSelfiesList)
	mux.HandleFunc("/api/selfie/list", handleSelfiesList)

	// 2. GET hoặc DELETE /api/selfie hoặc /api/selfie/random -> Lấy ngẫu nhiên hoặc theo tên / Xóa ảnh
	handleSelfieGet := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		q := r.URL.Query()
		name := strings.TrimSpace(q.Get("name"))

		// Nếu là phương thức DELETE
		if r.Method == http.MethodDelete {
			if name == "" {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]string{"error": "Vui lòng cung cấp tham số 'name' của ảnh cần xóa"})
				return
			}
			err := DeleteSelfieFile(name)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]interface{}{"success": false, "error": err.Error()})
				return
			}
			appendAPILog("🗑️ Đã xóa ảnh selfie qua API: %s", name)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]interface{}{"success": true, "deleted": name})
			return
		}

		var item *SelfieItem
		var err error

		if name != "" {
			item, err = GetSelfieByName(name)
		} else {
			item, err = GetRandomSelfie()
		}

		if err != nil || item == nil {
			rw.WriteHeader(http.StatusNotFound)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{
				"error": "Không tìm thấy ảnh selfie phù hợp trong kho",
			})
			return
		}

		// Kiểm tra định dạng client yêu cầu
		format := strings.ToLower(q.Get("format"))
		accept := strings.ToLower(r.Header.Get("Accept"))

		// Nếu client yêu cầu JSON hoặc base64
		if format == "json" || (strings.Contains(accept, "application/json") && !strings.Contains(accept, "image/")) {
			if q.Get("base64") == "true" {
				data, err := os.ReadFile(item.Path)
				if err != nil {
					rw.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(rw).Encode(map[string]string{"error": "Không thể đọc tệp ảnh"})
					return
				}
				mime := "image/jpeg"
				if strings.HasSuffix(strings.ToLower(item.Filename), ".png") {
					mime = "image/png"
				} else if strings.HasSuffix(strings.ToLower(item.Filename), ".webp") {
					mime = "image/webp"
				}
				dataURI := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))

				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]interface{}{
					"selfie":   item,
					"data_uri": dataURI,
				})
				return
			}

			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(item)
			return
		}

		// Mặc định hoặc format=raw / format=image -> Trả về trực tiếp nội dung byte của ảnh
		appendAPILog("📸 200 OK: Xuất ảnh selfie '%s' cho %s", item.Filename, r.RemoteAddr)
		serveImageFile(rw, r, item.Path, item.Filename, q.Get("download") == "true")
	}

	mux.HandleFunc("/api/selfie", handleSelfieGet)
	mux.HandleFunc("/api/selfie/random", handleSelfieGet)

	// 3. GET /api/selfie/file -> Trả về file ảnh thô trực tiếp
	mux.HandleFunc("/api/selfie/file", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{"error": "Thiếu tham số 'name'"})
			return
		}

		cleanName := filepath.Base(name)

		if r.Method == http.MethodDelete {
			err := DeleteSelfieFile(cleanName)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]interface{}{"success": false, "error": err.Error()})
				return
			}
			appendAPILog("🗑️ Đã xóa ảnh selfie qua API: %s", cleanName)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]interface{}{"success": true, "deleted": cleanName})
			return
		}

		item, err := GetSelfieByName(cleanName)
		if err != nil || item == nil {
			rw.WriteHeader(http.StatusNotFound)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{"error": "Không tìm thấy ảnh: " + cleanName})
			return
		}

		serveImageFile(rw, r, item.Path, item.Filename, r.URL.Query().Get("download") == "true")
	})

	// 5. POST /api/selfie/download-url (hoặc /api/selfie/download) -> Tải ảnh từ URL bên ngoài về kho
	handleSelfieDownloadURL := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		var rawURL string
		var customName string

		if r.Method == http.MethodPost {
			bodyBytes, _ := io.ReadAll(r.Body)
			if len(bodyBytes) > 0 {
				var req struct {
					URL      string `json:"url"`
					Filename string `json:"filename"`
				}
				if json.Unmarshal(bodyBytes, &req) == nil && req.URL != "" {
					rawURL = req.URL
					customName = req.Filename
				} else {
					rawURL = strings.TrimSpace(string(bodyBytes))
				}
			}
		}

		if rawURL == "" {
			rawURL = strings.TrimSpace(r.URL.Query().Get("url"))
			if customName == "" {
				customName = strings.TrimSpace(r.URL.Query().Get("filename"))
			}
		}

		if rawURL == "" {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{
				"error": "Vui lòng cung cấp link 'url' của ảnh cần tải về (POST JSON {\"url\": \"...\"} hoặc GET ?url=...)",
			})
			return
		}

		item, err := DownloadSelfieFromURL(rawURL, customName)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		appendAPILog("🌐 200 OK: Đã tải ảnh selfie từ link '%s' -> %s (%s)", rawURL, item.Filename, item.SizeHuman)

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"success": true,
			"selfie":  item,
		})
	}

	mux.HandleFunc("/api/selfie/download-url", handleSelfieDownloadURL)
	mux.HandleFunc("/api/selfie/download", handleSelfieDownloadURL)

	// 6. POST hoặc DELETE /api/selfie/delete (hoặc /api/selfie/remove) -> Xóa ảnh selfie
	handleSelfieDelete := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		var names []string
		var clearAll bool

		// 1. Đọc từ Body nếu có
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			if len(bodyBytes) > 0 {
				var req struct {
					Name      string   `json:"name"`
					Filename  string   `json:"filename"`
					Names     []string `json:"names"`
					Filenames []string `json:"filenames"`
					Action    string   `json:"action"`
				}
				if json.Unmarshal(bodyBytes, &req) == nil {
					if req.Action == "clear_all" {
						clearAll = true
					}
					if req.Name != "" {
						names = append(names, req.Name)
					}
					if req.Filename != "" {
						names = append(names, req.Filename)
					}
					names = append(names, req.Names...)
					names = append(names, req.Filenames...)
				} else {
					rawStr := strings.TrimSpace(string(bodyBytes))
					if rawStr != "" {
						names = append(names, rawStr)
					}
				}
			}
		}

		// 2. Đọc từ Query parameters
		q := r.URL.Query()
		if q.Get("action") == "clear_all" || q.Get("clear") == "all" {
			clearAll = true
		}
		if v := q.Get("name"); v != "" {
			names = append(names, v)
		}
		if v := q.Get("filename"); v != "" {
			names = append(names, v)
		}

		// Xóa toàn bộ ảnh nếu client yêu cầu clear_all
		if clearAll {
			selfiesMu.RLock()
			var allNames []string
			for _, s := range selfiesList {
				allNames = append(allNames, s.Filename)
			}
			selfiesMu.RUnlock()

			deletedCount := 0
			for _, n := range allNames {
				if err := DeleteSelfieFile(n); err == nil {
					deletedCount++
				}
			}
			appendAPILog("🗑️ 200 OK: Đã xóa toàn bộ %d ảnh selfie qua API", deletedCount)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": true,
				"cleared": deletedCount,
				"message": fmt.Sprintf("Đã xóa toàn bộ %d ảnh selfie", deletedCount),
			})
			return
		}

		if len(names) == 0 {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{
				"error": "Vui lòng cung cấp tên ảnh cần xóa qua JSON body {'name': '...'} hoặc query ?name=...",
			})
			return
		}

		var deleted []string
		var errorsList []string

		for _, n := range names {
			cleanName := filepath.Base(strings.TrimSpace(n))
			if cleanName == "" || cleanName == "." || cleanName == "/" {
				continue
			}
			if err := DeleteSelfieFile(cleanName); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", cleanName, err))
			} else {
				deleted = append(deleted, cleanName)
				appendAPILog("🗑️ 200 OK: Đã xóa ảnh selfie '%s' qua API", cleanName)
			}
		}

		rw.Header().Set("Content-Type", "application/json")
		if len(deleted) == 0 && len(errorsList) > 0 {
			rw.WriteHeader(http.StatusNotFound)
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": false,
				"errors":  errorsList,
			})
			return
		}

		json.NewEncoder(rw).Encode(map[string]interface{}{
			"success": true,
			"deleted": deleted,
			"errors":  errorsList,
			"message": fmt.Sprintf("Đã xóa thành công %d ảnh", len(deleted)),
		})
	}

	mux.HandleFunc("/api/selfie/delete", handleSelfieDelete)
	mux.HandleFunc("/api/selfie/remove", handleSelfieDelete)

	// 7. POST /api/selfie/upload (hoặc /api/selfie/add) -> Thêm ảnh mới vào kho qua API
	handleSelfieUpload := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{"error": "Chỉ chấp nhận phương thức POST hoặc PUT"})
			return
		}

		contentType := r.Header.Get("Content-Type")
		var uploadedItems []*SelfieItem
		var uploadErrors []string

		// Cách 1: multipart/form-data (Upload file truyền thống qua Form hoặc cURL -F file=@...)
		if strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data") {
			if err := r.ParseMultipartForm(64 << 20); err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]string{"error": fmt.Sprintf("Lỗi đọc multipart form: %v", err)})
				return
			}

			var fileHeaders []*multipart.FileHeader
			for _, key := range []string{"file", "image", "photo", "upload", "selfie", "files"} {
				if fhs, ok := r.MultipartForm.File[key]; ok {
					fileHeaders = append(fileHeaders, fhs...)
				}
			}
			if len(fileHeaders) == 0 && r.MultipartForm != nil {
				for _, fhs := range r.MultipartForm.File {
					fileHeaders = append(fileHeaders, fhs...)
				}
			}

			if len(fileHeaders) == 0 {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]string{
					"error": "Không tìm thấy file ảnh trong form data. Vui lòng đặt name cho field là 'file' hoặc 'image'.",
				})
				return
			}

			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					uploadErrors = append(uploadErrors, fmt.Sprintf("%s: %v", fh.Filename, err))
					continue
				}
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil || len(data) == 0 {
					uploadErrors = append(uploadErrors, fmt.Sprintf("%s: file rỗng hoặc lỗi đọc", fh.Filename))
					continue
				}

				item, err := SaveSelfieBytes(data, fh.Filename)
				if err != nil {
					uploadErrors = append(uploadErrors, fmt.Sprintf("%s: %v", fh.Filename, err))
				} else {
					uploadedItems = append(uploadedItems, item)
					appendAPILog("📥 200 OK: Đã thêm ảnh selfie qua upload form '%s' (%s)", item.Filename, item.SizeHuman)
				}
			}
		} else {
			// Đọc body cho JSON hoặc Raw Binary
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil || len(bodyBytes) == 0 {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]string{"error": "Dữ liệu tải lên trống!"})
				return
			}

			var jsonReq struct {
				Image    string `json:"image"`
				Base64   string `json:"base64"`
				Data     string `json:"data"`
				Filename string `json:"filename"`
				Name     string `json:"name"`
				URL      string `json:"url"`
				Images   []struct {
					Image    string `json:"image"`
					Base64   string `json:"base64"`
					Filename string `json:"filename"`
					Name     string `json:"name"`
				} `json:"images"`
			}

			if json.Unmarshal(bodyBytes, &jsonReq) == nil {

				// 2a. Nhiều ảnh Base64
				if len(jsonReq.Images) > 0 {
					for idx, imgObj := range jsonReq.Images {
						rawB64 := imgObj.Image
						if rawB64 == "" {
							rawB64 = imgObj.Base64
						}
						fn := imgObj.Filename
						if fn == "" {
							fn = imgObj.Name
						}
						if fn == "" {
							fn = fmt.Sprintf("selfie_upload_%d_%d.jpg", time.Now().Unix(), idx+1)
						}

						decBytes, err := decodeBase64Image(rawB64)
						if err != nil {
							uploadErrors = append(uploadErrors, fmt.Sprintf("ảnh %d: lỗi giải mã base64 (%v)", idx+1, err))
							continue
						}
						item, err := SaveSelfieBytes(decBytes, fn)
						if err != nil {
							uploadErrors = append(uploadErrors, fmt.Sprintf("ảnh %d: %v", idx+1, err))
						} else {
							uploadedItems = append(uploadedItems, item)
							appendAPILog("📥 200 OK: Đã thêm ảnh selfie base64 '%s' (%s)", item.Filename, item.SizeHuman)
						}
					}
				}

				// 2b. 1 ảnh Base64
				rawB64 := jsonReq.Image
				if rawB64 == "" {
					rawB64 = jsonReq.Base64
				}
				if rawB64 == "" {
					rawB64 = jsonReq.Data
				}

				if rawB64 != "" {
					fn := jsonReq.Filename
					if fn == "" {
						fn = jsonReq.Name
					}
					decBytes, err := decodeBase64Image(rawB64)
					if err != nil {
						rw.WriteHeader(http.StatusBadRequest)
						rw.Header().Set("Content-Type", "application/json")
						json.NewEncoder(rw).Encode(map[string]string{"error": fmt.Sprintf("Lỗi giải mã base64: %v", err)})
						return
					}
					item, err := SaveSelfieBytes(decBytes, fn)
					if err != nil {
						rw.WriteHeader(http.StatusBadRequest)
						rw.Header().Set("Content-Type", "application/json")
						json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
						return
					}
					uploadedItems = append(uploadedItems, item)
					appendAPILog("📥 200 OK: Đã thêm ảnh selfie base64 '%s' (%s)", item.Filename, item.SizeHuman)
				} else if jsonReq.URL != "" {
					// 2c. Tải từ URL nếu client gửi {"url": "..."} vào upload endpoint
					fn := jsonReq.Filename
					if fn == "" {
						fn = jsonReq.Name
					}
					item, err := DownloadSelfieFromURL(jsonReq.URL, fn)
					if err != nil {
						rw.WriteHeader(http.StatusBadRequest)
						rw.Header().Set("Content-Type", "application/json")
						json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
						return
					}
					uploadedItems = append(uploadedItems, item)
					appendAPILog("📥 200 OK: Đã thêm ảnh selfie từ URL '%s' -> %s", jsonReq.URL, item.Filename)
				}

				if len(uploadedItems) == 0 && len(uploadErrors) == 0 {
					rw.WriteHeader(http.StatusBadRequest)
					rw.Header().Set("Content-Type", "application/json")
					json.NewEncoder(rw).Encode(map[string]string{
						"error": "Vui lòng cung cấp chuỗi Base64 qua trường 'image' hoặc danh sách 'images'",
					})
					return
				}
			} else {
				// Cách 3: Raw Binary Body (gửi trực tiếp binary dữ liệu qua Body)
				customName := r.URL.Query().Get("filename")
				if customName == "" {
					customName = r.URL.Query().Get("name")
				}
				if customName == "" {
					customName = r.Header.Get("X-Filename")
				}

				item, err := SaveSelfieBytes(bodyBytes, customName)
				if err != nil {
					rw.WriteHeader(http.StatusBadRequest)
					rw.Header().Set("Content-Type", "application/json")
					json.NewEncoder(rw).Encode(map[string]string{"error": err.Error()})
					return
				}
				uploadedItems = append(uploadedItems, item)
				appendAPILog("📥 200 OK: Đã thêm ảnh selfie từ raw binary '%s' (%s)", item.Filename, item.SizeHuman)
			}
		}

		rw.Header().Set("Content-Type", "application/json")
		if len(uploadedItems) == 0 {
			rw.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": false,
				"errors":  uploadErrors,
				"message": "Không thể thêm ảnh",
			})
			return
		}

		if len(uploadedItems) == 1 {
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Đã thêm thành công ảnh '%s'", uploadedItems[0].Filename),
				"selfie":  uploadedItems[0],
				"errors":  uploadErrors,
			})
		} else {
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Đã thêm thành công %d ảnh", len(uploadedItems)),
				"selfies": uploadedItems,
				"count":   len(uploadedItems),
				"errors":  uploadErrors,
			})
		}
	}

	mux.HandleFunc("/api/selfie/upload", handleSelfieUpload)
	mux.HandleFunc("/api/selfie/add", handleSelfieUpload)
	mux.HandleFunc("/api/selfie/create", handleSelfieUpload)
}

func serveImageFile(rw http.ResponseWriter, r *http.Request, filePath, filename string, isDownload bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := "image/jpeg"
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	case ".bmp":
		contentType = "image/bmp"
	}

	rw.Header().Set("Content-Type", contentType)
	rw.Header().Set("Cache-Control", "public, max-age=86400")

	if isDownload {
		rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	} else {
		rw.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	}

	http.ServeFile(rw, r, filePath)
}
