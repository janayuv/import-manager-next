package reconciliation

import (
	"math"
)

// ReconcileStatus represents the status of a BOE matching pass.
type ReconcileStatus string

const (
	StatusReconciled       ReconcileStatus = "Reconciled"
	StatusDiscrepancyFound ReconcileStatus = "Discrepancy Found"
	StatusAwaitingBOEData  ReconcileStatus = "Awaiting BOE Data"
)

// DefaultToleranceINR is the standard rounding drift tolerance allowed (±2 INR).
const DefaultToleranceINR = 2.0

// BOEReconcileResult captures duty matching outcome.
type BOEReconcileResult struct {
	Status           ReconcileStatus `json:"status"`
	DutyPaid         float64         `json:"dutyPaid"`
	CalculatedDuty   float64         `json:"calculatedDuty"`
	DifferenceINR    float64         `json:"differenceInr"`
	ToleranceINR     float64         `json:"toleranceInr"`
	IsMatched        bool            `json:"isMatched"`
	HasDiscrepancy   bool            `json:"hasDiscrepancy"`
	DiscrepancyNotes string          `json:"discrepancyNotes,omitempty"`
}

// ReconcileBOEDuty compares the actual duty paid on a Bill of Entry against the calculated customs duty total.
func ReconcileBOEDuty(dutyPaid, calculatedDuty, toleranceINR float64) BOEReconcileResult {
	if toleranceINR <= 0 {
		toleranceINR = DefaultToleranceINR
	}

	if dutyPaid <= 0 && calculatedDuty > 0 {
		return BOEReconcileResult{
			Status:         StatusAwaitingBOEData,
			DutyPaid:       dutyPaid,
			CalculatedDuty: calculatedDuty,
			ToleranceINR:   toleranceINR,
			IsMatched:      false,
			HasDiscrepancy: false,
		}
	}

	diff := math.Abs(dutyPaid - calculatedDuty)
	matched := diff <= toleranceINR

	status := StatusReconciled
	var notes string
	if !matched {
		status = StatusDiscrepancyFound
		notes = "Actual duty paid differs from calculated duty beyond tolerance threshold"
	}

	return BOEReconcileResult{
		Status:           status,
		DutyPaid:         dutyPaid,
		CalculatedDuty:   calculatedDuty,
		DifferenceINR:    math.Round(diff*100.0) / 100.0,
		ToleranceINR:     toleranceINR,
		IsMatched:        matched,
		HasDiscrepancy:   !matched,
		DiscrepancyNotes: notes,
	}
}
