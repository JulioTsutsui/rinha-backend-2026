package main

import (
	"slices"
	"time"
)

const FRAUD_THRESHOLD float32 = 0.6
const TOP_K int = 5

func FraudScoreCalculator(ctx FraudScoreContext) FraudScoreResponse {
	req := ctx.req
	norm := ctx.norm

	var minutesLastTx float32 = -1
	var kmLastTx float32 = -1
	if req.LastTransaction != nil {
		minutesLastTx = Limitar(float32(req.Transaction.RequestedAt.Sub(req.LastTransaction.Timestamp).Minutes()) / norm.MaxMinutes)
		kmLastTx = Limitar(float32(req.LastTransaction.KmFromCurrent) / norm.MaxKM)
	}

	vec := []float32{
		Limitar(float32(req.Transaction.Amount) / norm.MaxAmount),
		Limitar(float32(req.Transaction.Installments) / norm.MaxInstallments),
		Limitar((float32(req.Transaction.Amount) / float32(req.Customer.AvgAmount)) / norm.AmountVsAvgRatio),
		Limitar(float32(req.Transaction.RequestedAt.Hour()) / 23.0),
		Limitar(float32(correctWeekday(req.Transaction.RequestedAt.Weekday())) / 6.0),
		minutesLastTx,
		kmLastTx,
		Limitar(float32(req.Terminal.KmFromHome) / norm.MaxKM),
		Limitar(float32(req.Customer.TxCount24h) / norm.MaxTxCount24),
		Bool2Float(req.Terminal.IsOnline),
		Bool2Float(req.Terminal.CardPresent),
		checkMerchants(req.Merchant.ID, req.Customer.KnownMerchants),
		checkMccRisk(ctx.mcc_risk, req.Merchant.MCC),
		Limitar(float32(req.Merchant.AvgAmount) / norm.MaxMerchantAvgAmount),
	}

	labels := ctx.index.SearchTopK(vec, TOP_K)

	fraud := 0
	for _, l := range labels {
		if l == "fraud" {
			fraud++
		}
	}

	fraudScore := float32(fraud) / float32(TOP_K)
	return FraudScoreResponse{
		Approved:   fraudScore < FRAUD_THRESHOLD,
		FraudScore: fraudScore,
	}
}

// Monday-indexed weekday (Monday = 0, Tuesday = 1, ..., Sunday = 6)
func correctWeekday(weekday time.Weekday) int {
	mondayFirst := (weekday + 6) % 7
	return int(mondayFirst)
}

func checkMerchants(merchID string, merchList []string) float32 {
	if slices.Contains(merchList, merchID) {
		return 0
	}

	return 1 // 1 == unknown merchant
}

func checkMccRisk(mcc_risk map[string]float32, mcc string) float32 {
	if val, ok := mcc_risk[mcc]; ok {
		return val
	}

	return 0.5
}
