//go:build non700 || combined

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

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

	if combinedMode {
		runCombinedTool()
		return
	}

	start := time.Now()
	runtime.GOMAXPROCS(runtime.NumCPU())

	pdfFiles, err := collectPDFFiles(defaultNon700Dir)
	if err != nil {
		fmt.Println("❌ 扫描文件失败:", err)
		return
	}

	outputFile, err := os.Create(outputCSV)
	if err != nil {
		fmt.Println("❌ 创建文件失败 (请关闭正在打开的 CSV 文件):", err)
		return
	}
	defer outputFile.Close()
	outputFile.WriteString("\xEF\xBB\xBF") // BOM

	writer := csv.NewWriter(outputFile)
	if err := writer.Write(csvHeaders()); err != nil {
		fmt.Println("❌ 写入表头失败:", err)
		return
	}

	fmt.Println("⚡ ZEISS IOLMaster 非700数据提取工具 v1.0 (旧版双版式)")
	fmt.Println("正在扫描当前文件夹...")

	count := len(pdfFiles)
	if count == 0 {
		writer.Flush()
		fmt.Println("\n⚠️  未找到PDF文件！")
		return
	}

	workers := chooseWorkerCount(count)
	fmt.Printf("共发现 %d 个 PDF，启用 %d 个处理线程，CPU核心数: %d\n", count, workers, runtime.NumCPU())

	success, failed := processPDFs(pdfFiles, workers, writer)
	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Println("❌ 保存CSV失败:", err)
		return
	}

	if failed > 0 {
		fmt.Printf("\n⚠️  已完成，成功 %d 个，失败 %d 个，耗时 %s。\n结果已保存至: %s\n", success, failed, time.Since(start).Round(time.Millisecond), outputCSV)
	} else {
		fmt.Printf("\n🎉 全部完成！成功处理 %d/%d 个文件，耗时 %s。\n结果已保存至: %s\n", success, count, time.Since(start).Round(time.Millisecond), outputCSV)
	}
}

func csvHeaders() []string {
	return []string{
		"File Name",
		"Patient Name", "Patient ID", "Date of birth", "Gender", "Surgeon", "Measurement Date",
		"Eye",
		"LS", "VS", "Ref", "VA", "LVC", "LVC mode", "Target ref (D)", "SIA (D@°)",
		"AL (mm)", "AL SD (µm)", "ACD (mm)", "ACD SD (µm)", "LT (mm)", "LT SD (µm)", "WTW (mm)",
		"SE (D)", "SE SD (D)",
		"K1 (D)", "K1 Axis (°)", "K2 (D)", "K2 Axis (°)", "ΔK (D)", "ΔK Axis (°)",
		"TSE (D)", "TSE SD (D)", "TK1 (D)", "TK1 Axis (°)", "TK2 (D)", "TK2 Axis (°)", "ΔTK (D)", "ΔTK Axis (°)",
	}
}

func runCombinedTool() {
	start := time.Now()
	runtime.GOMAXPROCS(runtime.NumCPU())

	root, err := appDir()
	if err != nil {
		fmt.Println("❌ 获取程序目录失败:", err)
		return
	}
	if err := os.Chdir(root); err != nil {
		fmt.Println("❌ 切换到程序目录失败:", err)
		return
	}

	outputRoot := filepath.Join(root, combinedOutput)
	dirAScan := filepath.Join(outputRoot, "a超报告")
	dirNonAScan := filepath.Join(outputRoot, "非a超报告")
	if err := os.RemoveAll(outputRoot); err != nil {
		fmt.Println("❌ 清理旧输出目录失败:", err)
		return
	}
	if err := os.MkdirAll(dirAScan, 0755); err != nil {
		fmt.Println("❌ 创建a超输出目录失败:", err)
		return
	}
	if err := os.MkdirAll(dirNonAScan, 0755); err != nil {
		fmt.Println("❌ 创建非a超输出目录失败:", err)
		return
	}

	pdfFiles, err := collectPDFRecursive(root)
	if err != nil {
		fmt.Println("❌ 扫描PDF失败:", err)
		return
	}

	fmt.Println("⚡ ZEISS IOLMaster 一键分选提取工具 v1.0")
	fmt.Println("处理目录:", root)
	fmt.Println("输出目录:", outputRoot)
	if len(pdfFiles) == 0 {
		fmt.Println("\n⚠️  未找到PDF文件！")
		return
	}

	workers := chooseWorkerCount(len(pdfFiles))
	fmt.Printf("共发现 %d 个 PDF，启用 %d 个处理线程，CPU核心数: %d\n", len(pdfFiles), workers, runtime.NumCPU())

	results := processCombinedPDFs(root, dirAScan, dirNonAScan, pdfFiles, workers)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Display < results[j].Display
	})

	csvAScan := filepath.Join(dirAScan, "ZEISS_A超_Extract.csv")
	if err := writeCombinedCSVs(results, csvAScan); err != nil {
		fmt.Println("❌ 写入CSV失败:", err)
		return
	}

	var count700, countOld, unsupported, failed int
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

	logPath := filepath.Join(outputRoot, "一键分选提取日志.txt")
	writeCombinedLog(logPath, results, count700, countOld, unsupported, failed)

	fmt.Printf("\n🎉 完成。a超报告 %d 个（700 %d，非700旧版 %d），非a超报告 %d 个，失败 %d 个，耗时 %s。\n", count700+countOld, count700, countOld, unsupported, failed, time.Since(start).Round(time.Millisecond))
	fmt.Println("a超合并表:", csvAScan)
	fmt.Println("日志:", logPath)
}
