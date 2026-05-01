//go:build non700 || combined

package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func appDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func collectPDFRecursive(root string) ([]string, error) {
	var files []string
	skipDirs := map[string]bool{
		strings.ToLower(combinedOutput): true,
		"iol master 700报告":              true,
		"非iol master 700报告":             true,
		"iol master报告":                  true,
		"a超报告":                          true,
		"非a超报告":                         true,
		"__pycache__":                   true,
		"build":                         true,
		"dist":                          true,
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if d.IsDir() {
			if skipDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".pdf") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func processCombinedPDFs(root, dirAScan, dirNonAScan string, files []string, workers int) []combinedResult {
	jobs := make(chan combinedJob, workers*2)
	resultCh := make(chan combinedResult, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				resultCh <- processCombinedPDF(root, dirAScan, dirNonAScan, job)
			}
		}()
	}

	go func() {
		for i, path := range files {
			jobs <- combinedJob{Index: i, SourcePath: path, Display: flattenedPDFName(root, path)}
		}
		close(jobs)
		wg.Wait()
		close(resultCh)
	}()

	results := make([]combinedResult, 0, len(files))
	done := 0
	start := time.Now()
	lastProgress := start
	for result := range resultCh {
		done++
		results = append(results, result)
		printCombinedProgress(done, len(files), results, start, &lastProgress, false)
	}
	printCombinedProgress(done, len(files), results, start, &lastProgress, true)
	return results
}

func processCombinedPDF(root, dirAScan, dirNonAScan string, job combinedJob) combinedResult {
	start := time.Now()
	content, err := readPdfContent(job.SourcePath)
	if err != nil {
		return combinedResult{Index: job.Index, SourcePath: job.SourcePath, Display: job.Display, Err: err, Elapsed: time.Since(start)}
	}

	kind := classifyReportContent(content)
	result := combinedResult{
		Index:      job.Index,
		SourcePath: job.SourcePath,
		Display:    job.Display,
		Kind:       kind,
		Elapsed:    time.Since(start),
	}
	targetDir := dirAScan
	if kind == reportUnsupported {
		targetDir = dirNonAScan
	}
	target, err := reserveCombinedTarget(targetDir, job.Display)
	if err != nil {
		result.Err = err
		return result
	}
	if err := copyFile(job.SourcePath, target); err != nil {
		result.Err = err
		return result
	}
	result.Display = filepath.Base(target)

	if kind == reportUnsupported {
		result.Elapsed = time.Since(start)
		return result
	}

	od, osData := parseReport(content, filepath.Base(target))
	od.FileName = filepath.Base(target)
	osData.FileName = filepath.Base(target)
	result.Rows = [][]string{structToRow(od), structToRow(osData)}
	result.Elapsed = time.Since(start)
	return result
}

func classifyReportContent(content string) reportKind {
	compact := strings.ToLower(compactText(content))
	if strings.Contains(compact, "iolmaster700") {
		return reportIOLMaster700
	}
	if isOldIOLMasterReport(content) {
		return reportOldIOLMaster
	}
	return reportUnsupported
}

func compactText(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r != ' ' && r != '\n' && r != '\r' && r != '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func flattenedPDFName(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return safeFileName(filepath.Base(path))
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) <= 1 {
		return safeFileName(filepath.Base(path))
	}

	prefixStart := len(parts) - 3
	if prefixStart < 0 {
		prefixStart = 0
	}
	nameParts := append([]string{}, parts[prefixStart:len(parts)-1]...)
	nameParts = append(nameParts, filepath.Base(path))
	for i := range nameParts {
		nameParts[i] = safeFileName(nameParts[i])
	}
	return strings.Join(nameParts, "_")
}

func safeFileName(name string) string {
	replacer := strings.NewReplacer(
		`<`, "_", `>`, "_", `:`, "_", `"`, "_", `/`, "_", `\`, "_", `|`, "_", `?`, "_", `*`, "_",
	)
	return strings.TrimSpace(replacer.Replace(name))
}

func reserveCombinedTarget(dir, fileName string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, fileName)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target, nil
	}
	ext := filepath.Ext(fileName)
	stem := strings.TrimSuffix(fileName, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err == nil {
		_ = os.Chtimes(target, info.ModTime(), info.ModTime())
	}
	return nil
}

func writeCombinedCSVs(results []combinedResult, csvPath string) error {
	file, writer, err := createCSV(csvPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, result := range results {
		if result.Err != nil || len(result.Rows) == 0 {
			continue
		}
		for _, row := range result.Rows {
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}

	writer.Flush()
	return writer.Error()
}

func createCSV(path string) (*os.File, *csv.Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, err
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	if _, err := file.WriteString("\xEF\xBB\xBF"); err != nil {
		file.Close()
		return nil, nil, err
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(csvHeaders()); err != nil {
		file.Close()
		return nil, nil, err
	}
	return file, writer, nil
}

func printCombinedProgress(done, total int, results []combinedResult, start time.Time, lastProgress *time.Time, force bool) {
	now := time.Now()
	if !force && (done == total || now.Sub(*lastProgress) < progressInterval) {
		return
	}
	*lastProgress = now
	count700, countOld, unsupported, failed := 0, 0, 0, 0
	for _, result := range results {
		switch {
		case result.Err != nil:
			failed++
		case result.Kind == reportIOLMaster700:
			count700++
		case result.Kind == reportOldIOLMaster:
			countOld++
		default:
			unsupported++
		}
	}
	elapsed := now.Sub(start).Seconds()
	rate := 0.0
	if elapsed > 0 {
		rate = float64(done) / elapsed
	}
	fmt.Printf("进度 %d/%d，a超 %d（700 %d，非700旧版 %d），非a超 %d，失败 %d，速度 %.1f 文件/秒\n", done, total, count700+countOld, count700, countOld, unsupported, failed, rate)
}

func writeCombinedLog(path string, results []combinedResult, count700, countOld, unsupported, failed int) {
	var b strings.Builder
	b.WriteString("ZEISS IOLMaster 一键分选提取日志\n")
	b.WriteString(fmt.Sprintf("a超报告: %d (IOL Master 700: %d, 非700旧版: %d)\n", count700+countOld, count700, countOld))
	b.WriteString(fmt.Sprintf("非a超报告: %d\n", unsupported))
	b.WriteString(fmt.Sprintf("失败: %d\n\n", failed))
	for _, result := range results {
		switch {
		case result.Err != nil:
			b.WriteString(fmt.Sprintf("[失败] %s | %v\n", result.SourcePath, result.Err))
		case result.Kind == reportIOLMaster700:
			b.WriteString(fmt.Sprintf("[a超-700] %s -> %s\n", result.SourcePath, result.Display))
		case result.Kind == reportOldIOLMaster:
			b.WriteString(fmt.Sprintf("[a超-非700旧版] %s -> %s\n", result.SourcePath, result.Display))
		default:
			b.WriteString(fmt.Sprintf("[非a超] %s -> %s\n", result.SourcePath, result.Display))
		}
	}
	_ = os.WriteFile(path, []byte(b.String()), 0644)
}
