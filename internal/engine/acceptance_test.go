package engine

import "testing"

func TestAcceptanceMatrixDoesNotDeclareMVPWithoutAllScenarios(t *testing.T) {
	t.Parallel()
	matrix := NewAcceptanceMatrix()
	updated, err := matrix.Record(AcceptanceRecord{ID: "AT-001", Passed: true, Device: "test", Evidence: "unit"})
	if err != nil {
		t.Fatal(err)
	}
	if err := updated.ValidateMVP(); err == nil {
		t.Fatal("incomplete acceptance matrix was declared MVP")
	}
}
