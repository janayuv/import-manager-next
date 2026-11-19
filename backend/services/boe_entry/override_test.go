package boe_entry

import (
	"strings"
	"testing"

	"github.com/janayuv/import-manager-next/backend/models"
)

func TestParseOverrideCSVWithPastedUserContent(t *testing.T) {
	// The exact 18 part numbers and rates pasted by the user from Excel (tab-separated, headerless)
	pastedContent := `56132711	Standard	15	10	18
56277372	Standard	7.5	10	18
50490871	Standard	10	10	18
55681771	Standard	7.5	10	18
54018872	Standard	7.5	10	18
54018772	Standard	7.5	10	18
54018572	Standard	7.5	10	18
54970072	Standard	15	10	18
54969972	Standard	15	10	18
54969872	Standard	15	10	18
54970372	Standard	10	10	18
54970272	Standard	10	10	18
54970172	Standard	10	10	18
54015672	Standard	7.5	10	18
55104672	Standard	15	10	18
54970473	Standard	15	10	18
55024272	Standard	15	10	18
54588773	Standard	7.5	10	18
`

	parts := []string{
		"56132711", "56277372", "50490871", "55681771", "54018872",
		"54018772", "54018572", "54970072", "54969972", "54969872",
		"54970372", "54970272", "54970172", "54015672", "55104672",
		"54970473", "55024272", "54588773",
	}

	var currentItems []models.BOECalculationItemInput
	for _, p := range parts {
		currentItems = append(currentItems, models.BOECalculationItemInput{
			PartNumber:        p,
			CalculationMethod: "Standard",
			ActualBCDRate:     7.5,
			ActualSWSRate:     10,
			ActualIGSTRate:    18,
			BOEBCDRate:        7.5,
			BOESWSRate:        10,
			BOEIGSTRate:       18,
		})
	}

	svc := &Service{}
	res, err := svc.ParseOverrideCSV(pastedContent, currentItems)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.AppliedCount != 18 {
		t.Errorf("expected 18 applied overrides, got %d (skipped: %d, warnings: %+v)", res.AppliedCount, res.SkippedCount, res.Warnings)
	}

	if res.SkippedCount != 0 {
		t.Errorf("expected 0 skipped, got %d (warnings: %+v)", res.SkippedCount, res.Warnings)
	}

	// Verify specific rates were updated properly
	for _, it := range res.Items {
		if it.PartNumber == "56132711" && it.BOEBCDRate != 15 {
			t.Errorf("expected 56132711 to have BCD=15, got %f", it.BOEBCDRate)
		}
		if it.PartNumber == "56277372" && it.BOEBCDRate != 7.5 {
			t.Errorf("expected 56277372 to have BCD=7.5, got %f", it.BOEBCDRate)
		}
	}
}

func TestParseOverrideCSVVariousFormats(t *testing.T) {
	currentItems := []models.BOECalculationItemInput{
		{PartNumber: "PN-101", CalculationMethod: "Standard", ActualBCDRate: 10, ActualSWSRate: 10, ActualIGSTRate: 18},
		{PartNumber: "PN-102", CalculationMethod: "Standard", ActualBCDRate: 15, ActualSWSRate: 10, ActualIGSTRate: 18},
	}

	svc := &Service{}

	// 1. With standard CSV header
	csv1 := "partNo,calculationMethod,boeBcdRate,boeSwsRate,boeIgstRate\nPN-101,CEPA,5.0,10,18\nPN-102,Standard,15,10,18"
	res1, err := svc.ParseOverrideCSV(csv1, currentItems)
	if err != nil || res1.AppliedCount != 2 {
		t.Fatalf("test 1 failed: err=%v, res=%+v", err, res1)
	}
	if res1.Items[0].CalculationMethod != "CEPA" || res1.Items[0].BOEBCDRate != 5.0 {
		t.Errorf("test 1 item 0 mismatch: %+v", res1.Items[0])
	}

	// 2. Tab-separated with Excel header variants
	tsv2 := "Part No\tMethod\tBCD %\tSWS %\tIGST %\nPN-101\tStandard\t7.5%\t10%\t18%\n"
	res2, err := svc.ParseOverrideCSV(tsv2, currentItems)
	if err != nil || res2.AppliedCount != 1 {
		t.Fatalf("test 2 failed: err=%v, res=%+v", err, res2)
	}
	if res2.Items[0].BOEBCDRate != 7.5 {
		t.Errorf("test 2 rate mismatch: %f", res2.Items[0].BOEBCDRate)
	}

	// 3. 4 columns without method (defaults to Standard)
	csv3 := "PN-101,7.5,10,18\nPN-102,12.5,10,18\n"
	res3, err := svc.ParseOverrideCSV(csv3, currentItems)
	if err != nil || res3.AppliedCount != 2 {
		t.Fatalf("test 3 failed: err=%v, res=%+v", err, res3)
	}
	if res3.Items[0].CalculationMethod != "Standard" || res3.Items[0].BOEBCDRate != 7.5 {
		t.Errorf("test 3 item 0 mismatch: %+v", res3.Items[0])
	}

	// 4. Unknown part number produces clear warning
	csv4 := "UNKNOWN-PART,Standard,10,10,18\n"
	res4, err := svc.ParseOverrideCSV(csv4, currentItems)
	if err != nil || res4.SkippedCount != 1 {
		t.Fatalf("test 4 failed: err=%v, res=%+v", err, res4)
	}
	if len(res4.Warnings) == 0 || !strings.Contains(res4.Warnings[0].Message, "UNKNOWN-PART") {
		t.Errorf("expected warning to mention UNKNOWN-PART, got: %+v", res4.Warnings)
	}

	// 5. IGST mismatch produces clear warning
	csv5 := "PN-101,Standard,10,10,28\n"
	res5, err := svc.ParseOverrideCSV(csv5, currentItems)
	if err != nil || res5.SkippedCount != 1 {
		t.Fatalf("test 5 failed: err=%v, res=%+v", err, res5)
	}
	if len(res5.Warnings) == 0 || !strings.Contains(res5.Warnings[0].Message, "actualIgstRate") {
		t.Errorf("expected IGST warning, got: %+v", res5.Warnings)
	}
}
