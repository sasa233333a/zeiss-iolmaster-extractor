//go:build non700 || combined

package main

import (
	"github.com/ledongthuc/pdf"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func structToRow(d ReportData) []string {
	return []string{
		dash(d.FileName),
		dash(d.Name), dash(d.ID), dash(d.DOB), dash(d.Gender), dash(d.Surgeon), dash(d.ExamDate),
		dash(d.EyeSide),
		dash(d.LS), dash(d.VS), dash(d.Ref), dash(d.VA), dash(d.LVC), dash(d.Mode), dash(d.Target), dash(d.SIA),
		dash(d.AL), dash(d.AL_SD), dash(d.ACD), dash(d.ACD_SD), dash(d.LT), dash(d.LT_SD), dash(d.WTW),
		dash(d.SE), dash(d.SE_SD),
		dash(d.K1), dash(d.K1_Axis), dash(d.K2), dash(d.K2_Axis), dash(d.DeltaK), dash(d.DK_Axis),
		dash(d.TSE), dash(d.TSE_SD), dash(d.TK1), dash(d.TK1_Axis), dash(d.TK2), dash(d.TK2_Axis), dash(d.DTK), dash(d.DTK_Axis),
	}
}

func dash(val string) string {
	if strings.TrimSpace(val) == "" {
		return "--"
	}
	return val
}

func readPdfContent(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	pages := 3
	if r.NumPage() < pages {
		pages = r.NumPage()
	}

	var contentBuilder strings.Builder
	contentBuilder.Grow(pages * 8192)
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		text, err := p.GetPlainText(nil)
		if err != nil {
			return "", err
		}
		contentBuilder.WriteString(text)
		contentBuilder.WriteByte(' ')
	}

	return textCleaner.Replace(contentBuilder.String()), nil
}

func cleanValue(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return "--"
	}
	return valueCleaner.Replace(val)
}

func extractAll(text string, label string) []string {
	reLabel, ok := labelRegexes[label]
	if !ok {
		if strings.ContainsAny(label, "Δ") {
			reLabel = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(label))
		} else {
			reLabel = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(label))
		}
	}
	return extractAllByRegex(text, reLabel)
}

func extractAllByRegex(text string, reLabel *regexp.Regexp) []string {
	labelIndices := reLabel.FindAllStringIndex(text, -1)
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

func parseReport(text string, fileName string) (od ReportData, os ReportData) {
	commonName, commonDOB, commonGender, commonID, commonSurg, commonDate := extractBaseInfo(text, fileName)
	fillCommon(&od, commonName, commonDOB, commonGender, commonID, commonSurg, commonDate, "OD")
	fillCommon(&os, commonName, commonDOB, commonGender, commonID, commonSurg, commonDate, "OS")

	if isOldIOLMasterReport(text) {
		parseOldIOLMaster(text, &od, &os)
		applyOldCalculatedFallbacks(&od)
		applyOldCalculatedFallbacks(&os)
		return od, os
	}

	return parseIOLMaster700(text, od, os)
}

func extractBaseInfo(text string, fileName string) (name, dob, gender, id, surgeon, examDate string) {
	// 1. 基本信息
	name, id = parseNameIDFromFile(fileName)
	if name == "" {
		name = "--"
	}
	if id == "" {
		id = "--"
	}
	dob, gender, surgeon, examDate = "--", "--", "--", "--"

	if name == "--" {
		for _, m := range reChinese.FindAllString(text, -1) {
			clean := strings.TrimSpace(m)
			if !strings.Contains(clean, "院区") && !strings.Contains(clean, "医院") && !strings.Contains(clean, "复旦") {
				name = clean
				break
			}
		}
	}

	allDateStrings := reDates.FindAllString(text, -1)
	var validTimeObjs []time.Time
	seenDates := make(map[time.Time]bool, len(allDateStrings))
	for _, dStr := range allDateStrings {
		t := parseAnyDate(dStr)
		if !t.IsZero() && !seenDates[t] {
			seenDates[t] = true
			validTimeObjs = append(validTimeObjs, t)
		}
	}

	sort.Slice(validTimeObjs, func(i, j int) bool {
		return validTimeObjs[i].Before(validTimeObjs[j])
	})

	if len(validTimeObjs) >= 1 {
		dob = validTimeObjs[0].Format("2006/01/02")
		examDate = validTimeObjs[len(validTimeObjs)-1].Format("2006/01/02")
	}

	if m := reGender.FindStringSubmatch(text); len(m) > 1 {
		gender = m[1]
	}
	if m := rePatientID.FindStringSubmatch(text); len(m) > 1 {
		id = m[1]
	} else {
		for _, d := range reFallbackID.FindAllString(text, -1) {
			if len(d) >= 9 {
				id = d
				break
			}
		}
	}

	for _, m := range reSurg.FindAllStringSubmatch(text, -1) {
		v := strings.ToUpper(strings.TrimSpace(m[1]))
		if len(m[1]) < 50 && v != "PAGE" && v != "ADMINISTRATOR" && v != "SURGEON" && v != "OPERATOR" && v != "FUDAN" && v != "HOSPITAL" {
			surgeon = strings.TrimSpace(m[1])
			break
		}
	}

	return name, dob, gender, id, surgeon, examDate
}

func parseNameIDFromFile(fileName string) (string, string) {
	base := filepath.Base(fileName)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if m := reFileNameID.FindStringSubmatch(base); len(m) > 2 {
		return m[1], strings.TrimSpace(m[2])
	}
	return "", ""
}

func fillCommon(d *ReportData, name, dob, gender, id, surgeon, examDate, side string) {
	d.Name = name
	d.ID = id
	d.DOB = dob
	d.Gender = gender
	d.Surgeon = surgeon
	d.ExamDate = examDate
	d.EyeSide = side
}
