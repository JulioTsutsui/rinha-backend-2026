package main

const (
	Dims     = 14
	Scale    = 10000
	Sentinel = -10000
)

const (
	maxAmount            = 10000.0
	maxInstallments      = 12.0
	amountVsAvgRatio     = 10.0
	maxMinutes           = 1440.0
	maxKm                = 1000.0
	maxTxCount24h        = 20.0
	maxMerchantAvgAmount = 10000.0
)

const defaultMccRisk = 0.5

var mccRisk = map[int]float64{
	5411: 0.15, 5812: 0.30, 5912: 0.20, 5944: 0.45,
	7801: 0.80, 7802: 0.75, 7995: 0.85, 4511: 0.35,
	5311: 0.25, 5999: 0.50,
}

var mccRiskTable [10000]int16

func init() {
	d := quantClamp01(defaultMccRisk)
	for i := range mccRiskTable {
		mccRiskTable[i] = d
	}
	for code, risk := range mccRisk {
		if code >= 0 && code < 10000 {
			mccRiskTable[code] = quantClamp01(risk)
		}
	}
}

func quantizeRef(v float64) int16 {
	if v < 0 {
		return Sentinel
	}
	if v >= 1 {
		return Scale
	}
	return int16(v*Scale + 0.5)
}

func quantClamp01(x float64) int16 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return Scale
	}
	return int16(x*Scale + 0.5)
}
