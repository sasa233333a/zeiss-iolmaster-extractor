//go:build non700 || combined

package main

import (
	"strings"
)

func isOldIOLMasterReport(text string) bool {
	compact := strings.ToLower(compactText(text))
	return strings.Contains(compact, "iolmaster(r)") && strings.Contains(compact, "advancedtechnologyv.7.7")
}

func parseIOLMaster700(text string, od ReportData, os ReportData) (ReportData, ReportData) {
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

	dkRes := extractAllByRegex(text, reDeltaK)
	if len(dkRes) == 0 {
		dkRes = extractAll(text, `ΔK:`)
	}
	od.DeltaK, od.DK_Axis, os.DeltaK, os.DK_Axis = assignAxis(dkRes)

	od.TK1, od.TK1_Axis, os.TK1, os.TK1_Axis = assignAxis(extractAll(text, `TK1:`))
	od.TK2, od.TK2_Axis, os.TK2, os.TK2_Axis = assignAxis(extractAll(text, `TK2:`))

	dtkRes := extractAllByRegex(text, reDeltaTK)
	if len(dtkRes) == 0 {
		dtkRes = extractAll(text, `ΔTK:`)
	}
	od.DTK, od.DTK_Axis, os.DTK, os.DTK_Axis = assignAxis(dtkRes)

	return od, os
}
