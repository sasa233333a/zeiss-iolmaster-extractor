//go:build non700 || combined

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func collectPDFFiles(dir string) ([]string, error) {
	var pdfFiles []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		if strings.HasSuffix(strings.ToLower(fileName), ".pdf") {
			pdfFiles = append(pdfFiles, fileName)
		}
	}
	sort.Strings(pdfFiles)
	return pdfFiles, nil
}

func chooseWorkerCount(fileCount int) int {
	workers := runtime.NumCPU() * 2
	if override := strings.TrimSpace(os.Getenv("ZEISS_WORKERS")); override != "" {
		if n, err := strconv.Atoi(override); err == nil && n > 0 {
			workers = n
		}
	}
	if workers < 1 {
		workers = 1
	}
	if workers > fileCount {
		workers = fileCount
	}
	return workers
}

func processPDFs(files []string, workers int, writer *csv.Writer) (success int, failed int) {
	jobs := make(chan pdfJob, workers*2)
	resultCh := make(chan pdfResult, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				resultCh <- processOnePDF(job)
			}
		}()
	}

	go func() {
		for i, fileName := range files {
			jobs <- pdfJob{Index: i, FileName: fileName}
		}
		close(jobs)
		wg.Wait()
		close(resultCh)
	}()

	done := 0
	start := time.Now()
	lastProgress := start
	var failedFiles []string

	for result := range resultCh {
		done++
		if result.Err != nil {
			failed++
			if len(failedFiles) < 20 {
				failedFiles = append(failedFiles, fmt.Sprintf("%s: %v", result.FileName, result.Err))
			}
			printProgress(done, len(files), success, failed, start, &lastProgress, false)
			continue
		}
		writeOK := true
		for _, row := range result.Rows {
			if err := writer.Write(row); err != nil {
				writeOK = false
				failed++
				if len(failedFiles) < 20 {
					failedFiles = append(failedFiles, fmt.Sprintf("%s: 写入CSV失败: %v", result.FileName, err))
				}
				break
			}
		}
		if writeOK {
			success++
		}
		if done%progressBatch == 0 {
			writer.Flush()
		}
		printProgress(done, len(files), success, failed, start, &lastProgress, false)
	}

	printProgress(done, len(files), success, failed, start, &lastProgress, true)
	if len(failedFiles) > 0 {
		fmt.Println("失败文件示例:")
		for _, item := range failedFiles {
			fmt.Println(" -", item)
		}
		if failed > len(failedFiles) {
			fmt.Printf(" - 其余 %d 个失败文件未展开显示\n", failed-len(failedFiles))
		}
	}
	return success, failed
}

func printProgress(done, total, success, failed int, start time.Time, lastProgress *time.Time, force bool) {
	now := time.Now()
	if !force && (done == total || now.Sub(*lastProgress) < progressInterval) {
		return
	}
	*lastProgress = now

	elapsed := now.Sub(start).Seconds()
	rate := 0.0
	if elapsed > 0 {
		rate = float64(done) / elapsed
	}
	remaining := total - done
	eta := "--"
	if rate > 0 && remaining > 0 {
		eta = (time.Duration(float64(remaining)/rate) * time.Second).Round(time.Second).String()
	}
	fmt.Printf("进度 %d/%d，成功 %d，失败 %d，速度 %.1f 文件/秒，预计剩余 %s\n", done, total, success, failed, rate, eta)
}

func processOnePDF(job pdfJob) pdfResult {
	start := time.Now()
	content, err := readPdfContent(job.FileName)
	if err != nil {
		return pdfResult{Index: job.Index, FileName: job.FileName, Err: err, Elapsed: time.Since(start)}
	}
	if !isOldIOLMasterReport(content) {
		return pdfResult{Index: job.Index, FileName: job.FileName, Err: fmt.Errorf("不是非700旧版IOLMaster报告"), Elapsed: time.Since(start)}
	}

	od, osData := parseReport(content, job.FileName)
	od.FileName = job.FileName
	osData.FileName = job.FileName

	return pdfResult{
		Index:    job.Index,
		FileName: job.FileName,
		Rows:     [][]string{structToRow(od), structToRow(osData)},
		Elapsed:  time.Since(start),
	}
}
