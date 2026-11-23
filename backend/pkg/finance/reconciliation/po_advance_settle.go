package reconciliation

import (
	"errors"
	"fmt"
	"math"
)

// POAdvanceAllocation represents an allocated portion of a PO advance against an invoice.
type POAdvanceAllocation struct {
	PaymentID          string  `json:"paymentId"`
	InvoiceID          string  `json:"invoiceId"`
	AllocatedAmount    float64 `json:"allocatedAmount"`    // in payment currency
	ExchangeRate       float64 `json:"exchangeRate"`       // FX rate to invoice currency
	ConvertedAmount    float64 `json:"convertedAmount"`    // in invoice currency
}

// SettlementResult captures the advance allocation pass outcome.
type SettlementResult struct {
	TotalInvoiceValue   float64               `json:"totalInvoiceValue"`
	TotalAllocated      float64               `json:"totalAllocated"`      // in invoice currency
	RemainingUnsettled  float64               `json:"remainingUnsettled"`  // in invoice currency
	IsFullySettled      bool                  `json:"isFullySettled"`
	Allocations         []POAdvanceAllocation `json:"allocations"`
}

// SettleAdvanceAgainstInvoice allocates available advance payments against an invoice total.
func SettleAdvanceAgainstInvoice(invoiceValue float64, availableAdvances []POAdvanceAllocation) (SettlementResult, error) {
	if invoiceValue < 0 {
		return SettlementResult{}, errors.New("invoice value cannot be negative")
	}

	remainingToSettle := invoiceValue
	var totalAllocated float64
	var actualAllocations []POAdvanceAllocation

	for _, adv := range availableAdvances {
		if remainingToSettle <= 0 {
			break
		}
		if adv.AllocatedAmount <= 0 {
			continue
		}
		fxRate := adv.ExchangeRate
		if fxRate <= 0 {
			fxRate = 1.0
		}

		availableConverted := adv.AllocatedAmount * fxRate
		allocatedConverted := math.Min(availableConverted, remainingToSettle)
		allocatedOriginal := allocatedConverted / fxRate

		allocatedConverted = math.Round(allocatedConverted*100.0) / 100.0
		allocatedOriginal = math.Round(allocatedOriginal*100.0) / 100.0

		actualAllocations = append(actualAllocations, POAdvanceAllocation{
			PaymentID:       adv.PaymentID,
			InvoiceID:       adv.InvoiceID,
			AllocatedAmount: allocatedOriginal,
			ExchangeRate:    fxRate,
			ConvertedAmount: allocatedConverted,
		})

		remainingToSettle -= allocatedConverted
		totalAllocated += allocatedConverted
	}

	remainingToSettle = math.Max(0, math.Round(remainingToSettle*100.0)/100.0)
	totalAllocated = math.Round(totalAllocated*100.0) / 100.0

	return SettlementResult{
		TotalInvoiceValue:  invoiceValue,
		TotalAllocated:     totalAllocated,
		RemainingUnsettled: remainingToSettle,
		IsFullySettled:     remainingToSettle == 0,
		Allocations:        actualAllocations,
	}, nil
}

// CheckReconciliationParity ensures total advance does not exceed original PO value.
func CheckReconciliationParity(poValue, advanceSum float64) error {
	if advanceSum > poValue {
		return fmt.Errorf("advance sum (%.2f) cannot exceed PO total value (%.2f)", advanceSum, poValue)
	}
	return nil
}
