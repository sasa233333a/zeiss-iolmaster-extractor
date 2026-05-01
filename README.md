# ZEISS IOLMaster Extractor

Windows command-line tools for sorting and extracting structured CSV data from ZEISS IOLMaster PDF reports.

The project currently supports:

- IOLMaster 700 reports
- Older IOLMaster Advanced Technology V7.7 reports
- A combined sorter/extractor that separates reports into `a超报告` and `非a超报告`
- CSV output with a shared table schema for 700 and older reports

No sample medical reports, generated CSV files, or binaries are included in this repository.

## Build

Install Go, then run one of:

```powershell
go build -o "ZEISS数据提取工具.exe" .
go build -tags non700 -o "ZEISS非700数据提取工具.exe" .
go build -tags combined -o "ZEISS一键分选提取工具.exe" .
```

## Usage

### IOLMaster 700 only

Place `ZEISS数据提取工具.exe` in a folder containing IOLMaster 700 PDF reports and double-click it.

Output:

- `ZEISS_Final_Extract_v16.csv`

### Older non-700 IOLMaster reports

Place `ZEISS非700数据提取工具.exe` in a folder containing older IOLMaster V7.7 PDF reports and double-click it.

Output:

- `ZEISS_Non700_Extract.csv`

### Combined sorter and extractor

Place `ZEISS一键分选提取工具.exe` in a root folder containing PDF reports and double-click it.

Output:

- `ZEISS_分选提取结果/a超报告/`
- `ZEISS_分选提取结果/非a超报告/`
- `ZEISS_分选提取结果/a超报告/ZEISS_A超_Extract.csv`
- `ZEISS_分选提取结果/一键分选提取日志.txt`

The combined tool classifies both IOLMaster 700 and older IOLMaster V7.7 reports as `a超报告`.

## Performance

The combined and non-700 tools use a worker pool. By default, they use twice the detected CPU thread count, capped by the number of files.

You can override the worker count:

```powershell
$env:ZEISS_WORKERS="64"
.\ZEISS一键分选提取工具.exe
```

## Privacy

This project is designed for local processing of medical PDFs. Do not commit patient reports, extracted CSV files, generated executables, or archives to the repository.

## License

MIT
