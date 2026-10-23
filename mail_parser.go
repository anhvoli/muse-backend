package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ==================== CẤU TRÚC DỮ LIỆU MAIL PARSER ====================

type ParserRule struct {
	FieldName string `json:"fieldName"` // Tên trường dữ liệu (ví dụ: otp, order_id, link, amount)
	Source    string `json:"source"`    // "body", "subject", "from"
	Type      string `json:"type"`      // "regex", "between"
	Pattern   string `json:"pattern"`   // Regex hoặc "StartKeyword||EndKeyword"
	Group     int    `json:"group"`     // Capture group (0: toàn bộ match, 1: group 1)
}

type MailParser struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`        // Tên bộ parser
	Description string       `json:"description"` // Mô tả mục đích
	FilterFrom  string       `json:"filterFrom"`  // Lọc theo người gửi (để trống = tất cả)
	FilterSubj  string       `json:"filterSubj"`  // Lọc theo tiêu đề (để trống = tất cả)
	Enabled     bool         `json:"enabled"`     // Bật / tắt parser
	Rules       []ParserRule `json:"rules"`       // Danh sách các quy tắc trích xuất
}

var (
	parsersFile          = "parsers.json"
	parsers              []MailParser
	parsersMu            sync.Mutex
	selectedParserIdx    = -1
	parserListWidget     *widget.List
	parserRulesContainer *fyne.Container

	// Form controls editor
	entryParserName       *widget.Entry
	entryParserDesc       *widget.Entry
	entryParserFilterFrom *widget.Entry
	entryParserFilterSubj *widget.Entry
	checkParserEnabled    *widget.Check

	// Live Tester controls
	testInputEntry  *widget.Entry
	testOutputEntry *widget.Entry
	testEmailSelect *widget.Select
)

// ==================== TẢI & LƯU TRỮ PARSERS ====================

func loadParsers() {
	parsersMu.Lock()
	defer parsersMu.Unlock()

	data, err := os.ReadFile(parsersFile)
	if err == nil && len(data) > 0 {
		if json.Unmarshal(data, &parsers) == nil && len(parsers) > 0 {
			return
		}
	}

	// Nếu file chưa có hoặc rỗng, tạo các bộ parser mẫu chuyên nghiệp
	parsers = []MailParser{
		{
			ID:          "p-otp",
			Name:        "🔑 Trích xuất Mã OTP & Mã Xác Nhận",
			Description: "Tự động phát hiện và bóc tách mã xác thực OTP 4-8 số từ nội dung thư",
			FilterFrom:  "",
			FilterSubj:  "",
			Enabled:     true,
			Rules: []ParserRule{
				{
					FieldName: "otp",
					Source:    "body",
					Type:      "regex",
					Pattern:   `(?i)(?:mã|code|otp|xác thực|xác minh|verification|passcode)[^\d]{0,25}(\b\d{4,8}\b)`,
					Group:     1,
				},
				{
					FieldName: "sender_name",
					Source:    "from",
					Type:      "regex",
					Pattern:   `^([^<]+)`,
					Group:     1,
				},
			},
		},
		{
			ID:          "p-link",
			Name:        "🔗 Trích xuất Link Kích Hoạt / Xác Nhận",
			Description: "Bóc tách các liên kết xác nhận tài khoản, kích hoạt hoặc đổi mật khẩu",
			FilterFrom:  "",
			FilterSubj:  "",
			Enabled:     true,
			Rules: []ParserRule{
				{
					FieldName: "action_url",
					Source:    "body",
					Type:      "regex",
					Pattern:   `https?://[^\s"'<>]+(?:verify|confirm|activate|token|reset-password|auth)[^\s"'<>]*`,
					Group:     0,
				},
			},
		},
		{
			ID:          "p-order",
			Name:        "📦 Trích xuất Đơn Hàng & Số Tiền",
			Description: "Bóc tách mã đơn hàng (order id) và số tiền thanh toán từ email hóa đơn",
			FilterFrom:  "",
			FilterSubj:  "",
			Enabled:     true,
			Rules: []ParserRule{
				{
					FieldName: "order_id",
					Source:    "body",
					Type:      "regex",
					Pattern:   `(?i)(?:mã đơn|đơn hàng|order\s*id|order\s*#)[\s:#]*([A-Za-z0-9_-]{5,25})`,
					Group:     1,
				},
				{
					FieldName: "amount",
					Source:    "body",
					Type:      "regex",
					Pattern:   `(?i)(\d{1,3}(?:[.,]\d{3})*(?:\s*(?:đ|vnd|vnđ|\$|usd)))`,
					Group:     1,
				},
			},
		},
	}

	saveParsersLocked()
}

func saveParsers() {
	parsersMu.Lock()
	defer parsersMu.Unlock()
	saveParsersLocked()
}

func saveParsersLocked() {
	data, err := json.MarshalIndent(parsers, "", "  ")
	if err == nil {
		_ = os.WriteFile(parsersFile, data, 0644)
	}
}

// ==================== CƠ CHẾ BÓC TÁCH (PARSING ENGINE) ====================

func (p *MailParser) Execute(email EmailItem) map[string]interface{} {
	if !p.Enabled {
		return nil
	}

	// Kiểm tra bộ lọc Người gửi (From)
	if p.FilterFrom != "" {
		if !strings.Contains(strings.ToLower(email.From), strings.ToLower(p.FilterFrom)) {
			return nil
		}
	}

	// Kiểm tra bộ lọc Tiêu đề (Subject)
	if p.FilterSubj != "" {
		if !strings.Contains(strings.ToLower(email.Subject), strings.ToLower(p.FilterSubj)) {
			return nil
		}
	}

	result := make(map[string]interface{})

	for _, rule := range p.Rules {
		if rule.FieldName == "" || rule.Pattern == "" {
			continue
		}

		// Xác định văn bản nguồn
		var sourceText string
		switch strings.ToLower(rule.Source) {
		case "subject":
			sourceText = email.Subject
		case "from":
			sourceText = email.From
		default:
			sourceText = email.Body
			if sourceText == "" {
				sourceText = email.Snippet
			}
		}

		if sourceText == "" {
			continue
		}

		// Xử lý theo loại bóc tách
		switch rule.Type {
		case "between":
			parts := strings.Split(rule.Pattern, "||")
			if len(parts) == 2 {
				startKey := parts[0]
				endKey := parts[1]
				startIdx := strings.Index(strings.ToLower(sourceText), strings.ToLower(startKey))
				if startIdx != -1 {
					sub := sourceText[startIdx+len(startKey):]
					endIdx := strings.Index(strings.ToLower(sub), strings.ToLower(endKey))
					if endIdx != -1 {
						result[rule.FieldName] = strings.TrimSpace(sub[:endIdx])
					} else {
						result[rule.FieldName] = strings.TrimSpace(sub)
					}
				}
			}
		default: // "regex"
			re, err := regexp.Compile(rule.Pattern)
			if err == nil {
				matches := re.FindStringSubmatch(sourceText)
				if len(matches) > 0 {
					val := ""
					if rule.Group > 0 && rule.Group < len(matches) {
						val = matches[rule.Group]
					} else if len(matches) > 1 {
						val = matches[1]
					} else {
						val = matches[0]
					}
					if val != "" {
						result[rule.FieldName] = strings.TrimSpace(val)
					}
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// ExecuteAllParsers chạy toàn bộ các parser đang bật và tổng hợp kết quả
func ExecuteAllParsers(email EmailItem) map[string]interface{} {
	parsersMu.Lock()
	activeParsers := make([]MailParser, len(parsers))
	copy(activeParsers, parsers)
	parsersMu.Unlock()

	merged := make(map[string]interface{})
	for _, p := range activeParsers {
		if p.Enabled {
			res := p.Execute(email)
			if res != nil {
				for k, v := range res {
					merged[k] = v
				}
			}
		}
	}

	if len(merged) == 0 {
		return nil
	}
	return merged
}

// ==================== GIAO DIỆN TAB MAIL PARSER ====================

func buildParserTab(w fyne.Window) fyne.CanvasObject {
	// Khởi tạo các ô nhập form chi tiết
	entryParserName = widget.NewEntry()
	entryParserName.SetPlaceHolder("Tên bộ bóc tách (ví dụ: Trích xuất OTP ngân hàng)")

	entryParserDesc = widget.NewEntry()
	entryParserDesc.SetPlaceHolder("Mô tả tóm tắt mục đích...")

	entryParserFilterFrom = widget.NewEntry()
	entryParserFilterFrom.SetPlaceHolder("Để trống nếu áp dụng cho mọi người gửi")

	entryParserFilterSubj = widget.NewEntry()
	entryParserFilterSubj.SetPlaceHolder("Để trống nếu áp dụng cho mọi tiêu đề thư")

	checkParserEnabled = widget.NewCheck("Kích hoạt Parser này", nil)
	checkParserEnabled.SetChecked(true)

	parserRulesContainer = container.NewVBox()

	// Khởi tạo Live Tester
	testInputEntry = widget.NewMultiLineEntry()
	testInputEntry.SetPlaceHolder("Dán nội dung email hoặc bấm nút bên trên để chọn thư từ Hộp Thư Đến...")
	testInputEntry.Wrapping = fyne.TextWrapWord

	testOutputEntry = widget.NewMultiLineEntry()
	testOutputEntry.SetPlaceHolder("Kết quả bóc tách dạng JSON sẽ hiển thị tại đây...")
	testOutputEntry.Wrapping = fyne.TextWrapWord
	testOutputEntry.Disable()

	testEmailSelect = widget.NewSelect([]string{"-- Chọn thư vừa đọc từ Hộp Thư --"}, func(selected string) {
		mu.Lock()
		defer mu.Unlock()
		for _, m := range emails {
			label := fmt.Sprintf("[%s] %s", m.Date, m.Subject)
			if label == selected {
				testInputEntry.SetText(fmt.Sprintf("From: %s\nSubject: %s\nDate: %s\n\n%s", m.From, m.Subject, m.Date, m.Body))
				break
			}
		}
	})

	// Danh sách Parser bên trái
	parserListWidget = widget.NewList(
		func() int {
			parsersMu.Lock()
			defer parsersMu.Unlock()
			return len(parsers)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.DocumentIcon())
			titleLbl := widget.NewLabelWithStyle("Tên Parser", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			subLbl := widget.NewLabel("0 quy tắc")
			subLbl.TextStyle.Italic = true

			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			delBtn.Importance = widget.LowImportance

			textVBox := container.NewVBox(titleLbl, subLbl)
			return container.NewBorder(nil, nil, icon, delBtn, textVBox)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			parsersMu.Lock()
			if id < 0 || id >= len(parsers) {
				parsersMu.Unlock()
				return
			}
			p := parsers[id]
			parsersMu.Unlock()

			border := obj.(*fyne.Container)
			vbox := border.Objects[0].(*fyne.Container)
			titleLbl := vbox.Objects[0].(*widget.Label)
			subLbl := vbox.Objects[1].(*widget.Label)
			delBtn := border.Objects[2].(*widget.Button)

			statusText := "🟢 BẬT"
			if !p.Enabled {
				statusText = "⚪ TẮT"
			}
			titleLbl.SetText(p.Name)
			subLbl.SetText(fmt.Sprintf("%s • %d quy tắc trích xuất", statusText, len(p.Rules)))

			delBtn.OnTapped = func() {
				dialog.ShowConfirm("Xác nhận xóa", fmt.Sprintf("Bạn có chắc chắn muốn xóa Parser '%s'?", p.Name), func(ok bool) {
					if ok {
						parsersMu.Lock()
						if id >= 0 && id < len(parsers) {
							parsers = append(parsers[:id], parsers[id+1:]...)
							saveParsersLocked()
						}
						parsersMu.Unlock()

						selectedParserIdx = -1
						clearParserEditor()
						parserListWidget.Refresh()
						parserListWidget.UnselectAll()
					}
				}, w)
			}
		},
	)

	parserListWidget.OnSelected = func(id widget.ListItemID) {
		parsersMu.Lock()
		if id < 0 || id >= len(parsers) {
			parsersMu.Unlock()
			return
		}
		selectedParserIdx = id
		p := parsers[id]
		parsersMu.Unlock()

		loadParserToEditor(p)
	}

	// Toolbar bên trái
	btnAddNewParser := widget.NewButtonWithIcon("Tạo Parser Mới", theme.ContentAddIcon(), func() {
		newParser := MailParser{
			ID:          fmt.Sprintf("p-%d", time.Now().UnixNano()),
			Name:        "Bộ Parser Mới",
			Description: "Mô tả quy tắc bóc tách",
			Enabled:     true,
			Rules: []ParserRule{
				{
					FieldName: "field_1",
					Source:    "body",
					Type:      "regex",
					Pattern:   `(\d+)`,
					Group:     1,
				},
			},
		}

		parsersMu.Lock()
		parsers = append(parsers, newParser)
		saveParsersLocked()
		newIdx := len(parsers) - 1
		parsersMu.Unlock()

		parserListWidget.Refresh()
		parserListWidget.Select(newIdx)
	})
	btnAddNewParser.Importance = widget.HighImportance

	btnPresets := widget.NewButtonWithIcon("Mẫu Có Sẵn", theme.HelpIcon(), func() {
		showPresetsDialog(w)
	})

	leftHeader := container.NewVBox(
		widget.NewLabelWithStyle("📑 QUẢN LÝ MAIL PARSER", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, btnAddNewParser, btnPresets),
		widget.NewSeparator(),
	)

	leftPanel := container.NewBorder(
		leftHeader,
		nil, nil, nil,
		parserListWidget,
	)

	// BÊN PHẢI: Form Chỉnh Sửa & Trình Test Thử Nghiệm
	btnAddRule := widget.NewButtonWithIcon("Thêm Trường Trích Xuất", theme.ContentAddIcon(), func() {
		addRuleRow(ParserRule{
			FieldName: "ten_truong",
			Source:    "body",
			Type:      "regex",
			Pattern:   "",
			Group:     1,
		})
	})

	btnSaveParser := widget.NewButtonWithIcon("Lưu Cấu Hình Parser", theme.DocumentSaveIcon(), func() {
		if selectedParserIdx < 0 {
			dialog.ShowInformation("Thông báo", "Vui lòng chọn một Parser từ danh sách bên trái hoặc nhấn 'Tạo Parser Mới'!", w)
			return
		}

		name := strings.TrimSpace(entryParserName.Text)
		if name == "" {
			dialog.ShowError(fmt.Errorf("Tên Parser không được để trống!"), w)
			return
		}

		rules := readRulesFromUI()

		parsersMu.Lock()
		if selectedParserIdx >= 0 && selectedParserIdx < len(parsers) {
			parsers[selectedParserIdx].Name = name
			parsers[selectedParserIdx].Description = entryParserDesc.Text
			parsers[selectedParserIdx].FilterFrom = entryParserFilterFrom.Text
			parsers[selectedParserIdx].FilterSubj = entryParserFilterSubj.Text
			parsers[selectedParserIdx].Enabled = checkParserEnabled.Checked
			parsers[selectedParserIdx].Rules = rules
			saveParsersLocked()
		}
		parsersMu.Unlock()

		parserListWidget.Refresh()
		dialog.ShowInformation("Thành công", fmt.Sprintf("Đã lưu cấu hình Parser '%s' thành công!", name), w)
	})
	btnSaveParser.Importance = widget.HighImportance

	// Nút kiểm thử trực tiếp
	btnTestRun := widget.NewButtonWithIcon("🧪 Chạy Thử Bóc Tách", theme.MediaPlayIcon(), func() {
		rawText := testInputEntry.Text
		if strings.TrimSpace(rawText) == "" {
			dialog.ShowInformation("Thông báo", "Vui lòng dán văn bản thư hoặc chọn một thư từ danh sách để thử nghiệm!", w)
			return
		}

		dummyEmail := EmailItem{
			Body:    rawText,
			Snippet: rawText,
			Subject: "",
			From:    "",
		}

		// Đọc thử nếu có header giả
		lines := strings.Split(rawText, "\n")
		for _, l := range lines {
			if strings.HasPrefix(strings.ToLower(l), "subject:") {
				dummyEmail.Subject = strings.TrimSpace(l[8:])
			}
			if strings.HasPrefix(strings.ToLower(l), "from:") {
				dummyEmail.From = strings.TrimSpace(l[5:])
			}
		}

		// Áp dụng rules hiện tại trên form
		currentRules := readRulesFromUI()
		tempParser := MailParser{
			Name:       entryParserName.Text,
			FilterFrom: entryParserFilterFrom.Text,
			FilterSubj: entryParserFilterSubj.Text,
			Enabled:    true,
			Rules:      currentRules,
		}

		tStart := time.Now()
		extracted := tempParser.Execute(dummyEmail)
		dur := time.Since(tStart)

		if len(extracted) == 0 {
			testOutputEntry.SetText(fmt.Sprintf("// ⚠ Không tìm thấy trường dữ liệu nào phù hợp (%v).\n// Hãy kiểm tra lại Biểu thức Regex hoặc từ khóa!", dur))
		} else {
			jsonBytes, _ := json.MarshalIndent(extracted, "", "  ")
			testOutputEntry.SetText(fmt.Sprintf("// ✔ Bóc tách thành công %d trường (%v):\n%s", len(extracted), dur, string(jsonBytes)))
		}
	})
	btnTestRun.Importance = widget.HighImportance

	// Live Tester Container
	testerHeader := container.NewHBox(
		widget.NewLabelWithStyle("🧪 TRÌNH THỬ NGHIỆM BÓC TÁCH (LIVE PLAYGROUND)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		widget.NewButtonWithIcon("Làm mới danh sách thư", theme.ViewRefreshIcon(), func() {
			refreshTestEmailOptions()
		}),
	)

	testerGrid := container.NewGridWithColumns(2,
		container.NewBorder(
			container.NewVBox(
				testEmailSelect,
				widget.NewLabel("Văn bản email kiểm thử:"),
			),
			btnTestRun,
			nil, nil,
			testInputEntry,
		),
		container.NewBorder(
			widget.NewLabel("Kết quả JSON bóc tách được:"),
			nil, nil, nil,
			testOutputEntry,
		),
	)

	testerBox := container.NewBorder(
		testerHeader,
		nil, nil, nil,
		testerGrid,
	)

	// Editor Form Container
	editorForm := container.NewVBox(
		container.NewHBox(
			widget.NewLabelWithStyle("🛠️ CẤU HÌNH BỘ BÓC TÁCH (PARSER EDITOR)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			checkParserEnabled,
			btnSaveParser,
		),
		container.NewGridWithColumns(2,
			container.NewVBox(
				widget.NewLabelWithStyle("Tên Parser:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				entryParserName,
				widget.NewLabelWithStyle("Lọc Người Gửi (From):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				entryParserFilterFrom,
			),
			container.NewVBox(
				widget.NewLabelWithStyle("Mô tả:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				entryParserDesc,
				widget.NewLabelWithStyle("Lọc Tiêu Đề (Subject):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				entryParserFilterSubj,
			),
		),
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabelWithStyle("🎯 DANH SÁCH CÁC TRƯỜNG DỮ LIỆU CẦN TRÍCH XUẤT", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			btnAddRule,
		),
		parserRulesContainer,
	)

	editorScroll := container.NewVScroll(container.NewPadded(editorForm))

	rightSplit := container.NewVSplit(editorScroll, testerBox)
	rightSplit.SetOffset(0.55)

	mainSplit := container.NewHSplit(leftPanel, rightSplit)
	mainSplit.SetOffset(0.28)

	// Chọn parser đầu tiên nếu có
	parsersMu.Lock()
	hasParsers := len(parsers) > 0
	parsersMu.Unlock()
	if hasParsers {
		parserListWidget.Select(0)
	}

	return mainSplit
}

// ==================== CÁC HÀM TIỆN ÍCH FORM ====================

func refreshTestEmailOptions() {
	mu.Lock()
	var opts []string
	opts = append(opts, "-- Chọn thư vừa đọc từ Hộp Thư --")
	for _, m := range emails {
		label := fmt.Sprintf("[%s] %s", m.Date, m.Subject)
		opts = append(opts, label)
	}
	mu.Unlock()

	if testEmailSelect != nil {
		testEmailSelect.Options = opts
		testEmailSelect.SetSelectedIndex(0)
		testEmailSelect.Refresh()
	}
}

func clearParserEditor() {
	entryParserName.SetText("")
	entryParserDesc.SetText("")
	entryParserFilterFrom.SetText("")
	entryParserFilterSubj.SetText("")
	checkParserEnabled.SetChecked(true)
	parserRulesContainer.Objects = nil
	parserRulesContainer.Refresh()
}

func loadParserToEditor(p MailParser) {
	entryParserName.SetText(p.Name)
	entryParserDesc.SetText(p.Description)
	entryParserFilterFrom.SetText(p.FilterFrom)
	entryParserFilterSubj.SetText(p.FilterSubj)
	checkParserEnabled.SetChecked(p.Enabled)

	parserRulesContainer.Objects = nil
	for _, r := range p.Rules {
		addRuleRow(r)
	}
	parserRulesContainer.Refresh()
}

func addRuleRow(r ParserRule) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(r.FieldName)
	nameEntry.SetPlaceHolder("Tên trường (vd: otp, link, amount)")

	sourceSelect := widget.NewSelect([]string{"body (Nội dung)", "subject (Tiêu đề)", "from (Người gửi)"}, nil)
	switch strings.ToLower(r.Source) {
	case "subject":
		sourceSelect.SetSelected("subject (Tiêu đề)")
	case "from":
		sourceSelect.SetSelected("from (Người gửi)")
	default:
		sourceSelect.SetSelected("body (Nội dung)")
	}

	typeSelect := widget.NewSelect([]string{"regex (Biểu thức chính quy)", "between (Giữa 2 từ khóa)"}, nil)
	if strings.ToLower(r.Type) == "between" {
		typeSelect.SetSelected("between (Giữa 2 từ khóa)")
	} else {
		typeSelect.SetSelected("regex (Biểu thức chính quy)")
	}

	patternEntry := widget.NewEntry()
	patternEntry.SetText(r.Pattern)
	patternEntry.SetPlaceHolder(`Ví dụ: (?i)mã:?\s*(\d{4,8}) hoặc TừKhóaĐầu||TừKhóaCuối`)

	groupEntry := widget.NewEntry()
	if r.Group <= 0 {
		groupEntry.SetText("1")
	} else {
		groupEntry.SetText(fmt.Sprintf("%d", r.Group))
	}
	groupEntry.SetPlaceHolder("Group (1)")

	var rowContainer *fyne.Container

	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		var newObjs []fyne.CanvasObject
		for _, o := range parserRulesContainer.Objects {
			if o != rowContainer {
				newObjs = append(newObjs, o)
			}
		}
		parserRulesContainer.Objects = newObjs
		parserRulesContainer.Refresh()
	})
	delBtn.Importance = widget.LowImportance

	rowContainer = container.NewVBox(
		container.NewBorder(
			nil, nil, nil, delBtn,
			container.NewGridWithColumns(3,
				container.NewVBox(widget.NewLabel("Tên Trường:"), nameEntry),
				container.NewVBox(widget.NewLabel("Nguồn Bóc Tách:"), sourceSelect),
				container.NewVBox(widget.NewLabel("Phương Thức:"), typeSelect),
			),
		),
		container.NewGridWithColumns(4,
			container.NewVBox(widget.NewLabel("Biểu thức / Từ khóa:"), patternEntry),
			container.NewVBox(widget.NewLabel("Group:"), groupEntry),
			layout.NewSpacer(),
			layout.NewSpacer(),
		),
		widget.NewSeparator(),
	)

	parserRulesContainer.Add(rowContainer)
	parserRulesContainer.Refresh()
}

func readRulesFromUI() []ParserRule {
	var rules []ParserRule
	for _, obj := range parserRulesContainer.Objects {
		c, ok := obj.(*fyne.Container)
		if !ok || len(c.Objects) < 2 {
			continue
		}

		// Hàng 1
		row1Border, ok := c.Objects[0].(*fyne.Container)
		if !ok {
			continue
		}
		row1Grid, ok := row1Border.Objects[0].(*fyne.Container)
		if !ok || len(row1Grid.Objects) < 3 {
			continue
		}

		nameVBox := row1Grid.Objects[0].(*fyne.Container)
		sourceVBox := row1Grid.Objects[1].(*fyne.Container)
		typeVBox := row1Grid.Objects[2].(*fyne.Container)

		nameEntry := nameVBox.Objects[1].(*widget.Entry)
		sourceSelect := sourceVBox.Objects[1].(*widget.Select)
		typeSelect := typeVBox.Objects[1].(*widget.Select)

		// Hàng 2
		row2Grid, ok := c.Objects[1].(*fyne.Container)
		if !ok || len(row2Grid.Objects) < 2 {
			continue
		}
		patternVBox := row2Grid.Objects[0].(*fyne.Container)
		groupVBox := row2Grid.Objects[1].(*fyne.Container)

		patternEntry := patternVBox.Objects[1].(*widget.Entry)
		groupEntry := groupVBox.Objects[1].(*widget.Entry)

		fName := strings.TrimSpace(nameEntry.Text)
		pattern := strings.TrimSpace(patternEntry.Text)
		if fName == "" || pattern == "" {
			continue
		}

		source := "body"
		if strings.HasPrefix(sourceSelect.Selected, "subject") {
			source = "subject"
		} else if strings.HasPrefix(sourceSelect.Selected, "from") {
			source = "from"
		}

		pType := "regex"
		if strings.HasPrefix(typeSelect.Selected, "between") {
			pType = "between"
		}

		group := 1
		fmt.Sscanf(groupEntry.Text, "%d", &group)

		rules = append(rules, ParserRule{
			FieldName: fName,
			Source:    source,
			Type:      pType,
			Pattern:   pattern,
			Group:     group,
		})
	}
	return rules
}

func showPresetsDialog(w fyne.Window) {
	presetMD := `### ✨ Thư Viện Mẫu Parser Có Sẵn (Presets)
Chọn một trong các mẫu bên dưới để thêm ngay vào danh sách:

1. **🔑 Mã OTP 4-6 Số**: Tìm kiếm mã xác thực nhanh chóng.
2. **🔗 Link Xác Nhận**: Bóc tách link kích hoạt tài khoản.
3. **📦 Mã Đơn Hàng & Số Tiền**: Bóc tách thông tin mua hàng.
`
	rt := widget.NewRichTextFromMarkdown(presetMD)

	btn1 := widget.NewButton("Thêm Mẫu OTP (4-6 số)", func() {
		p := MailParser{
			ID:          fmt.Sprintf("p-otp-%d", time.Now().UnixNano()),
			Name:        "🔑 Bóc Tách Mã OTP",
			Description: "Bóc tách mã xác nhận số",
			Enabled:     true,
			Rules: []ParserRule{
				{FieldName: "otp", Source: "body", Type: "regex", Pattern: `(?i)(?:mã|code|otp)[^\d]{0,20}(\b\d{4,8}\b)`, Group: 1},
			},
		}
		addPresetParser(p)
		dialog.ShowInformation("Thành công", "Đã thêm mẫu OTP thành công!", w)
	})

	btn2 := widget.NewButton("Thêm Mẫu Link Kích Hoạt", func() {
		p := MailParser{
			ID:          fmt.Sprintf("p-link-%d", time.Now().UnixNano()),
			Name:        "🔗 Trích Xuất Link Kích Hoạt",
			Description: "Lấy URL kích hoạt hoặc xác nhận",
			Enabled:     true,
			Rules: []ParserRule{
				{FieldName: "url", Source: "body", Type: "regex", Pattern: `https?://[^\s"'<>]+(?:verify|confirm|activate)[^\s"'<>]*`, Group: 0},
			},
		}
		addPresetParser(p)
		dialog.ShowInformation("Thành công", "Đã thêm mẫu Link thành công!", w)
	})

	btn3 := widget.NewButton("Thêm Mẫu Đơn Hàng & Tiền", func() {
		p := MailParser{
			ID:          fmt.Sprintf("p-order-%d", time.Now().UnixNano()),
			Name:        "📦 Bóc Tách Đơn Hàng & Tiền",
			Description: "Trích xuất mã hóa đơn và giá tiền",
			Enabled:     true,
			Rules: []ParserRule{
				{FieldName: "order_id", Source: "body", Type: "regex", Pattern: `(?i)(?:đơn|order)[\s:#]*([A-Za-z0-9_-]{5,20})`, Group: 1},
				{FieldName: "amount", Source: "body", Type: "regex", Pattern: `(?i)(\d{1,3}(?:[.,]\d{3})*(?:\s*(?:đ|vnd|vnđ|\$)))`, Group: 1},
			},
		}
		addPresetParser(p)
		dialog.ShowInformation("Thành công", "Đã thêm mẫu Đơn hàng thành công!", w)
	})

	content := container.NewVBox(
		rt,
		widget.NewSeparator(),
		btn1,
		btn2,
		btn3,
	)

	d := dialog.NewCustom("Thư Viện Mẫu Mail Parser", "Đóng", content, w)
	d.Resize(fyne.NewSize(450, 360))
	d.Show()
}

func addPresetParser(p MailParser) {
	parsersMu.Lock()
	parsers = append(parsers, p)
	saveParsersLocked()
	newIdx := len(parsers) - 1
	parsersMu.Unlock()

	parserListWidget.Refresh()
	parserListWidget.Select(newIdx)
}
