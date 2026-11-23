package landedcost

import (
	"errors"
	"fmt"
	"math"
)

// AllocationBasis defines the weighting method for apportioning common import expenses.
type AllocationBasis string

const (
	BasisAssessableValue AllocationBasis = "ASSESSABLE_VALUE"
	BasisInvoiceValue    AllocationBasis = "INVOICE_VALUE"
	BasisGrossWeight     AllocationBasis = "GROSS_WEIGHT"
	BasisQuantity        AllocationBasis = "QUANTITY"
)

// ItemLandedCostInput represents line inputs for expense allocation and landed cost calculation.
type ItemLandedCostInput struct {
	ItemID                 string         `json:"itemId"`
	PartNumber             string         `json:"partNumber"`
	Quantity               float64        `json:"quantity"`
	BasisValue             float64        `json:"basisValue"`             // Assessable Value, Invoice Value, or Weight
	AssessableValue        float64        `json:"assessableValue"`        // INR
	BCDAmount              float64        `json:"bcdAmount"`              // INR
	SWSAmount              float64        `json:"swsAmount"`              // INR
	IGSTAmount             float64        `json:"igstAmount"`             // INR
	OtherDutiesAmount      float64        `json:"otherDutiesAmount"`      // Anti-dumping / safeguard duties
	ITCEligibility         ITCEligibility `json:"itcEligibility"`         // ITC_ELIGIBLE | ITC_INELIGIBLE_17_5 | ITCCapitalGoods
}

// ExpenseItemInput represents a basic expense to be allocated.
type ExpenseItemInput struct {
	ExpenseID   string `json:"expenseId"`
	Category    string `json:"category"`
	AmountPaise int64  `json:"amountPaise"`
}

// LineLandedCostResult captures the landed cost breakdown for a single item.
type LineLandedCostResult struct {
	ItemID                string  `json:"itemId"`
	PartNumber            string  `json:"partNumber"`
	Quantity              float64 `json:"quantity"`
	AssessableValue       float64 `json:"assessableValue"`
	BCDAmount             float64 `json:"bcdAmount"`
	SWSAmount             float64 `json:"swsAmount"`
	IGSTAmount            float64 `json:"igstAmount"`
	IGSTIncludedInCost    bool    `json:"igstIncludedInCost"`
	AllocatedExpensePaise int64   `json:"allocatedExpensePaise"`
	AllocatedExpenseINR   float64 `json:"allocatedExpenseInr"`
	TotalInventoryCostINR float64 `json:"totalInventoryCostInr"`
	LDCPerQuantity        float64 `json:"ldcPerQty"`
}

// AllocationSummary aggregates allocated expenses and landed costs across all items.
type AllocationSummary struct {
	Items                   []LineLandedCostResult `json:"items"`
	TotalAllocatedPaise     int64                  `json:"totalAllocatedPaise"`
	TotalAllocatedINR       float64                `json:"totalAllocatedInr"`
	TotalInventoryCostINR   float64                `json:"totalInventoryCostInr"`
	ResidualPaiseBalanced   bool                   `json:"residualPaiseBalanced"`
}

// roundTo2 rounds a float to 2 decimal places.
func roundTo2(val float64) float64 {
	return math.Round((val+math.Copysign(1e-9, val))*100.0) / 100.0
}

// ComputeLandedCost allocates common import expenses across items and calculates landed cost per unit.
// Enforces residual paise balancing on the final item to guarantee exact integer sum parity.
func ComputeLandedCost(items []ItemLandedCostInput, expenses []ExpenseItemInput, basis AllocationBasis) (AllocationSummary, error) {
	if len(items) == 0 {
		return AllocationSummary{}, errors.New("at least one line item is required for landed cost allocation")
	}

	var totalBaseExpensePaise int64
	for _, exp := range expenses {
		if exp.AmountPaise < 0 {
			return AllocationSummary{}, fmt.Errorf("expense %s has negative amount", exp.ExpenseID)
		}
		totalBaseExpensePaise += exp.AmountPaise
	}

	// Calculate total allocation basis
	var totalBasis float64
	for _, it := range items {
		if it.BasisValue > 0 {
			totalBasis += it.BasisValue
		}
	}

	// Fallback to equal pro-rata if totalBasis is 0
	if totalBasis <= 0 {
		totalBasis = float64(len(items))
	}

	summary := AllocationSummary{
		Items: make([]LineLandedCostResult, 0, len(items)),
	}

	var allocatedPaiseSum int64

	for i, it := range items {
		var itemPaise int64

		if totalBaseExpensePaise > 0 {
			if i == len(items)-1 {
				// Residual balancing on terminal item
				itemPaise = totalBaseExpensePaise - allocatedPaiseSum
				summary.ResidualPaiseBalanced = true
			} else {
				weight := it.BasisValue
				if it.BasisValue <= 0 {
					weight = 1.0 // fallback
				}
				itemPaise = int64(math.Round(float64(totalBaseExpensePaise) * (weight / totalBasis)))
				allocatedPaiseSum += itemPaise
			}
		}

		itemExpenseINR := float64(itemPaise) / 100.0
		summary.TotalAllocatedPaise += itemPaise

		// Landed Cost Formula: AV + BCD + SWS + OtherDuties + AllocatedExpenses (+ NonRecoverableIGST if ineligible)
		igstInCost := ShouldIncludeInLandedCost(it.ITCEligibility)
		nonRecoverableIGST := 0.0
		if igstInCost {
			nonRecoverableIGST = it.IGSTAmount
		}

		totalCost := it.AssessableValue + it.BCDAmount + it.SWSAmount + it.OtherDutiesAmount + itemExpenseINR + nonRecoverableIGST
		totalCost = roundTo2(totalCost)

		var ldcPerQty float64
		if it.Quantity > 0 {
			ldcPerQty = roundTo2(totalCost / it.Quantity)
		}

		summary.TotalInventoryCostINR = roundTo2(summary.TotalInventoryCostINR + totalCost)

		summary.Items = append(summary.Items, LineLandedCostResult{
			ItemID:                it.ItemID,
			PartNumber:            it.PartNumber,
			Quantity:              it.Quantity,
			AssessableValue:       it.AssessableValue,
			BCDAmount:             it.BCDAmount,
			SWSAmount:             it.SWSAmount,
			IGSTAmount:            it.IGSTAmount,
			IGSTIncludedInCost:    igstInCost,
			AllocatedExpensePaise: itemPaise,
			AllocatedExpenseINR:   itemExpenseINR,
			TotalInventoryCostINR: totalCost,
			LDCPerQuantity:        ldcPerQty,
		})
	}

	summary.TotalAllocatedINR = float64(summary.TotalAllocatedPaise) / 100.0
	return summary, nil
}
