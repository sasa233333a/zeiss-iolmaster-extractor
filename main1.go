//go:build !non700 && !combined

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// 定义数据结构
type ReportData struct {
	FileName string
	Name     string
	ID       string
	DOB      string // 统一格式 YYYY/MM/DD
	Gender   string
	Surgeon  string
	ExamDate string // 统一格式 YYYY/MM/DD

	EyeSide     string
	LS, VS      string
	Ref, VA     string
	LVC, Mode   string
	Target, SIA string

	AL, AL_SD   string
	ACD, ACD_SD string
	LT, LT_SD   string
	WTW         string

	SE, SE_SD       string
	K1, K1_Axis     string
	K2, K2_Axis     string
	DeltaK, DK_Axis string

	TSE, TSE_SD   string
	TK1, TK1_Axis string
	TK2, TK2_Axis string
	DTK, DTK_Axis string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("⚠️ 错误恢复:", r)
		}
		fmt.Println("\n------------------------------------------------")
		fmt.Println("程序运行结束。")
		fmt.Println("请按【回车键】退出窗口...")
		fmt.Scanln()
	}()

	outputFile, err := os.Create("ZEISS_Final_Extract_v16.csv")
	if err != nil {
		fmt.Println("❌ 创建文件失败 (请关闭正在打开的 CSV 文件):", err)
		return
	}
	defer outputFile.Close()
	outputFile.WriteString("\xEF\xBB\xBF") // BOM

	writer := csv.NewWriter(outputFile)
	headers := []string{
		"File Name",
		"Patient Name", "Patient ID", "Date of birth", "Gender", "Surgeon", "Measurement Date",
		"Eye",
		"LS", "VS", "Ref", "VA", "LVC", "LVC mode", "Target ref (D)", "SIA (D@°)",
		"AL (mm)", "AL SD (µm)", "ACD (mm)", "ACD SD (µm)", "LT (mm)", "LT SD (µm)", "WTW (mm)",
		"SE (D)", "SE SD (D)",
		"K1 (D)", "K1 Axis (°)", "K2 (D)", "K2 Axis (°)", "ΔK (D)", "ΔK Axis (°)",
		"TSE (D)", "TSE SD (D)", "TK1 (D)", "TK1 Axis (°)", "TK2 (D)", "TK2 Axis (°)", "ΔTK (D)", "ΔTK Axis (°)",
	}
	writer.Write(headers)

	files, _ := os.ReadDir(".")
	count := 0
	fmt.Println("⚡ ZEISS IOLMaster 700 数据提取工具 v16.0 (日期格式统一版)")
	fmt.Println("正在扫描当前文件夹...")

	for _, file := range files {
		fileName := file.Name()
		if strings.HasSuffix(strings.ToLower(fileName), ".pdf") {
			count++
			fmt.Printf("正在提取 [%d]: %s ... ", count, fileName)

			content, err := readPdfContent(fileName)
			if err != nil {
				fmt.Printf("❌ 读取失败\n")
				continue
			}

			od, osData := parseReport(content)
			od.FileName = fileName
			osData.FileName = fileName

			writer.Write(structToRow(od))
			writer.Write(structToRow(osData))
			fmt.Printf("✅ 完成\n")
			writer.Flush()
		}
	}

	if count == 0 {
		fmt.Println("\n⚠️  未找到PDF文件！")
	} else {
		fmt.Printf("\n🎉 全部成功！共处理 %d 个文件。\n结果已保存至: ZEISS_Final_Extract_v16.csv\n", count)
	}
}

func structToRow(d ReportData) []string {
	return []string{
		d.FileName,
		d.Name, d.ID, d.DOB, d.Gender, d.Surgeon, d.ExamDate,
		d.EyeSide,
		d.LS, d.VS, d.Ref, d.VA, d.LVC, d.Mode, d.Target, d.SIA,
		d.AL, d.AL_SD, d.ACD, d.ACD_SD, d.LT, d.LT_SD, d.WTW,
		d.SE, d.SE_SD,
		d.K1, d.K1_Axis, d.K2, d.K2_Axis, d.DeltaK, d.DK_Axis,
		d.TSE, d.TSE_SD, d.TK1, d.TK1_Axis, d.TK2, d.TK2_Axis, d.DTK, d.DTK_Axis,
	}
}

func readPdfContent(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var contentBuilder strings.Builder
	pages := 3
	if r.NumPage() < 3 {
		pages = r.NumPage()
	}
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		text, _ := p.GetPlainText(nil)
		contentBuilder.WriteString(text + " ")
	}
	raw := contentBuilder.String()

	// --- 全局预处理 ---
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, "$", " ")
	raw = strings.ReplaceAll(raw, "{", " ")
	raw = strings.ReplaceAll(raw, "}", " ")
	raw = strings.ReplaceAll(raw, "^", " ")
	raw = strings.ReplaceAll(raw, "circ", " ")
	raw = strings.ReplaceAll(raw, "\\", " ")
	raw = strings.ReplaceAll(raw, "~", " ")

	raw = strings.ReplaceAll(raw, "Target ref.:", "TargetRef:")
	raw = strings.ReplaceAll(raw, "Target ref:", "TargetRef:")
	raw = strings.ReplaceAll(raw, "Ref.", "Ref:")
	raw = strings.ReplaceAll(raw, "LVC mode:", "LVCMode:")

	return raw, nil
}

func cleanValue(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return "--"
	}
	val = strings.ReplaceAll(val, "mm", "")
	val = strings.ReplaceAll(val, "µm", "")
	val = strings.ReplaceAll(val, "um", "")
	val = strings.ReplaceAll(val, "D", "")
	val = strings.ReplaceAll(val, "°", "")
	val = strings.ReplaceAll(val, "deg", "")
	val = strings.ReplaceAll(val, " ", "")
	return val
}

const stopPattern = `(?i)(?:AL:|ACD:|LT:|WTW:|SE:|K1:|K2:|ΔK:|Delta|DK:|ΔΚ:|TSE:|TK1:|TK2:|ΔTK:|LS:|VS:|Ref:|VA:|LVC:|LVCMode:|TargetRef:|SIA:|Biometric|Warning)`

func extractAll(text string, label string) []string {
	var reLabel *regexp.Regexp
	if strings.ContainsAny(label, "Δ") {
		reLabel = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(label))
	} else {
		reLabel = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(label))
	}

	labelIndices := reLabel.FindAllStringIndex(text, -1)
	reStop := regexp.MustCompile(stopPattern)

	var results []string

	for _, loc := range labelIndices {
		startPos := loc[1]
		remainingText := text[startPos:]
		stopLoc := reStop.FindStringIndex(remainingText)

		var val string
		if stopLoc != nil {
			val = remainingText[:stopLoc[0]]
		} else {
			val = remainingText
			if len(val) > 100 {
				val = val[:100]
			}
		}

		results = append(results, strings.TrimSpace(val))
	}
	return results
}

func parseValSD(raw string) (string, string) {
	if raw == "" {
		return "--", "--"
	}
	valPart := raw
	sdPart := "--"

	reSD := regexp.MustCompile(`(?i)SD:?\s*([\d\.]+)`)
	sdMatch := reSD.FindStringSubmatch(raw)

	if len(sdMatch) > 1 {
		sdPart = sdMatch[1]
		idx := strings.Index(raw, sdMatch[0])
		if idx != -1 {
			valPart = raw[:idx]
		}
	}
	return cleanValue(valPart), sdPart
}

func parseValAxis(raw string) (string, string) {
	if raw == "" {
		return "--", "--"
	}
	parts := strings.Split(raw, "@")
	val := "--"
	axis := "--"

	if len(parts) >= 2 {
		val = parts[0]
		reDigit := regexp.MustCompile(`[\d\.]+`)
		axisMatch := reDigit.FindString(parts[1])
		if axisMatch != "" {
			axis = axisMatch
		}
	} else {
		val = raw
	}
	return cleanValue(val), axis
}

func assignText(res []string) (string, string) {
	od, os := "--", "--"
	if len(res) > 0 {
		od = res[0]
	}
	if len(res) > 1 {
		os = res[1]
	}
	return od, os
}
func assignNum(res []string) (string, string, string, string) {
	odVal, odSD, osVal, osSD := "--", "--", "--", "--"
	if len(res) > 0 {
		odVal, odSD = parseValSD(res[0])
	}
	if len(res) > 1 {
		osVal, osSD = parseValSD(res[1])
	}
	return odVal, odSD, osVal, osSD
}
func assignAxis(res []string) (string, string, string, string) {
	odVal, odAx, osVal, osAx := "--", "--", "--", "--"
	if len(res) > 0 {
		odVal, odAx = parseValAxis(res[0])
	}
	if len(res) > 1 {
		osVal, osAx = parseValAxis(res[1])
	}
	return odVal, odAx, osVal, osAx
}

func parseReport(text string) (od ReportData, os ReportData) {
	// 1. 基本信息 (v16.0 日期格式化)
	commonName, commonDOB, commonGender, commonID := "--", "--", "--", "--"
	commonSurg, commonDate := "--", "--"

	reChinese := regexp.MustCompile(`[\p{Han}]+[,\s]*[\p{Han}]+`)
	for _, m := range reChinese.FindAllString(text, -1) {
		clean := strings.TrimSpace(m)
		if !strings.Contains(clean, "院区") && !strings.Contains(clean, "医院") && !strings.Contains(clean, "复旦") {
			commonName = clean
			break
		}
	}

	// 抓取所有可能格式的日期
	reDates := regexp.MustCompile(`((?:19|20)\d{2}[-/.年]\d{1,2}[-/.月]\d{1,2}[日]?|\d{1,2}[-/.]\d{1,2}[-/.](?:19|20)\d{2})`)
	allDateStrings := reDates.FindAllString(text, -1)

	// 使用 time.Time 列表来存储和排序
	var validTimeObjs []time.Time

	// 智能日期解析器
	parseAnyDate := func(d string) time.Time {
		d = strings.ReplaceAll(d, "年", "-")
		d = strings.ReplaceAll(d, "月", "-")
		d = strings.ReplaceAll(d, "日", "")
		d = strings.ReplaceAll(strings.ReplaceAll(d, "/", "-"), ".", "-")

		parts := strings.Split(d, "-")
		if len(parts) != 3 {
			return time.Time{}
		}

		var year, month, day string
		if len(parts[0]) == 4 { // YYYY-MM-DD
			year, month, day = parts[0], parts[1], parts[2]
		} else if len(parts[2]) == 4 { // MM-DD-YYYY
			year, month, day = parts[2], parts[0], parts[1]
		} else {
			return time.Time{}
		}

		if len(month) == 1 {
			month = "0" + month
		}
		if len(day) == 1 {
			day = "0" + day
		}

		t, _ := time.Parse("2006-01-02", year+"-"+month+"-"+day)
		return t
	}

	for _, dStr := range allDateStrings {
		t := parseAnyDate(dStr)
		if !t.IsZero() {
			validTimeObjs = append(validTimeObjs, t)
		}
	}

	// 排序
	sort.Slice(validTimeObjs, func(i, j int) bool {
		return validTimeObjs[i].Before(validTimeObjs[j])
	})

	// 取出并格式化为 YYYY/MM/DD
	if len(validTimeObjs) >= 1 {
		commonDOB = validTimeObjs[0].Format("2006/01/02")
		commonDate = validTimeObjs[len(validTimeObjs)-1].Format("2006/01/02")
	}

	if m := regexp.MustCompile(`(?i)\b(Male|Female)\b`).FindStringSubmatch(text); len(m) > 1 {
		commonGender = m[1]
	}
	if m := regexp.MustCompile(`Patient ID\s*(\d+)`).FindStringSubmatch(text); len(m) > 1 {
		commonID = m[1]
	} else {
		for _, d := range regexp.MustCompile(`\b\d{9,12}\b`).FindAllString(text, -1) {
			if len(d) >= 9 {
				commonID = d
				break
			}
		}
	}

	reSurg := regexp.MustCompile(`(?:Surgeon|Operator)\s+([A-Za-z\p{Han}]+)`)
	for _, m := range reSurg.FindAllStringSubmatch(text, -1) {
		v := strings.ToUpper(strings.TrimSpace(m[1]))
		if len(m[1]) < 50 && v != "PAGE" && v != "ADMINISTRATOR" && v != "SURGEON" && v != "OPERATOR" && v != "FUDAN" && v != "HOSPITAL" {
			commonSurg = strings.TrimSpace(m[1])
			break
		}
	}

	fill := func(d *ReportData, side string) {
		d.Name = commonName
		d.ID = commonID
		d.DOB = commonDOB
		d.Gender = commonGender
		d.Surgeon = commonSurg
		d.ExamDate = commonDate
		d.EyeSide = side
	}
	fill(&od, "OD")
	fill(&os, "OS")

	// 2. 测量数据

	od.LS, os.LS = assignText(extractAll(text, `LS:`))
	od.VS, os.VS = assignText(extractAll(text, `VS:`))
	od.Ref, os.Ref = assignText(extractAll(text, `Ref:`))
	od.VA, os.VA = assignText(extractAll(text, `VA:`))
	od.LVC, os.LVC = assignText(extractAll(text, `LVC:`))
	od.Mode, os.Mode = assignText(extractAll(text, `LVCMode:`))
	od.Target, os.Target = assignText(extractAll(text, `TargetRef:`))

	odSIA, osSIA := assignText(extractAll(text, `SIA:`))
	od.SIA = cleanValue(odSIA)
	os.SIA = cleanValue(osSIA)

	od.AL, od.AL_SD, os.AL, os.AL_SD = assignNum(extractAll(text, `AL:`))
	od.ACD, od.ACD_SD, os.ACD, os.ACD_SD = assignNum(extractAll(text, `ACD:`))
	od.LT, od.LT_SD, os.LT, os.LT_SD = assignNum(extractAll(text, `LT:`))
	od.WTW, _, os.WTW, _ = assignNum(extractAll(text, `WTW:`))
	od.SE, od.SE_SD, os.SE, os.SE_SD = assignNum(extractAll(text, `SE:`))
	od.TSE, od.TSE_SD, os.TSE, os.TSE_SD = assignNum(extractAll(text, `TSE:`))

	od.K1, od.K1_Axis, os.K1, os.K1_Axis = assignAxis(extractAll(text, `K1:`))
	od.K2, od.K2_Axis, os.K2, os.K2_Axis = assignAxis(extractAll(text, `K2:`))

	dkRes := extractAll(text, `(?:ΔK:|Delta\s*K:|DK:|ΔΚ:)`)
	if len(dkRes) == 0 {
		dkRes = extractAll(text, `ΔK:`)
	}
	od.DeltaK, od.DK_Axis, os.DeltaK, os.DK_Axis = assignAxis(dkRes)

	od.TK1, od.TK1_Axis, os.TK1, os.TK1_Axis = assignAxis(extractAll(text, `TK1:`))
	od.TK2, od.TK2_Axis, os.TK2, os.TK2_Axis = assignAxis(extractAll(text, `TK2:`))

	dtkRes := extractAll(text, `(?:ΔTK:|Delta\s*TK:|DTK:|ΔΤΚ:)`)
	if len(dtkRes) == 0 {
		dtkRes = extractAll(text, `ΔTK:`)
	}
	od.DTK, od.DTK_Axis, os.DTK, os.DTK_Axis = assignAxis(dtkRes)

	return od, os
}
