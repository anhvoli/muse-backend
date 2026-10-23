package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// ==================== BỘ GIAO DIỆN HIỆN ĐẠI (WHITE & PINK THEME - TRẮNG & HỒNG) ====================
type modernTheme struct{}

var _ fyne.Theme = (*modernTheme)(nil)

func (m *modernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255} // Nền trắng tinh khiết (#FFFFFF)
	case theme.ColorNameButton:
		return color.NRGBA{R: 253, G: 242, B: 248, A: 255} // Hồng phấn nhẹ (#FDF2F8)
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 243, G: 244, B: 246, A: 255} // Xám nhạt vô hiệu hoá (#F3F4F6)
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 236, G: 72, B: 153, A: 255} // Hồng tươi rực rỡ chủ đạo (#EC4899 - Pink 500)
	case theme.ColorNameForeground:
		return color.NRGBA{R: 30, G: 41, B: 59, A: 255} // Chữ đen than rõ nét Slate 800 (#1E293B)
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255} // Chữ trắng trên nút hồng HighImportance
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255} // Chữ trắng trên thông báo trạng thái
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 71, G: 85, B: 105, A: 255} // Slate 600 (#475569) - chữ nhật ký rõ nét, tương phản cao trên nền trắng
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255} // Nền ô nhập trắng (#FFFFFF)
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 244, G: 114, B: 182, A: 255} // Viền ô nhập hồng nhẹ (#F472B6)
	case theme.ColorNameFocus:
		return color.NRGBA{R: 236, G: 72, B: 153, A: 255} // Viền focus hồng (#EC4899)
	case theme.ColorNameHover:
		return color.NRGBA{R: 251, G: 207, B: 232, A: 140} // Hover hồng phấn pastel (#FBCFE8)
	case theme.ColorNamePressed:
		return color.NRGBA{R: 244, G: 114, B: 182, A: 160} // Pressed hồng
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 251, G: 207, B: 232, A: 255} // Vạch ngăn cách hồng pastel (#FBCFE8)
	case theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground, theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 253, G: 242, B: 248, A: 255} // Nền header / menu hồng phấn nhẹ (#FDF2F8)
	case theme.ColorNameSelection:
		return color.NRGBA{R: 244, G: 114, B: 182, A: 80} // Bôi chọn văn bản ánh hồng
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 244, G: 114, B: 182, A: 160} // Thanh cuộn hồng
	case theme.ColorNameShadow:
		return color.NRGBA{R: 219, G: 39, B: 119, A: 35} // Đổ bóng hồng dịu
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 156, G: 163, B: 175, A: 255}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 219, G: 39, B: 119, A: 255} // Liên kết hồng sẫm (#DB2777)
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 16, G: 185, B: 129, A: 255} // Emerald 500
	case theme.ColorNameWarning:
		return color.NRGBA{R: 245, G: 158, B: 11, A: 255} // Amber 500
	case theme.ColorNameError:
		return color.NRGBA{R: 225, G: 29, B: 72, A: 255} // Rose 600 (#E11D48)
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (m *modernTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *modernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *modernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 14
	}
	return theme.DefaultTheme().Size(name)
}

// ==================== CẤU TRÚC DỮ LIỆU ====================
type Account struct {
	Email       string `json:"email"`
	AppPassword string `json:"appPassword"`
}

type EmailItem struct {
	ID           string                 `json:"id"`
	AccountEmail string                 `json:"accountEmail"`
	Subject      string                 `json:"subject"`
	From         string                 `json:"from"`
	Date         string                 `json:"date"`
	Snippet      string                 `json:"snippet"`
	Body         string                 `json:"body"`
	ParsedData   map[string]interface{} `json:"parsed,omitempty"`
}

var (
	accountsFile = "accounts.json"
	accounts     []Account
	emails       []EmailItem
	mu           sync.Mutex

	accListWidget  *widget.List
	mailListWidget *widget.List

	detailSubject *widget.Label
	detailFrom    *widget.Label
	detailAccount *widget.Label
	detailDate    *widget.Label
	detailParsed  *widget.Label
	detailBody    *widget.Entry
	statusLabel   *widget.Label
	progressBar   *widget.ProgressBarInfinite
	syncBtn       *widget.Button

	// API Server state (Không cần bảo mật)
	apiServer    *http.Server
	apiRunning   bool
	apiMu        sync.Mutex
	apiLogs      []string
	apiLogsMu    sync.Mutex
	apiLogEntry  *widget.Entry
	lblAPIStatus *widget.Label
	btnToggleAPI *widget.Button
	portEntry    *widget.Entry

	// Public Tunnel
	tunnelCmd    *exec.Cmd
	publicURL    string
	tunnelMu     sync.Mutex
	lblPublicURL *widget.Label
	btnCopyURL   *widget.Button
	checkPublic  *widget.Check

	// Tailscale state
	tsIP          string
	tsDNSName     string
	tsFunnelURL   string
	lblTailscale  *widget.Label
	btnCopyTS     *widget.Button
	checkTSFunnel *widget.Check
)

func main() {
	loadAccounts()
	loadParsers()
	loadSessions()
	initSelfiesDir()
	detectTailscale()

	// Kiểm tra chế độ Headless / Server (Dành cho Deploy Docker / VPS / Cloud)
	isServerMode := false
	for _, arg := range os.Args[1:] {
		if arg == "--server" || arg == "-server" || arg == "--headless" || arg == "-headless" {
			isServerMode = true
			break
		}
	}
	if os.Getenv("HEADLESS") == "true" || os.Getenv("SERVER_MODE") == "true" {
		isServerMode = true
	}

	if isServerMode {
		serverPort := os.Getenv("PORT")
		if serverPort == "" {
			serverPort = "8088"
		}
		log.Printf("🚀 Khởi chạy Gmail Hub & Selfie Server trên port %s (Chế độ Headless/Deploy)...", serverPort)
		if err := startAPIServer(serverPort, nil); err != nil {
			log.Fatalf("❌ Lỗi khởi động server: %v", err)
		}
		log.Printf("✅ Server Web & API đang hoạt động tại: http://0.0.0.0:%s", serverPort)
		// Giữ tiến trình chạy cho container / background service
		select {}
	}

	a := app.NewWithID("com.gmail.desktop.reader")
	a.Settings().SetTheme(&modernTheme{})

	w := a.NewWindow("Gmail Desktop Reader - White & Pink Edition")
	w.Resize(fyne.NewSize(1200, 760))
	w.CenterOnScreen()

	// ==================== TAB 1: QUẢN LÝ GMAIL & HỘP THƯ ====================
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("user@gmail.com")

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Mật khẩu ứng dụng (16 ký tự)")

	addBtn := widget.NewButtonWithIcon("Thêm Gmail", theme.ContentAddIcon(), func() {
		email := emailEntry.Text
		pass := passEntry.Text
		if email == "" || pass == "" {
			dialog.ShowInformation("Thông báo", "Vui lòng nhập đầy đủ Email và Mật khẩu ứng dụng!", w)
			return
		}

		mu.Lock()
		for _, acc := range accounts {
			if acc.Email == email {
				mu.Unlock()
				dialog.ShowError(fmt.Errorf("Tài khoản %s đã tồn tại", email), w)
				return
			}
		}
		accounts = append(accounts, Account{Email: email, AppPassword: pass})
		saveAccounts()
		mu.Unlock()

		emailEntry.SetText("")
		passEntry.SetText("")
		accListWidget.Refresh()
	})
	addBtn.Importance = widget.HighImportance

	// Danh sách tài khoản đã thêm
	accListWidget = widget.NewList(
		func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(accounts)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.AccountIcon())
			lbl := widget.NewLabelWithStyle("user@gmail.com", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			btn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			return container.NewBorder(nil, nil, icon, btn, lbl)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			mu.Lock()
			if id < 0 || id >= len(accounts) {
				mu.Unlock()
				return
			}
			acc := accounts[id]
			mu.Unlock()

			c := obj.(*fyne.Container)
			for _, child := range c.Objects {
				if l, ok := child.(*widget.Label); ok {
					l.SetText(acc.Email)
				}
				if b, ok := child.(*widget.Button); ok {
					b.OnTapped = func() {
						dialog.ShowConfirm("Xác nhận", "Xóa tài khoản "+acc.Email+" khỏi danh sách?", func(ok bool) {
							if ok {
								mu.Lock()
								if id < len(accounts) {
									accounts = append(accounts[:id], accounts[id+1:]...)
									saveAccounts()
								}
								mu.Unlock()
								accListWidget.Refresh()
							}
						}, w)
					}
				}
			}
		},
	)

	syncBtn = widget.NewButtonWithIcon("Quét Thư Mới (Tất Cả)", theme.ViewRefreshIcon(), func() {
		go fetchAllEmails(w)
	})
	syncBtn.Importance = widget.HighImportance

	statusLabel = widget.NewLabel("Sẵn sàng")
	progressBar = widget.NewProgressBarInfinite()
	progressBar.Hide()

	helpBtn := widget.NewButtonWithIcon("Hướng dẫn", theme.HelpIcon(), func() {
		showGuideDialog(w)
	})

	leftPanel := container.NewBorder(
		container.NewVBox(
			container.NewHBox(
				widget.NewLabelWithStyle("🌸 QUẢN LÝ GMAIL", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				layout.NewSpacer(),
				helpBtn,
			),
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Thêm tài khoản mới:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
			emailEntry,
			passEntry,
			addBtn,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Tài khoản đã liên kết:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		),
		container.NewVBox(
			widget.NewSeparator(),
			progressBar,
			statusLabel,
			syncBtn,
		),
		nil, nil,
		accListWidget,
	)

	// Danh sách thư (Card view)
	mailListWidget = widget.NewList(
		func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(emails)
		},
		func() fyne.CanvasObject {
			senderLbl := widget.NewLabelWithStyle("Người gửi", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			dateLbl := widget.NewLabel("Thời gian")
			topRow := container.NewHBox(senderLbl, layout.NewSpacer(), dateLbl)

			accTag := widget.NewLabel("[user@gmail.com]")
			subjectLbl := widget.NewLabelWithStyle("Tiêu đề thư", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			midRow := container.NewHBox(accTag, subjectLbl)

			snippetLbl := widget.NewLabel("Trích đoạn nội dung thư...")

			return container.NewVBox(
				topRow,
				midRow,
				snippetLbl,
				widget.NewSeparator(),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			mu.Lock()
			if id < 0 || id >= len(emails) {
				mu.Unlock()
				return
			}
			m := emails[id]
			mu.Unlock()

			box := obj.(*fyne.Container)
			topRow := box.Objects[0].(*fyne.Container)
			senderLbl := topRow.Objects[0].(*widget.Label)
			dateLbl := topRow.Objects[2].(*widget.Label)

			midRow := box.Objects[1].(*fyne.Container)
			accTag := midRow.Objects[0].(*widget.Label)
			subjectLbl := midRow.Objects[1].(*widget.Label)

			snippetLbl := box.Objects[2].(*widget.Label)

			senderLbl.SetText(fmt.Sprintf("✉️ %s", m.From))
			dateLbl.SetText(fmt.Sprintf("📅 %s", m.Date))
			accTag.SetText(fmt.Sprintf("📬 %s", m.AccountEmail))
			subjectLbl.SetText(m.Subject)

			snippet := m.Snippet
			if snippet == "" {
				snippet = m.Body
			}
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			snippet = strings.ReplaceAll(snippet, "\r", " ")
			if len(snippet) > 85 {
				snippet = snippet[:85] + "..."
			}

			if len(m.ParsedData) > 0 {
				var tags []string
				for k, v := range m.ParsedData {
					tags = append(tags, fmt.Sprintf("%s: %v", k, v))
				}
				snippetLbl.SetText(fmt.Sprintf("🎯 [%s]  —  %s", strings.Join(tags, ", "), snippet))
			} else {
				snippetLbl.SetText(snippet)
			}
		},
	)

	// Chi tiết thư
	detailSubject = widget.NewLabelWithStyle("Chọn một email trong danh sách để xem chi tiết", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detailSubject.Wrapping = fyne.TextWrapWord

	detailFrom = widget.NewLabel("")
	detailFrom.Wrapping = fyne.TextWrapWord

	detailAccount = widget.NewLabel("")
	detailDate = widget.NewLabel("")

	detailParsed = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detailParsed.Hide()

	detailBody = widget.NewMultiLineEntry()
	detailBody.Wrapping = fyne.TextWrapWord
	detailBody.Disable()

	mailListWidget.OnSelected = func(id widget.ListItemID) {
		mu.Lock()
		if id < 0 || id >= len(emails) {
			mu.Unlock()
			return
		}
		m := emails[id]
		mu.Unlock()

		detailSubject.SetText(m.Subject)
		detailFrom.SetText(fmt.Sprintf("👤 Người gửi: %s", m.From))
		detailAccount.SetText(fmt.Sprintf("📥 Hộp thư nhận: %s", m.AccountEmail))
		detailDate.SetText(fmt.Sprintf("⏱️ Thời gian: %s", m.Date))
		detailBody.SetText(m.Body)

		if len(m.ParsedData) > 0 {
			var parts []string
			for k, v := range m.ParsedData {
				parts = append(parts, fmt.Sprintf("%s: %v", k, v))
			}
			detailParsed.SetText(fmt.Sprintf("🎯 DỮ LIỆU BÓC TÁCH (PARSER): [ %s ]", strings.Join(parts, "  |  ")))
			detailParsed.Show()
		} else {
			detailParsed.Hide()
		}
	}

	headerCard := container.NewVBox(
		detailSubject,
		container.NewHBox(detailFrom, layout.NewSpacer(), detailDate),
		detailAccount,
		detailParsed,
		widget.NewSeparator(),
	)

	detailView := container.NewBorder(
		headerCard,
		nil, nil, nil,
		detailBody,
	)

	rightSplit := container.NewVSplit(mailListWidget, detailView)
	rightSplit.SetOffset(0.38)

	mainMailSplit := container.NewHSplit(leftPanel, rightSplit)
	mainMailSplit.SetOffset(0.28)

	// ==================== TABS CHÍNH ====================
	apiTabPanel := buildAPITab(w)
	parserTabPanel := buildParserTab(w)
	sessionTabPanel := buildSessionTab(w)
	selfieTabPanel := buildSelfieTab(w)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Hộp Thư Đến", theme.HomeIcon(), mainMailSplit),
		container.NewTabItemWithIcon("Quản Lý Mail Parser", theme.DocumentCreateIcon(), parserTabPanel),
		container.NewTabItemWithIcon("Kho Ảnh Selfie", theme.FileImageIcon(), selfieTabPanel),
		container.NewTabItemWithIcon("Tiến Độ Phiên", theme.ViewFullScreenIcon(), sessionTabPanel),
		container.NewTabItemWithIcon("Cổng API Đọc Thư", theme.ComputerIcon(), apiTabPanel),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.SetOnClosed(stopAPIServer)

	if len(accounts) == 0 {
		go func() {
			time.Sleep(500 * time.Millisecond)
			showGuideDialog(w)
		}()
	}

	w.ShowAndRun()
}

// ==================== TAB CẤU HÌNH API MỞ CÔNG KHAI ====================
func buildAPITab(w fyne.Window) fyne.CanvasObject {
	portEntry = widget.NewEntry()
	portEntry.SetText("8088")

	lblAPIStatus = widget.NewLabelWithStyle("🔴 Trạng thái: ĐÃ DỪNG", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	lanIP := getLocalIP()
	lblLAN := widget.NewLabel(fmt.Sprintf("🏠 Mạng LAN: http://%s:8088", lanIP))
	lblWebDocs := widget.NewLabel("📖 Tài liệu Web: http://localhost:8088/api/document")

	// Tailscale UI
	detectTailscale()
	lblTailscale = widget.NewLabel("🦎 Tailscale: Đang kiểm tra...")
	btnCopyTS = widget.NewButtonWithIcon("Sao chép", theme.ContentCopyIcon(), func() {
		if tsIP != "" {
			p := portEntry.Text
			if p == "" {
				p = "8088"
			}
			urlStr := fmt.Sprintf("http://%s:%s", tsIP, p)
			w.Clipboard().SetContent(urlStr)
			dialog.ShowInformation("Đã sao chép", "Đã chép địa chỉ Tailscale vào bộ nhớ tạm:\n"+urlStr, w)
		}
	})
	btnCopyTS.Hide()

	if tsIP != "" {
		lblTailscale.SetText(fmt.Sprintf("🦎 Tailscale Mesh: http://%s:8088 (DNS: http://%s:8088)", tsIP, tsDNSName))
		btnCopyTS.Show()
	} else {
		lblTailscale.SetText("🦎 Tailscale: Chưa phát hiện đang chạy trên máy này")
	}

	lblPublicURL = widget.NewLabel("🌐 Public Internet: Chưa kích hoạt")
	btnCopyURL = widget.NewButtonWithIcon("Sao chép", theme.ContentCopyIcon(), func() {
		tunnelMu.Lock()
		u := publicURL
		if u == "" && tsFunnelURL != "" {
			u = tsFunnelURL
		}
		tunnelMu.Unlock()

		if u != "" {
			w.Clipboard().SetContent(u)
			dialog.ShowInformation("Đã sao chép", "Đã sao chép Public URL vào bộ nhớ tạm:\n"+u, w)
		}
	})
	btnCopyURL.Hide()

	checkTSFunnel = widget.NewCheck("Mở qua Tailscale Funnel (HTTPS chính chủ Tailscale)", func(checked bool) {
		apiMu.Lock()
		running := apiRunning
		apiMu.Unlock()

		if running {
			p := portEntry.Text
			if p == "" {
				p = "8088"
			}
			toggleTailscaleFunnel(checked, p, w)
		}
	})

	checkPublic = widget.NewCheck("Mở Public qua Tunnel (Không cần tài khoản, có HTTPS ngay)", func(checked bool) {
		apiMu.Lock()
		running := apiRunning
		apiMu.Unlock()

		if running {
			if checked {
				p := portEntry.Text
				if p == "" {
					p = "8088"
				}
				startPublicTunnel(p)
			} else {
				stopPublicTunnel()
			}
		}
	})
	checkPublic.SetChecked(true)

	btnToggleAPI = widget.NewButtonWithIcon("Khởi Chạy API Server", theme.MediaPlayIcon(), nil)
	btnToggleAPI.Importance = widget.HighImportance

	btnToggleAPI.OnTapped = func() {
		apiMu.Lock()
		running := apiRunning
		apiMu.Unlock()

		if running {
			stopAPIServer()
			lblAPIStatus.SetText("🔴 Trạng thái: ĐÃ DỪNG")
			btnToggleAPI.SetText("Khởi Chạy API Server")
			btnToggleAPI.SetIcon(theme.MediaPlayIcon())
			btnToggleAPI.Importance = widget.HighImportance
			portEntry.Enable()
			checkPublic.Enable()
			checkTSFunnel.Enable()
		} else {
			p := portEntry.Text
			if p == "" {
				p = "8088"
			}
			err := startAPIServer(p, w)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Không thể khởi động cổng %s: %v", p, err), w)
				return
			}
			lblAPIStatus.SetText(fmt.Sprintf("🟢 Trạng thái: ĐANG CHẠY (Cổng %s)", p))
			lblLAN.SetText(fmt.Sprintf("🏠 Mạng LAN: http://%s:%s", getLocalIP(), p))
			lblWebDocs.SetText(fmt.Sprintf("📖 Tài liệu Web: http://localhost:%s/api/document", p))

			detectTailscale()
			if tsIP != "" {
				lblTailscale.SetText(fmt.Sprintf("🦎 Tailscale Mesh: http://%s:%s (DNS: http://%s:%s)", tsIP, p, tsDNSName, p))
				btnCopyTS.Show()
			}

			btnToggleAPI.SetText("Dừng API Server")
			btnToggleAPI.SetIcon(theme.MediaStopIcon())
			btnToggleAPI.Importance = widget.DangerImportance
			portEntry.Disable()

			if checkPublic.Checked {
				startPublicTunnel(p)
			}
			if checkTSFunnel.Checked {
				toggleTailscaleFunnel(true, p, w)
			}
		}
	}

	// Tự động khởi chạy API Server ngay khi mở app (trên thread chính)
	if err := startAPIServer("8088", w); err == nil {
		lblAPIStatus.SetText("🟢 Trạng thái: ĐANG CHẠY (Cổng 8088)")
		btnToggleAPI.SetText("Dừng API Server")
		btnToggleAPI.SetIcon(theme.MediaStopIcon())
		btnToggleAPI.Importance = widget.DangerImportance
		portEntry.Disable()
	}

	btnOpenDocs := widget.NewButtonWithIcon("Mở Web Docs (/api/document)", theme.HelpIcon(), func() {
		p := portEntry.Text
		if p == "" {
			p = "8088"
		}
		targetURL := fmt.Sprintf("http://localhost:%s/api/document", p)
		if u, err := url.Parse(targetURL); err == nil {
			fyne.CurrentApp().OpenURL(u)
		}
	})

	configForm := container.NewVBox(
		container.NewHBox(
			widget.NewLabelWithStyle("⚡ Cổng lắng nghe (Port):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			portEntry,
			layout.NewSpacer(),
			widget.NewLabelWithStyle("🔓 Bảo mật: MỞ CÔNG KHAI (Không cần Token)", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
			layout.NewSpacer(),
			lblAPIStatus,
		),
		container.NewHBox(
			checkPublic,
			checkTSFunnel,
			layout.NewSpacer(),
			btnToggleAPI,
		),
		container.NewHBox(
			lblLAN,
			layout.NewSpacer(),
			lblTailscale,
			btnCopyTS,
		),
		container.NewHBox(
			lblPublicURL,
			layout.NewSpacer(),
			btnCopyURL,
		),
		container.NewHBox(
			lblWebDocs,
			layout.NewSpacer(),
			btnOpenDocs,
		),
		widget.NewSeparator(),
	)

	// Tài liệu API
	docsMD := `### 🌐 Cổng Web & Tài Liệu Trực Quan:
👉 **Truy cập trên trình duyệt:** ` + "`" + `http://localhost:8088/api/document` + "`" + `
*(Giao diện web hỗ trợ nút bấm kiểm thử trực tiếp, tự động lấy danh sách email, đọc thư dạng thẻ và JSON thô!)*

---
### 📡 Hướng Dẫn Sử Dụng API Đọc Thư (Công Khai - Không Cần Token)

Hệ thống cho phép gọi API trực tiếp không cần Header Authorization hay API Key:

---
#### 1. Lấy danh sách email hiện có:
` + "```http" + `
GET /api/getemaillist
` + "```" + `
*Kết quả trả về:* Mảng danh sách các địa chỉ Gmail đã lưu:
` + "```json" + `
[
  "user1@gmail.com",
  "user2@gmail.com"
]
` + "```" + `

---
#### 2. Đọc email từ một địa chỉ cụ thể:
` + "```http" + `
POST /api/getemail
Content-Type: application/json

{
  "email": "user1@gmail.com",
  "limit": 5
}
` + "```" + `
*Hoặc gọi nhanh qua GET:*
` + "```http" + `
GET /api/getemail?email=user1@gmail.com&limit=5
` + "```" + `
- **email** *(bắt buộc)*: Địa chỉ Gmail cần đọc.
- **limit** *(tuỳ chọn)*: Số email mới nhất (mặc định: 5, tối đa: 50).

---
#### 💻 Ví dụ gọi qua cURL:
` + "```bash" + `
# 1. Lấy danh sách email:
curl http://localhost:8088/api/getemaillist

# 2. Đọc thư của một tài khoản:
curl "http://localhost:8088/api/getemail?email=user1@gmail.com&limit=5"
` + "```" + `

#### 🐍 Ví dụ gọi bằng Python:
` + "```python" + `
import requests

BASE_URL = "http://localhost:8088"  # hoặc IP Tailscale / Public URL

# 1. Lấy danh sách email
emails = requests.get(f"{BASE_URL}/api/getemaillist").json()
print("Danh sách email:", emails)

# 2. Đọc email đầu tiên
if emails:
    payload = {"email": emails[0], "limit": 5}
    res = requests.post(f"{BASE_URL}/api/getemail", json=payload)
    print("Các thư mới:", res.json())
` + "```" + `
`
	docsWidget := widget.NewRichTextFromMarkdown(docsMD)
	docsWidget.Wrapping = fyne.TextWrapWord
	docsScroll := container.NewVScroll(docsWidget)

	// Khung Live Logs
	apiLogEntry = widget.NewMultiLineEntry()
	apiLogEntry.Wrapping = fyne.TextWrapWord
	apiLogEntry.Disable()
	apiLogEntry.SetText("Sẵn sàng. Nhấn 'Khởi Chạy API Server' để bắt đầu mở cổng...")

	btnClearLog := widget.NewButtonWithIcon("Xóa nhật ký", theme.DeleteIcon(), func() {
		apiLogsMu.Lock()
		apiLogs = nil
		apiLogsMu.Unlock()
		apiLogEntry.SetText("")
	})

	logBox := container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("📋 NHẬT KÝ YÊU CẦU (REALTIME LOGS)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			btnClearLog,
		),
		nil, nil, nil,
		apiLogEntry,
	)

	bodySplit := container.NewHSplit(docsScroll, logBox)
	bodySplit.SetOffset(0.48)

	return container.NewBorder(
		configForm,
		nil, nil, nil,
		bodySplit,
	)
}

func detectTailscale() {
	tsPath := `C:\Program Files\Tailscale\tailscale.exe`
	if _, err := os.Stat(tsPath); err != nil {
		p, err := exec.LookPath("tailscale")
		if err != nil {
			return
		}
		tsPath = p
	}

	outIP, err := exec.Command(tsPath, "ip", "-4").Output()
	if err == nil {
		tsIP = strings.TrimSpace(string(outIP))
	}

	outJSON, err := exec.Command(tsPath, "status", "--json").Output()
	if err == nil {
		var st struct {
			Self struct {
				DNSName string `json:"DNSName"`
			} `json:"Self"`
		}
		if json.Unmarshal(outJSON, &st) == nil {
			tsDNSName = strings.TrimSuffix(st.Self.DNSName, ".")
		}
	}
}

func stopTailscaleFunnel() {
	tsPath := `C:\Program Files\Tailscale\tailscale.exe`
	if _, err := os.Stat(tsPath); err != nil {
		p, err := exec.LookPath("tailscale")
		if err != nil {
			return
		}
		tsPath = p
	}
	_ = exec.Command(tsPath, "funnel", "reset").Run()
	_ = exec.Command(tsPath, "serve", "reset").Run()
	tsFunnelURL = ""
}

func toggleTailscaleFunnel(enable bool, port string, w fyne.Window) {
	tsPath := `C:\Program Files\Tailscale\tailscale.exe`
	if _, err := os.Stat(tsPath); err != nil {
		p, err := exec.LookPath("tailscale")
		if err != nil {
			appendAPILog("❌ Không tìm thấy tailscale.exe trên máy")
			return
		}
		tsPath = p
	}

	if enable {
		appendAPILog("🦎 Đang khởi động Tailscale Funnel cho cổng %s...", port)
		cmd := exec.Command(tsPath, "funnel", "--bg", "--yes", port)
		out, err := cmd.CombinedOutput()
		outStr := strings.TrimSpace(string(out))

		if err != nil || strings.Contains(outStr, "not enabled") {
			appendAPILog("⚠ Tailscale Funnel cần cấp quyền: %s", outStr)

			var authURL string
			for _, line := range strings.Split(outStr, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "https://login.tailscale.com/") {
					authURL = line
					break
				}
			}

			msg := "Tính năng Funnel chưa được bật trên Tailscale của bạn.\n\n" + outStr
			if authURL != "" {
				dialog.ShowConfirm("Bật Tailscale Funnel", msg+"\n\nBạn có muốn mở trình duyệt đến trang Tailscale Admin để cấp phép ngay không?", func(ok bool) {
					if ok {
						if u, e := url.Parse(authURL); e == nil {
							fyne.CurrentApp().OpenURL(u)
						}
					}
				}, w)
			} else {
				dialog.ShowInformation("Thông báo Tailscale", msg, w)
			}
			return
		}

		if tsDNSName != "" {
			tsFunnelURL = "https://" + tsDNSName
		}
		appendAPILog("🎉 Tailscale Funnel đang phát sóng HTTPS: %s", tsFunnelURL)
		if lblPublicURL != nil && tsFunnelURL != "" {
			lblPublicURL.SetText(fmt.Sprintf("🌐 Tailscale HTTPS: %s", tsFunnelURL))
			btnCopyURL.Show()
		}
	} else {
		stopTailscaleFunnel()
		appendAPILog("⏹ Đã dừng Tailscale Funnel.")
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
					return ip
				}
			}
		}
	}
	return "127.0.0.1"
}

func appendAPILog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, msg)

	// In ra terminal để xem log trên Docker / Back4app / Cloud
	log.Println(line)

	apiLogsMu.Lock()
	apiLogs = append(apiLogs, line)
	if len(apiLogs) > 200 {
		apiLogs = apiLogs[len(apiLogs)-200:]
	}
	text := strings.Join(apiLogs, "\n")
	apiLogsMu.Unlock()

	if apiLogEntry != nil {
		apiLogEntry.SetText(text)
	}
}

func startPublicTunnel(localPort string) {
	tunnelMu.Lock()
	if tunnelCmd != nil {
		tunnelMu.Unlock()
		return
	}
	tunnelMu.Unlock()

	appendAPILog("🌐 Đang kết nối dịch vụ Public Tunnel qua localhost.run...")
	if lblPublicURL != nil {
		lblPublicURL.SetText("🌐 Public URL: Đang kết nối...")
	}

	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ServerAliveInterval=30",
		"-R", fmt.Sprintf("80:localhost:%s", localPort),
		"nokey@localhost.run",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		appendAPILog("❌ Lỗi tạo pipe tunnel: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		appendAPILog("❌ Không thể khởi động ssh tunnel: %v", err)
		if lblPublicURL != nil {
			lblPublicURL.SetText("🌐 Public URL: Lỗi khởi động SSH")
		}
		return
	}

	tunnelMu.Lock()
	tunnelCmd = cmd
	tunnelMu.Unlock()

	go func() {
		scanner := bufio.NewScanner(stdout)
		re := regexp.MustCompile(`https://[a-zA-Z0-9\.\-]+\.lhr\.life`)

		found := false
		for scanner.Scan() {
			line := scanner.Text()
			match := re.FindString(line)
			if match != "" {
				tunnelMu.Lock()
				publicURL = match
				tunnelMu.Unlock()

				found = true
				appendAPILog("🎉 Public URL đã sẵn sàng: %s", publicURL)
				if lblPublicURL != nil {
					lblPublicURL.SetText(fmt.Sprintf("🌐 Public URL: %s", publicURL))
				}
				if btnCopyURL != nil {
					btnCopyURL.Show()
				}
				break
			}
		}

		if !found {
			appendAPILog("⚠ Không nhận được URL từ dịch vụ tunnel")
		}

		_ = cmd.Wait()

		tunnelMu.Lock()
		tunnelCmd = nil
		publicURL = ""
		tunnelMu.Unlock()

		if lblPublicURL != nil {
			lblPublicURL.SetText("🌐 Public URL: Đã ngắt kết nối")
		}
		if btnCopyURL != nil {
			btnCopyURL.Hide()
		}
	}()
}

func stopPublicTunnel() {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()

	if tunnelCmd != nil && tunnelCmd.Process != nil {
		_ = tunnelCmd.Process.Kill()
		tunnelCmd = nil
		publicURL = ""
		appendAPILog("⏹ Đã ngắt Public Tunnel.")
	}
	if lblPublicURL != nil {
		lblPublicURL.SetText("🌐 Public URL: Chưa kích hoạt")
	}
	if btnCopyURL != nil {
		btnCopyURL.Hide()
	}
}

func startAPIServer(port string, w fyne.Window) error {
	apiMu.Lock()
	defer apiMu.Unlock()

	if apiRunning {
		return nil
	}

	mux := http.NewServeMux()

	setCors := func(rw http.ResponseWriter) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Headers", "*")
		rw.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	}

	// 1. GET /api/getemaillist -> Danh sách các email hiện có trong app
	mux.HandleFunc("/api/getemaillist", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		mu.Lock()
		var emailList []string
		for _, a := range accounts {
			emailList = append(emailList, a.Email)
		}
		mu.Unlock()

		appendAPILog("✔ 200 OK: /api/getemaillist (%d email) từ %s", len(emailList), r.RemoteAddr)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(emailList)
	})

	// Alias /api/accounts cho tương thích
	mux.HandleFunc("/api/accounts", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}
		mu.Lock()
		var emailList []string
		for _, a := range accounts {
			emailList = append(emailList, a.Email)
		}
		mu.Unlock()
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(emailList)
	})

	// POST /api/addaccount -> Thêm hoặc cập nhật tài khoản Gmail
	handleAddAccount := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(rw).Encode(map[string]string{"error": "Chỉ chấp nhận phương thức POST"})
			return
		}

		var payload struct {
			Email       string `json:"email"`
			AppPassword string `json:"appPassword"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{"error": "Dữ liệu JSON không hợp lệ"})
			return
		}

		email := strings.TrimSpace(payload.Email)
		pass := strings.TrimSpace(payload.AppPassword)

		if email == "" || pass == "" {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{"error": "Vui lòng nhập đầy đủ Email và Mật khẩu ứng dụng 16 ký tự"})
			return
		}

		mu.Lock()
		updated := false
		for i, acc := range accounts {
			if strings.EqualFold(acc.Email, email) {
				accounts[i].AppPassword = pass
				updated = true
				break
			}
		}

		if !updated {
			accounts = append(accounts, Account{
				Email:       email,
				AppPassword: pass,
			})
		}

		saveAccounts()
		if accListWidget != nil {
			accListWidget.Refresh()
		}
		mu.Unlock()

		appendAPILog("✔ Đã lưu tài khoản Gmail qua API: %s", email)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"success": true,
			"message": "Đã lưu tài khoản thành công",
			"email":   email,
			"updated": updated,
		})
	}

	mux.HandleFunc("/api/addaccount", handleAddAccount)

	// DELETE hoặc POST /api/account
	mux.HandleFunc("/api/account", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodPost {
			handleAddAccount(rw, r)
			return
		}

		if r.Method == http.MethodDelete {
			email := strings.TrimSpace(r.URL.Query().Get("email"))
			if email == "" {
				var bodyPayload struct {
					Email string `json:"email"`
				}
				_ = json.NewDecoder(r.Body).Decode(&bodyPayload)
				email = strings.TrimSpace(bodyPayload.Email)
			}

			if email == "" {
				rw.WriteHeader(http.StatusBadRequest)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]string{"error": "Thiếu tham số 'email'"})
				return
			}

			mu.Lock()
			newAccounts := make([]Account, 0, len(accounts))
			found := false
			for _, acc := range accounts {
				if strings.EqualFold(acc.Email, email) {
					found = true
					continue
				}
				newAccounts = append(newAccounts, acc)
			}

			if !found {
				mu.Unlock()
				rw.WriteHeader(http.StatusNotFound)
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]string{"error": "Không tìm thấy tài khoản để xóa"})
				return
			}

			accounts = newAccounts
			saveAccounts()
			if accListWidget != nil {
				accListWidget.Refresh()
			}
			mu.Unlock()

			appendAPILog("🗑 Đã xóa tài khoản Gmail qua API: %s", email)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": true,
				"message": "Đã xóa tài khoản thành công",
				"email":   email,
			})
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
	})

	// 2. /api/getemail -> Nhận data là địa chỉ email để đọc các email của tài khoản đó
	mux.HandleFunc("/api/getemail", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		var targetEmail string
		var limit uint32 = 5

		// Lấy từ Query Params (?email=... hoặc ?data=...)
		if q := r.URL.Query().Get("email"); q != "" {
			targetEmail = strings.TrimSpace(q)
		} else if q := r.URL.Query().Get("data"); q != "" {
			targetEmail = strings.TrimSpace(q)
		}

		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 50 {
				limit = uint32(l)
			}
		}

		// Nếu là POST, đọc từ Body (JSON hoặc Raw String)
		if r.Method == http.MethodPost {
			bodyBytes, _ := io.ReadAll(r.Body)
			if len(bodyBytes) > 0 {
				var jsonBody struct {
					Email string `json:"email"`
					Data  string `json:"data"`
					Limit int    `json:"limit"`
				}
				if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
					if jsonBody.Email != "" {
						targetEmail = strings.TrimSpace(jsonBody.Email)
					} else if jsonBody.Data != "" {
						targetEmail = strings.TrimSpace(jsonBody.Data)
					}
					if jsonBody.Limit > 0 && jsonBody.Limit <= 50 {
						limit = uint32(jsonBody.Limit)
					}
				} else {
					rawStr := strings.TrimSpace(string(bodyBytes))
					if strings.Contains(rawStr, "@") {
						targetEmail = rawStr
					}
				}
			}
		}

		// Kiểm tra nếu không truyền email
		if targetEmail == "" {
			appendAPILog("⚠ 400 Bad Request: /api/getemail thiếu địa chỉ email từ %s", r.RemoteAddr)
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(rw).Encode(map[string]string{
				"error": "Vui lòng cung cấp địa chỉ email (ví dụ: POST {\"email\": \"user@gmail.com\"} hoặc GET ?email=user@gmail.com)",
			})
			return
		}

		// Tìm tài khoản trong danh sách
		mu.Lock()
		var matchedAccount *Account
		for _, a := range accounts {
			if strings.EqualFold(a.Email, targetEmail) {
				matchedAccount = &Account{Email: a.Email, AppPassword: a.AppPassword}
				break
			}
		}
		mu.Unlock()

		if matchedAccount == nil {
			appendAPILog("⚠ 404: Không tìm thấy tài khoản '%s' trong ứng dụng", targetEmail)
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusNotFound)
			json.NewEncoder(rw).Encode(map[string]string{
				"error": fmt.Sprintf("Tài khoản %s chưa được cấu hình trong app", targetEmail),
			})
			return
		}

		appendAPILog("⏳ Đang đọc thư cho %s (limit: %d)...", targetEmail, limit)
		fetched := fetchFromSingleAccount(*matchedAccount, limit)

		appendAPILog("✔ 200 OK: /api/getemail - Trả về %d thư cho %s", len(fetched), targetEmail)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(fetched)
	})

	// Alias /api/emails cho tương thích
	mux.HandleFunc("/api/emails", func(rw http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/api/getemail"
		mux.ServeHTTP(rw, r)
	})

	// 3. GET /api/status -> Kiểm tra máy chủ
	mux.HandleFunc("/api/status", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		mu.Lock()
		accCount := len(accounts)
		mu.Unlock()

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"status":         "running",
			"accounts_count": accCount,
			"tailscale_ip":   tsIP,
			"tailscale_dns":  tsDNSName,
			"public_url":     publicURL,
			"time":           time.Now().Format("2006-01-02 15:04:05"),
		})
	})

	// 4. GET /api/document -> Giao diện web & tài liệu kiểm thử trực tiếp
	mux.HandleFunc("/api/document", handleAPIDocument)
	mux.HandleFunc("/api/docs", handleAPIDocument)
	mux.HandleFunc("/api/doc", handleAPIDocument)
	mux.HandleFunc("/docs", handleAPIDocument)
	mux.HandleFunc("/document", handleAPIDocument)

	// 5. GET /api/parsers -> Danh sách các bộ Mail Parser
	mux.HandleFunc("/api/parsers", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}
		parsersMu.Lock()
		active := make([]MailParser, len(parsers))
		copy(active, parsers)
		parsersMu.Unlock()

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(active)
	})

	// 6. POST /api/parse -> Bóc tách văn bản theo các Parser
	mux.HandleFunc("/api/parse", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		var req struct {
			Text    string `json:"text"`
			Subject string `json:"subject"`
			From    string `json:"from"`
		}
		_ = json.Unmarshal(bodyBytes, &req)
		if req.Text == "" {
			req.Text = string(bodyBytes)
		}

		dummy := EmailItem{
			Body:    req.Text,
			Snippet: req.Text,
			Subject: req.Subject,
			From:    req.From,
		}

		extracted := ExecuteAllParsers(dummy)
		if extracted == nil {
			extracted = make(map[string]interface{})
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"success": true,
			"parsed":  extracted,
		})
	})

	// 7. POST /api/session/report (hoặc /api/progress) -> Báo cáo tiến độ phiên (Hỗ trợ kèm ảnh)
	handleSessionReport := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
		}

		var req SessionProgress

		// Kiểm tra nếu là multipart/form-data (Gửi kèm file ảnh trực tiếp)
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(32 << 20); err == nil {
				if v := r.FormValue("id"); v != "" {
					req.ID = v
				}
				if v := r.FormValue("name"); v != "" {
					req.Name = v
				}
				if v := r.FormValue("status"); v != "" {
					req.Status = v
				}
				if v := r.FormValue("message"); v != "" {
					req.Message = v
				}
				if v := r.FormValue("current"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						req.Current = n
					}
				}
				if v := r.FormValue("total"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						req.Total = n
					}
				}
				if v := r.FormValue("image"); v != "" {
					req.Image = v
				}

				// Đọc file ảnh upload
				file, header, err := r.FormFile("image")
				if err != nil {
					file, header, err = r.FormFile("file")
				}
				if err == nil && file != nil {
					defer file.Close()
					dataBytes, _ := io.ReadAll(file)
					if len(dataBytes) > 0 {
						targetID := req.ID
						if targetID == "" {
							targetID = fmt.Sprintf("sess-%d", time.Now().UnixNano()/1000000)
							req.ID = targetID
						}
						savedPath, err := SaveSessionImage(targetID, req.Current, header.Filename, dataBytes)
						if err == nil {
							req.ImagePath = savedPath
							req.ImageURL = "/api/session/image?id=" + targetID
							req.Image = req.ImageURL
						}
					}
				}
			}
		} else if r.Body != nil {
			// Đọc từ Body JSON
			bodyBytes, _ := io.ReadAll(r.Body)
			if len(bodyBytes) > 0 {
				_ = json.Unmarshal(bodyBytes, &req)
			}
		}

		// Cho phép ghi đè hoặc bổ sung bằng Query Params để test nhanh
		q := r.URL.Query()
		if v := q.Get("id"); v != "" {
			req.ID = v
		}
		if v := q.Get("name"); v != "" {
			req.Name = v
		}
		if v := q.Get("status"); v != "" {
			req.Status = v
		}
		if v := q.Get("message"); v != "" {
			req.Message = v
		}
		if v := q.Get("current"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.Current = n
			}
		}
		if v := q.Get("total"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.Total = n
			}
		}
		if v := q.Get("image"); v != "" {
			req.Image = v
		}

		sess := CreateOrUpdateSession(req, clientIP)

		imgInfo := ""
		if sess.ImagePath != "" {
			imgInfo = " [Kèm ảnh]"
		}
		appendAPILog("📊 Tiến độ: '%s' [%d/%d - %.1f%%]%s: %s", sess.Name, sess.Current, sess.Total, sess.Percent, imgInfo, sess.Message)

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"success":  true,
			"session":  sess,
			"imageUrl": sess.ImageURL,
			"hasImage": sess.ImagePath != "",
		})
	}

	mux.HandleFunc("/api/session/report", handleSessionReport)
	mux.HandleFunc("/api/progress", handleSessionReport)

	// 8. GET /api/session/image & /api/session/image/latest -> Phục vụ xem live ảnh
	handleSessionImage := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		q := r.URL.Query()
		id := strings.TrimSpace(q.Get("id"))
		indexStr := strings.TrimSpace(q.Get("index"))
		isLatest := q.Get("latest") == "1" || q.Get("latest") == "true" || id == "latest"

		var targetPath string
		if indexStr != "" {
			idx, err := strconv.Atoi(indexStr)
			if err == nil {
				targetPath = GetSessionImagePathByIndex(id, idx)
			}
		} else if isLatest || id == "" {
			targetPath = GetSessionImagePathByIndex(id, 0)
		} else {
			sess, found := GetSessionByID(id)
			if found && sess != nil && sess.ImagePath != "" && sessionFileExists(sess.ImagePath) {
				targetPath = sess.ImagePath
			} else {
				safeID := sanitizeSessionFilename(id)
				for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
					p := filepath.Join(sessionImagesDir, safeID+ext)
					if sessionFileExists(p) {
						targetPath = p
						break
					}
				}
			}
		}

		if targetPath == "" || !sessionFileExists(targetPath) {
			rw.WriteHeader(http.StatusNotFound)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{
				"error": "Chưa có ảnh báo cáo cho phiên này hoặc chưa có ảnh nào được gửi.",
			})
			return
		}

		// Disable cache để live preview luôn cập nhật ảnh mới nhất
		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		rw.Header().Set("Pragma", "no-cache")
		rw.Header().Set("Expires", "0")
		http.ServeFile(rw, r, targetPath)
	}

	mux.HandleFunc("/api/session/image", handleSessionImage)
	mux.HandleFunc("/api/session/image/latest", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		targetPath := GetLiveImagePathByIndex(0)

		if targetPath == "" || !sessionFileExists(targetPath) {
			rw.WriteHeader(http.StatusNotFound)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{
				"error": "Chưa có ảnh phiên nào được gửi tới server.",
			})
			return
		}

		rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(rw, r, targetPath)
	})

	// Trả về danh sách 3 ảnh gần nhất của cùng 1 phiên
	mux.HandleFunc("/api/session/images", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}
		qID := strings.TrimSpace(r.URL.Query().Get("id"))
		items := GetSessionImages(qID)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(items)
	})

	// 9. GET /api/session/list (hoặc /api/sessions) -> Lấy danh sách toàn bộ phiên
	handleSessionList := func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}
		list := GetSessionList()
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(list)
	}

	mux.HandleFunc("/api/session/list", handleSessionList)
	mux.HandleFunc("/api/sessions", handleSessionList)

	// 10. /api/session -> Xem chi tiết hoặc xóa phiên
	mux.HandleFunc("/api/session", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		q := r.URL.Query()
		id := strings.TrimSpace(q.Get("id"))
		action := strings.TrimSpace(q.Get("action"))

		// DELETE yêu cầu
		if r.Method == http.MethodDelete || action == "delete" {
			if action == "clear_completed" || q.Get("clear") == "completed" {
				count := ClearCompletedSessions()
				rw.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rw).Encode(map[string]interface{}{
					"success": true,
					"cleared": count,
				})
				return
			}
			if id == "" {
				rw.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(rw).Encode(map[string]string{"error": "Thiếu tham số 'id'"})
				return
			}
			ok := DeleteSession(id)
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]interface{}{
				"success": ok,
			})
			return
		}

		// GET chi tiết một phiên
		if id != "" {
			sess, found := GetSessionByID(id)
			if !found {
				rw.WriteHeader(http.StatusNotFound)
				json.NewEncoder(rw).Encode(map[string]string{"error": "Không tìm thấy phiên với ID: " + id})
				return
			}
			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(sess)
			return
		}

		// Nếu không truyền ID thì trả về toàn bộ danh sách phiên
		handleSessionList(rw, r)
	})

	// Đăng ký API Kho Ảnh Selfie
	registerSelfieAPIRoutes(mux, setCors)

	// Endpoint Healthcheck cho Back4app / Cloud Platforms
	mux.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		setCors(rw)
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(`{"status":"healthy","uptime":"ok"}`))
	})

	// Phục vụ Giao diện Web React Frontend (nếu đã build trong thư mục frontend/dist)
	frontendDist := filepath.Join(".", "frontend", "dist")
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		// Không can thiệp các API endpoint
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(rw, r)
			return
		}

		// Kiểm tra thư mục frontend/dist có index.html không
		indexPath := filepath.Join(frontendDist, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			// Yêu cầu root "/"
			if r.URL.Path == "/" {
				http.ServeFile(rw, r, indexPath)
				return
			}

			// Kiểm tra file tài nguyên tĩnh tương ứng (.js, .css, .svg, ...)
			reqFile := filepath.Join(frontendDist, filepath.Clean(r.URL.Path))
			if info, err := os.Stat(reqFile); err == nil && !info.IsDir() {
				http.ServeFile(rw, r, reqFile)
				return
			}

			// SPA Fallback: Trả về index.html cho các route phía client
			http.ServeFile(rw, r, indexPath)
			return
		}

		// Nếu là root "/" và chưa có frontend dist: Trả về 200 OK để Back4app Healthcheck thành công
		if r.URL.Path == "/" {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status":"healthy","app":"Gmail Hub & Selfie Vault API","docs":"/api/document"}`))
			return
		}
		http.NotFound(rw, r)
	})

	listener, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		appendAPILog("❌ Lỗi mở port %s: %v", port, err)
		return err
	}

	apiServer = &http.Server{Handler: mux}
	apiRunning = true

	go func() {
		if err := apiServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			appendAPILog("❌ Lỗi server: %v", err)
			apiMu.Lock()
			apiRunning = false
			apiMu.Unlock()
		}
	}()

	appendAPILog("🚀 API Server đã mở công khai trên 0.0.0.0:%s", port)
	appendAPILog("📸 API Kho Ảnh Selfie: http://localhost:%s/api/selfies", port)
	appendAPILog("📖 Tài liệu Web & Tester: http://localhost:%s/api/document", port)
	if tsIP != "" {
		appendAPILog("🦎 Tailscale Mesh: http://%s:%s", tsIP, port)
	}
	return nil
}

func stopAPIServer() {
	stopPublicTunnel()
	stopTailscaleFunnel()

	apiMu.Lock()
	defer apiMu.Unlock()
	if !apiRunning || apiServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = apiServer.Shutdown(ctx)
	apiServer = nil
	apiRunning = false
	appendAPILog("⏹ API Server đã dừng.")
}

// ==================== LOGIC ĐỌC EMAIL IMAP ====================
func fetchAllEmails(w fyne.Window) {
	mu.Lock()
	if len(accounts) == 0 {
		mu.Unlock()
		dialog.ShowInformation("Thông báo", "Chưa có tài khoản nào! Hãy thêm tài khoản trước.", w)
		return
	}
	accs := make([]Account, len(accounts))
	copy(accs, accounts)
	mu.Unlock()

	syncBtn.Disable()
	progressBar.Show()
	statusLabel.SetText("Đang kết nối & quét thư...")

	var fetchedList []EmailItem
	var fetchMu sync.Mutex
	var wg sync.WaitGroup

	for _, acc := range accs {
		wg.Add(1)
		go func(a Account) {
			defer wg.Done()
			res := fetchFromSingleAccount(a, 6)
			fetchMu.Lock()
			fetchedList = append(fetchedList, res...)
			fetchMu.Unlock()
		}(acc)
	}
	wg.Wait()

	mu.Lock()
	emails = fetchedList
	count := len(emails)
	mu.Unlock()

	progressBar.Hide()
	syncBtn.Enable()
	statusLabel.SetText(fmt.Sprintf("Đã nạp %d thư lúc %s", count, time.Now().Format("15:04:05")))
	mailListWidget.Refresh()
	refreshTestEmailOptions()
}

func fetchFromSingleAccount(acc Account, limit uint32) []EmailItem {
	var list []EmailItem

	c, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		log.Printf("[%s] Lỗi kết nối IMAP: %v", acc.Email, err)
		return list
	}
	defer c.Logout()

	if err := c.Login(acc.Email, acc.AppPassword); err != nil {
		log.Printf("[%s] Lỗi đăng nhập: %v", acc.Email, err)
		return list
	}

	mbox, err := c.Select("INBOX", true)
	if err != nil || mbox.Messages == 0 {
		return list
	}

	fromSeq := uint32(1)
	if mbox.Messages > limit {
		fromSeq = mbox.Messages - limit + 1
	}
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(fromSeq, mbox.Messages)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	msgChan := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqSet, items, msgChan)
	}()

	for msg := range msgChan {
		item := EmailItem{
			ID:           fmt.Sprintf("%s-%d", acc.Email, msg.SeqNum),
			AccountEmail: acc.Email,
			Subject:      "(Không có tiêu đề)",
			From:         "Không rõ",
			Date:         time.Now().Format("02/01/2006 15:04"),
		}

		if msg.Envelope != nil {
			if msg.Envelope.Subject != "" {
				item.Subject = msg.Envelope.Subject
			}
			if len(msg.Envelope.From) > 0 {
				f := msg.Envelope.From[0]
				if f.PersonalName != "" {
					item.From = fmt.Sprintf("%s <%s@%s>", f.PersonalName, f.MailboxName, f.HostName)
				} else {
					item.From = fmt.Sprintf("%s@%s", f.MailboxName, f.HostName)
				}
			}
			item.Date = msg.Envelope.Date.Format("02/01/2006 15:04")
		}

		r := msg.GetBody(section)
		if r != nil {
			mr, err := mail.CreateReader(r)
			if err == nil {
				for {
					p, err := mr.NextPart()
					if err == io.EOF || err != nil {
						break
					}
					switch p.Header.(type) {
					case *mail.InlineHeader:
						b, _ := io.ReadAll(p.Body)
						content := string(b)
						if item.Body == "" {
							item.Body = content
						}
					}
				}
			}
		}

		item.ParsedData = ExecuteAllParsers(item)
		list = append([]EmailItem{item}, list...)
	}
	<-done
	return list
}

func loadAccounts() {
	mu.Lock()
	defer mu.Unlock()
	data, err := os.ReadFile(accountsFile)
	if err == nil {
		_ = json.Unmarshal(data, &accounts)
	}
}

func saveAccounts() {
	data, _ := json.MarshalIndent(accounts, "", "  ")
	_ = os.WriteFile(accountsFile, data, 0644)
}

func showGuideDialog(w fyne.Window) {
	guideMD := `## 📖 Hướng Dẫn Cấu Hình Gmail

Để ứng dụng đọc được hộp thư Gmail, Google yêu cầu cấu hình **Mật khẩu ứng dụng** và bật **IMAP**:

---

### Bước 1: Bật Xác minh 2 bước
1. Vào trang tài khoản Google: [myaccount.google.com](https://myaccount.google.com)
2. Chọn mục **Bảo mật (Security)**.
3. Bật tính năng **Xác minh 2 bước (2-Step Verification)**.

### Bước 2: Tạo Mật khẩu ứng dụng (App Password)
1. Trong mục **Bảo mật**, gõ tìm kiếm: **"Mật khẩu ứng dụng"** (App Passwords).
2. Đặt tên ứng dụng (ví dụ: *GmailReader*), rồi nhấn **Tạo (Create)**.
3. Google sẽ cấp mã gồm **16 ký tự** (dạng *xxxx xxxx xxxx xxxx*).
4. Sao chép 16 ký tự này để nhập vào ô **Mật khẩu ứng dụng** trong app (không dùng mật khẩu đăng nhập chính).

### Bước 3: Bật giao thức IMAP trong Gmail
1. Truy cập [mail.google.com](https://mail.google.com) trên trình duyệt máy tính.
2. Nhấn vào biểu tượng **Bánh răng (Cài đặt)** > **Xem tất cả chế độ cài đặt**.
3. Chọn thẻ **Chuyển tiếp và POP/IMAP**.
4. Tích chọn **Bật IMAP (Enable IMAP)** rồi bấm **Lưu thay đổi**.

---
💡 *Sau khi hoàn thành 3 bước trên, nhập Email và Mật khẩu 16 ký tự vào ô bên trái rồi bấm **Thêm Gmail**, sau đó bấm **Quét Thư Tất Cả**!*
`
	rt := widget.NewRichTextFromMarkdown(guideMD)
	rt.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(rt)
	scroll.SetMinSize(fyne.NewSize(540, 380))

	d := dialog.NewCustom("Hướng Dẫn Cấu Hình Gmail", "Đã hiểu", container.NewPadded(scroll), w)
	d.Resize(fyne.NewSize(580, 480))
	d.Show()
}
