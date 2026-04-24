package output

import (
	"fmt"
	"regexp"

	"url-checker/internal/checker"
	"url-checker/internal/util"

	"github.com/xuri/excelize/v2"
)

var illegalChars = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`)

// Result 结果类型别名
type Result = checker.Result

const (
	maxColWidth = 50
)

// WriteResults 将结果写入xlsx文件（成功/失败分sheet）
func WriteResults(results []Result, outputFile string) error {
	f := excelize.NewFile()
	defer f.Close()

	// 删除默认的Sheet1
	f.DeleteSheet("Sheet1")

	// 创建成功结果sheet
	sheetRes := "res"
	f.NewSheet(sheetRes)

	// 创建失败结果sheet
	sheetErr := "err"
	f.NewSheet(sheetErr)

	// 设置表头
	setHeader(f, sheetRes, []string{"ID", "URL", "域名/IP", "响应状态码", "Content-Type", "响应体长度", "响应标题", "响应正文前100字符"})
	setHeader(f, sheetErr, []string{"ID", "URL", "错误信息"})

	// 写入数据
	successCount := 0
	errCount := 0

	successID := 1
	errID := 1

	for _, r := range results {
		if r.Success {
			writeSuccessRow(f, sheetRes, r, successID)
			successID++
			successCount++
		} else {
			writeErrorRow(f, sheetErr, r, errID)
			errID++
			errCount++
		}
	}

	// 设置列宽
	setColumnWidth(f, sheetRes, []float64{8, 50, 30, 12, 30, 12, 30, 50})
	setColumnWidth(f, sheetErr, []float64{8, 50, 50})

	// 保存文件
	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	fmt.Printf("%s结果已保存到: %s (成功: %d, 失败: %d)%s\n", util.ColorGreen, outputFile, successCount, errCount, util.ColorReset)

	return nil
}

func setHeader(f *excelize.File, sheet string, headers []string) {
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, header)
		f.SetCellStyle(sheet, cell, cell, style)
	}
}

func writeSuccessRow(f *excelize.File, sheet string, r Result, id int) {
	row := []interface{}{
		id,
		cleanIllegalChars(r.URL),
		cleanIllegalChars(r.Host),
		r.StatusCode,
		r.ContentType,
		r.ContentLen,
		cleanIllegalChars(r.Title),
		cleanIllegalChars(r.Content),
	}
	for col, value := range row {
		cell, _ := excelize.CoordinatesToCellName(col+1, id+1)
		f.SetCellValue(sheet, cell, value)
	}
}

func writeErrorRow(f *excelize.File, sheet string, r Result, id int) {
	row := []interface{}{
		id,
		cleanIllegalChars(r.URL),
		cleanIllegalChars(r.Error),
	}
	for col, value := range row {
		cell, _ := excelize.CoordinatesToCellName(col+1, id+1)
		f.SetCellValue(sheet, cell, value)
	}
}

func setColumnWidth(f *excelize.File, sheet string, widths []float64) {
	for col, width := range widths {
		colName, _ := excelize.ColumnNumberToName(col + 1)
		f.SetColWidth(sheet, colName, colName, width)
	}
}

func cleanIllegalChars(s string) string {
	return illegalChars.ReplaceAllString(s, "")
}

// DisplayResult 显示结果
func DisplayResult(r Result, completed, total int) {
	protocol := r.Protocol
	if protocol == "" {
		protocol = "URL"
	}

	displayURL := r.URL
	if len(displayURL) > 45 {
		displayURL = displayURL[:42] + "..."
	}

	if r.Success {
		color := getStatusColor(r.StatusCode)
		title := r.Title
		if title == "" {
			title = "N/A"
		} else if len(title) > 30 {
			title = title[:30]
		}

		line := fmt.Sprintf("✓ %3d/%d  [%s]  %-45s [code] %3d  [len] %8d  [title] %s",
			completed, total, protocol, displayURL, r.StatusCode, r.ContentLen, title)
		fmt.Println(colorize(line, color))
	} else {
		errMsg := r.Error
		if errMsg == "" {
			errMsg = "未知"
		}
		line := fmt.Sprintf("✗ %3d/%d  [%s]  %-45s [code] ---  [len]       0  [title] ---  %s",
			completed, total, protocol, displayURL, errMsg)
		fmt.Println(colorize(line, util.ColorRed))
	}
}

func getStatusColor(statusCode int) string {
	if statusCode >= 200 && statusCode < 300 {
		return util.ColorGreen
	}
	if statusCode >= 300 && statusCode < 400 {
		return util.ColorCyan
	}
	if statusCode >= 400 && statusCode < 500 {
		return util.ColorYellow
	}
	if statusCode >= 500 && statusCode < 600 {
		return util.ColorRed
	}
	return util.ColorReset
}

func colorize(text string, color string) string {
	return fmt.Sprintf("%s%s%s", color, text, util.ColorReset)
}
