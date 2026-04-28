package output

import (
	"fmt"
	"regexp"

	"url-checker/internal/checker"
	"url-checker/internal/util"

	"github.com/xuri/excelize/v2"
)

var illegalChars = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`)

const (
	maxColWidth = 50
)

// WriteResults 将结果写入xlsx文件（成功/失败分sheet）
func WriteResults(results []checker.Result, outputFile string) error {
	f := excelize.NewFile()
	defer f.Close()

	// 拆分为成功和失败列表
	var successResults, errResults []checker.Result
	for _, r := range results {
		if r.Success {
			successResults = append(successResults, r)
		} else {
			errResults = append(errResults, r)
		}
	}

	// 按需创建sheet
	if len(successResults) > 0 {
		f.NewSheet("res")
		setHeader(f, "res", []string{"ID", "URL", "域名/IP", "响应状态码", "Content-Type", "响应体长度", "响应标题", "响应正文前100字符"})
		for i, r := range successResults {
			writeSuccessRow(f, "res", r, i+1)
		}
		setColumnWidth(f, "res", []float64{8, 50, 30, 12, 30, 12, 30, 50})
	}

	if len(errResults) > 0 {
		f.NewSheet("err")
		setHeader(f, "err", []string{"ID", "URL", "错误信息"})
		for i, r := range errResults {
			writeErrorRow(f, "err", r, i+1)
		}
		setColumnWidth(f, "err", []float64{8, 50, 50})
	}

	// 删除默认Sheet1（在创建所需sheet之后删除，确保文件始终有一个sheet）
	if len(successResults) > 0 || len(errResults) > 0 {
		f.DeleteSheet("Sheet1")
	}

	// 设置第一个sheet为活动sheet
	if len(successResults) > 0 {
		idx, _ := f.GetSheetIndex("res")
		f.SetActiveSheet(idx)
	} else if len(errResults) > 0 {
		idx, _ := f.GetSheetIndex("err")
		f.SetActiveSheet(idx)
	}

	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	fmt.Printf("%s结果已保存到: %s (成功: %d, 失败: %d)%s\n", util.ColorGreen, outputFile, len(successResults), len(errResults), util.ColorReset)

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

func writeSuccessRow(f *excelize.File, sheet string, r checker.Result, id int) {
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

func writeErrorRow(f *excelize.File, sheet string, r checker.Result, id int) {
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
func DisplayResult(r checker.Result, completed, total int) {
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
		}
		titleRunes := []rune(title)
		if len(titleRunes) > 30 {
			title = string(titleRunes[:30])
		}

		line := fmt.Sprintf("%s %3d/%-3d  [%s]  %-45s  [code] %d  [len] %d  [title] %s",
			"✓", completed, total, protocol, displayURL, r.StatusCode, r.ContentLen, title)
		fmt.Println(colorize(line, color))
	} else {
		errMsg := r.Error
		if errMsg == "" {
			errMsg = "未知"
		}
		line := fmt.Sprintf("%s %3d/%-3d  [%s]  %-45s  [code] ---  [len] 0  [title] ---  %s",
			"✗", completed, total, protocol, displayURL, errMsg)
		fmt.Println(colorize(line, util.ColorRed))
	}
}

func getStatusColor(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return util.ColorGreen
	case statusCode >= 300 && statusCode < 400:
		return util.ColorCyan
	case statusCode >= 400 && statusCode < 500:
		return util.ColorYellow
	case statusCode >= 500 && statusCode < 600:
		return util.ColorRed
	default:
		return util.ColorReset
	}
}

func colorize(text string, color string) string {
	return fmt.Sprintf("%s%s%s", color, text, util.ColorReset)
}
