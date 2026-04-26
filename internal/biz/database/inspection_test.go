package database

import "testing"

func TestCalculateInspectionScore(t *testing.T) {
	cases := []struct {
		name     string
		findings []*DatabaseInspectionFindingVO
		score    int
		risk     string
	}{
		{name: "clean", score: 100, risk: DatabaseQueryRiskLow},
		{
			name: "medium",
			findings: []*DatabaseInspectionFindingVO{
				{Severity: "warning"},
			},
			score: 85,
			risk:  DatabaseQueryRiskMedium,
		},
		{
			name: "high",
			findings: []*DatabaseInspectionFindingVO{
				{Severity: "warning"},
				{Severity: "warning"},
			},
			score: 70,
			risk:  DatabaseQueryRiskHigh,
		},
		{
			name: "critical",
			findings: []*DatabaseInspectionFindingVO{
				{Severity: "critical"},
				{Severity: "warning"},
				{Severity: "info"},
			},
			score: 55,
			risk:  DatabaseQueryRiskCritical,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			score, risk := calculateInspectionScore(tt.findings)
			if score != tt.score {
				t.Fatalf("score = %d, want %d", score, tt.score)
			}
			if risk != tt.risk {
				t.Fatalf("risk = %q, want %q", risk, tt.risk)
			}
		})
	}
}

func TestCalculateCapacityGrowth(t *testing.T) {
	growthBytes, growthPercent := calculateCapacityGrowth([]*DatabaseCapacitySnapshot{
		{TotalSizeBytes: 200},
		{TotalSizeBytes: 260},
	})

	if growthBytes != 60 {
		t.Fatalf("growth bytes = %d, want 60", growthBytes)
	}
	if growthPercent != 30 {
		t.Fatalf("growth percent = %.2f, want 30.00", growthPercent)
	}
}
