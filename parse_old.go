//go:build non700 || combined

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseOldIOLMaster(text string, od *ReportData, os *ReportData) {
	parseOldIOLCalculationReport(text, od, os)
	parseOldMeasurementReport(text, od, os)
}

func applyOldCalculatedFallbacks(d *ReportData) {
	if isMissingValue(d.SE) {
		d.SE = calcMean(d.K1, d.K2)
	}
	if isMissingValue(d.DeltaK) {
		d.DeltaK = calcDelta(d.K1, d.K2)
	}
	if isMissingValue(d.DK_Axis) && !isMissingValue(d.DeltaK) {
		d.DK_Axis = d.K1_Axis
	}
}

func isMissingValue(val string) bool {
	val = strings.TrimSpace(val)
	return val == "" || val == "--" || val == "---"
}

func parseOldIOLCalculationReport(text string, od *ReportData, os *ReportData) {
	blocks := splitOldIOLCalcBlocks(text)
	if len(blocks) == 0 {
		return
	}

	targets := []*ReportData{od, os}
	for i, block := range blocks {
		if i >= len(targets) {
			break
		}
		fillOldIOLCalcBlock(block, targets[i])
	}
}

func splitOldIOLCalcBlocks(text string) []string {
	matches := reOldIOLCalcStart.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	var blocks []string
	for i, loc := range matches {
		start := loc[1]
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		} else if idx := strings.Index(text[start:], "Carl Zeiss"); idx >= 0 {
			end = start + idx
		}

		block := text[start:end]
		if idx := firstNonNegativeIndex(block, []string{"AMO ", "Alcon "}); idx >= 0 {
			block = block[:idx]
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func firstNonNegativeIndex(text string, needles []string) int {
	best := -1
	for _, needle := range needles {
		idx := strings.Index(text, needle)
		if idx >= 0 && (best == -1 || idx < best) {
			best = idx
		}
	}
	return best
}

func fillOldIOLCalcBlock(block string, d *ReportData) {
	numbers := reNumber.FindAllString(block, -1)
	if len(numbers) < 10 {
		return
	}

	d.AL = numbers[0]

	if m := reOldKLineAt.FindStringSubmatch(block); len(m) > 2 {
		d.K1 = m[1]
		d.K1_Axis = m[2]
	}
	remaining := block
	if first := reOldKLineAt.FindStringIndex(block); first != nil {
		remaining = block[first[1]:]
		if m := reOldKLineAt.FindStringSubmatch(remaining); len(m) > 2 {
			d.K2 = m[1]
			d.K2_Axis = m[2]
		}
	}

	if m := reOldRSE.FindStringSubmatch(remaining); len(m) > 2 {
		d.Ref = m[1]
		d.SE = m[2]
	}
	if m := reOldCyl.FindStringSubmatch(remaining); len(m) > 2 {
		d.DeltaK = m[1]
		d.DK_Axis = m[2]
	}

	// 旧版 IOL 计算报告只有部分报告含 ACD，数值位于 Cyl 后。
	if strings.Contains(block, "ACD:") && len(numbers) > 15 {
		d.ACD = numbers[15]
	}
}

func parseOldMeasurementReport(text string, od *ReportData, os *ReportData) {
	alMatches := reOldALAvg.FindAllStringSubmatch(text, -1)
	if len(alMatches) >= 1 {
		od.AL = alMatches[0][1]
	}
	if len(alMatches) >= 2 {
		os.AL = alMatches[1][1]
	}

	kBlocks := splitOldKMeasurementBlocks(text)
	if len(kBlocks) >= 1 {
		fillOldKMeasurementBlock(kBlocks[0], od)
	}
	if len(kBlocks) >= 2 {
		fillOldKMeasurementBlock(kBlocks[1], os)
	}
}

type oldKMeasurementBlock struct {
	Summary []string
	Text    string
}

func splitOldKMeasurementBlocks(text string) []oldKMeasurementBlock {
	locs := reOldKSummary.FindAllStringSubmatchIndex(text, -1)
	if len(locs) == 0 {
		return nil
	}

	blocks := make([]oldKMeasurementBlock, 0, len(locs))
	for i, loc := range locs {
		summary := []string{
			text[loc[2]:loc[3]],
			text[loc[4]:loc[5]],
			text[loc[6]:loc[7]],
		}
		start := loc[1]
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else if idx := firstNonNegativeIndex(text[start:], []string{"前前", "白白", "Carl Zeiss"}); idx >= 0 {
			end = start + idx
		}
		blocks = append(blocks, oldKMeasurementBlock{Summary: summary, Text: text[start:end]})
	}
	return blocks
}

func fillOldKMeasurementBlock(block oldKMeasurementBlock, d *ReportData) {
	if len(block.Summary) < 2 {
		return
	}

	d.K1 = block.Summary[0]
	d.K2 = block.Summary[1]
	d.DeltaK = calcDelta(d.K1, d.K2)

	triples := reOldKTriple.FindAllStringSubmatch(block.Text, -1)
	if len(triples) == 0 {
		return
	}

	chosen := triples[0]
	for _, triple := range triples {
		if len(triple) >= 7 && sameFloatText(triple[1], d.K1) && sameFloatText(triple[3], d.K2) {
			chosen = triple
			break
		}
	}
	if len(chosen) >= 7 {
		d.K1_Axis = chosen[2]
		d.K2_Axis = chosen[4]
		d.DK_Axis = chosen[6]
	}
}

func sameFloatText(a, b string) bool {
	av, errA := strconv.ParseFloat(a, 64)
	bv, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return fmt.Sprintf("%.2f", av) == fmt.Sprintf("%.2f", bv)
}

func calcDelta(k1 string, k2 string) string {
	v1, err1 := strconv.ParseFloat(k1, 64)
	v2, err2 := strconv.ParseFloat(k2, 64)
	if err1 != nil || err2 != nil {
		return "--"
	}
	return fmt.Sprintf("%.2f", v1-v2)
}

func calcMean(k1 string, k2 string) string {
	v1, err1 := strconv.ParseFloat(k1, 64)
	v2, err2 := strconv.ParseFloat(k2, 64)
	if err1 != nil || err2 != nil {
		return "--"
	}
	return fmt.Sprintf("%.2f", (v1+v2)/2)
}

func parseAnyDate(d string) time.Time {
	d = strings.ReplaceAll(d, "年", "-")
	d = strings.ReplaceAll(d, "月", "-")
	d = strings.ReplaceAll(d, "日", "")
	d = strings.ReplaceAll(strings.ReplaceAll(d, "/", "-"), ".", "-")

	parts := strings.Split(d, "-")
	if len(parts) != 3 {
		return time.Time{}
	}

	var year, month, day string
	if len(parts[0]) == 4 {
		year, month, day = parts[0], parts[1], parts[2]
	} else if len(parts[2]) == 4 {
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
