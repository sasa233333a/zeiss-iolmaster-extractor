//go:build non700 || combined

package main

import (
	"regexp"
	"strings"
	"time"
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

type pdfJob struct {
	Index    int
	FileName string
}

type pdfResult struct {
	Index    int
	FileName string
	Rows     [][]string
	Err      error
	Elapsed  time.Duration
}

type reportKind int

const (
	reportUnsupported reportKind = iota
	reportIOLMaster700
	reportOldIOLMaster
)

type combinedJob struct {
	Index      int
	SourcePath string
	Display    string
}

type combinedResult struct {
	Index      int
	SourcePath string
	Display    string
	Kind       reportKind
	Rows       [][]string
	Err        error
	Elapsed    time.Duration
}

const (
	outputCSV        = "ZEISS_Non700_Extract.csv"
	defaultNon700Dir = "."
	combinedOutput   = "ZEISS_分选提取结果"
	progressInterval = 2 * time.Second
	progressBatch    = 500
	stopPattern      = `(?i)(?:AL:|ACD:|LT:|WTW:|SE:|K1:|K2:|ΔK:|Delta|DK:|ΔΚ:|TSE:|TK1:|TK2:|ΔTK:|LS:|VS:|Ref:|VA:|LVC:|LVCMode:|TargetRef:|SIA:|Biometric|Warning)`
)

var (
	textCleaner = strings.NewReplacer(
		"\n", " ",
		"$", " ",
		"{", " ",
		"}", " ",
		"^", " ",
		"circ", " ",
		"\\", " ",
		"~", " ",
		"Target ref.:", "TargetRef:",
		"Target ref:", "TargetRef:",
		"Ref.", "Ref:",
		"LVC mode:", "LVCMode:",
	)
	valueCleaner = strings.NewReplacer(
		"mm", "",
		"µm", "",
		"um", "",
		"D", "",
		"°", "",
		"deg", "",
		" ", "",
	)

	reStop            = regexp.MustCompile(stopPattern)
	reSD              = regexp.MustCompile(`(?i)SD:?\s*([\d\.]+)`)
	reDigit           = regexp.MustCompile(`[\d\.]+`)
	reChinese         = regexp.MustCompile(`[\p{Han}]+[,\s]*[\p{Han}]+`)
	reDates           = regexp.MustCompile(`((?:19|20)\d{2}[-/.年]\d{1,2}[-/.月]\d{1,2}[日]?|\d{1,2}[-/.]\d{1,2}[-/.](?:19|20)\d{2})`)
	reGender          = regexp.MustCompile(`(?i)\b(Male|Female)\b`)
	rePatientID       = regexp.MustCompile(`Patient ID\s*(\d+)`)
	reFallbackID      = regexp.MustCompile(`\b\d{9,12}\b`)
	reSurg            = regexp.MustCompile(`(?:Surgeon|Operator)\s+([A-Za-z\p{Han}]+)`)
	reFileNameID      = regexp.MustCompile(`^([\p{Han}]+)([^_]+)`)
	reDeltaK          = regexp.MustCompile(`(?i)(?:ΔK:|Delta\s*K:|DK:|ΔΚ:)`)
	reDeltaTK         = regexp.MustCompile(`(?i)(?:ΔTK:|Delta\s*TK:|DTK:|ΔΤΚ:)`)
	reOldIOLCalcStart = regexp.MustCompile(`AL:\s*K1:\s*K2:\s*R\s*/\s*SE:\s*Cyl\.:`)
	reOldALAvg        = regexp.MustCompile(`AL:\s*([0-9.]+)\s*mm\s*\(SNR\s*=\s*([0-9.]+)\)`)
	reOldKSummary     = regexp.MustCompile(`([0-9.]+)\s*/\s*([0-9.]+)\s*D\s*SD:\s*([0-9.]+)\s*mm`)
	reOldKTriple      = regexp.MustCompile(`(?s)K1:\s*([0-9.]+)\s*D\s*X\s*([0-9]+)°.*?K2:\s*([0-9.]+)\s*D\s*X\s*([0-9]+)°.*?(?:∆|Δ)\s*K:\s*([-0-9.]+)\s*D\s*X\s*([0-9]+)°`)
	reNumber          = regexp.MustCompile(`[-+]?[0-9]+(?:\.[0-9]+)?`)
	reOldKLineAt      = regexp.MustCompile(`([0-9.]+)\s*D\s*/\s*[0-9.]+\s*mm\s*@\s*([0-9]+)°`)
	reOldRSE          = regexp.MustCompile(`([0-9.]+)\s*mm\s*/\s*([-+]?[0-9.]+)\s*D`)
	reOldCyl          = regexp.MustCompile(`([-+]?[0-9.]+)\s*D\s*@\s*([0-9]+)°`)

	labelRegexes = map[string]*regexp.Regexp{
		`LS:`:        regexp.MustCompile(`(?i)\bLS:`),
		`VS:`:        regexp.MustCompile(`(?i)\bVS:`),
		`Ref:`:       regexp.MustCompile(`(?i)\bRef:`),
		`VA:`:        regexp.MustCompile(`(?i)\bVA:`),
		`LVC:`:       regexp.MustCompile(`(?i)\bLVC:`),
		`LVCMode:`:   regexp.MustCompile(`(?i)\bLVCMode:`),
		`TargetRef:`: regexp.MustCompile(`(?i)\bTargetRef:`),
		`SIA:`:       regexp.MustCompile(`(?i)\bSIA:`),
		`AL:`:        regexp.MustCompile(`(?i)\bAL:`),
		`ACD:`:       regexp.MustCompile(`(?i)\bACD:`),
		`LT:`:        regexp.MustCompile(`(?i)\bLT:`),
		`WTW:`:       regexp.MustCompile(`(?i)\bWTW:`),
		`SE:`:        regexp.MustCompile(`(?i)\bSE:`),
		`TSE:`:       regexp.MustCompile(`(?i)\bTSE:`),
		`K1:`:        regexp.MustCompile(`(?i)\bK1:`),
		`K2:`:        regexp.MustCompile(`(?i)\bK2:`),
		`TK1:`:       regexp.MustCompile(`(?i)\bTK1:`),
		`TK2:`:       regexp.MustCompile(`(?i)\bTK2:`),
		`ΔK:`:        regexp.MustCompile(`(?i)ΔK:`),
		`ΔTK:`:       regexp.MustCompile(`(?i)ΔTK:`),
	}
)
