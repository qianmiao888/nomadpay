package nomadpay

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"
)

const MinAnomalySamples = 20

// PaymentAnomaly is a paid invoice whose amount or settlement delay is far
// from the recipient's own history. The score is a robust standardized distance.
type PaymentAnomaly struct {
	InvoiceID  string  `json:"invoiceId"`
	AmountMON  string  `json:"amountMon"`
	DelayHours float64 `json:"delayHours"`
	Score      float64 `json:"score"`
}

type Insights struct {
	Status           string           `json:"status"`
	SampleCount      int              `json:"sampleCount"`
	MinimumSamples   int              `json:"minimumSamples"`
	Synthetic        bool             `json:"synthetic"`
	MedianAmountMON  float64          `json:"medianAmountMon,omitempty"`
	MedianDelayHours float64          `json:"medianDelayHours,omitempty"`
	Anomalies        []PaymentAnomaly `json:"anomalies"`
	Explanation      string           `json:"explanation"`
}

type paymentPoint struct {
	invoice  Invoice
	amount   float64
	delay    float64
	features [2]float64
}

func median(values []float64) float64 {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	n := len(copyValues)
	if n%2 == 1 {
		return copyValues[n/2]
	}
	return (copyValues[n/2-1] + copyValues[n/2]) / 2
}

// AnalyzePayments fits an unsupervised robust baseline to paid invoices.
// It deliberately abstains with fewer than 20 samples: one-off payments cannot
// support a meaningful personalized anomaly score.
func AnalyzePayments(invoices []Invoice, synthetic bool) Insights {
	report := Insights{Status: "insufficient_data", MinimumSamples: MinAnomalySamples, Synthetic: synthetic, Anomalies: []PaymentAnomaly{}, Explanation: "At least 20 paid invoices are needed for a personal baseline."}
	points := make([]paymentPoint, 0, len(invoices))
	for _, invoice := range invoices {
		if invoice.Status != "paid" || invoice.PaidAt == nil {
			continue
		}
		wei, ok := new(big.Int).SetString(invoice.AmountWei, 10)
		if !ok || wei.Sign() <= 0 {
			continue
		}
		amountBig := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e18))
		amount, _ := amountBig.Float64()
		delay := invoice.PaidAt.Sub(invoice.CreatedAt).Hours()
		if delay < 0 || math.IsInf(amount, 0) || amount <= 0 {
			continue
		}
		points = append(points, paymentPoint{invoice: invoice, amount: amount, delay: delay, features: [2]float64{math.Log10(amount), math.Log1p(delay)}})
	}
	report.SampleCount = len(points)
	if len(points) < MinAnomalySamples {
		return report
	}
	amounts := make([]float64, len(points))
	delays := make([]float64, len(points))
	featureA := make([]float64, len(points))
	featureD := make([]float64, len(points))
	for i, p := range points {
		amounts[i] = p.amount
		delays[i] = p.delay
		featureA[i] = p.features[0]
		featureD[i] = p.features[1]
	}
	report.MedianAmountMON = median(amounts)
	report.MedianDelayHours = math.Round(median(delays)*100) / 100
	medA, medD := median(featureA), median(featureD)
	deviationsA := make([]float64, len(points))
	deviationsD := make([]float64, len(points))
	for i, p := range points {
		deviationsA[i] = math.Abs(p.features[0] - medA)
		deviationsD[i] = math.Abs(p.features[1] - medD)
	}
	scaleA := math.Max(1.4826*median(deviationsA), 0.1)
	scaleD := math.Max(1.4826*median(deviationsD), 0.25)
	for _, p := range points {
		zA := (p.features[0] - medA) / scaleA
		zD := (p.features[1] - medD) / scaleD
		score := math.Hypot(zA, zD)
		if score >= 3.5 {
			report.Anomalies = append(report.Anomalies, PaymentAnomaly{InvoiceID: p.invoice.ID, AmountMON: p.invoice.AmountMON, DelayHours: math.Round(p.delay*100) / 100, Score: math.Round(score*100) / 100})
		}
	}
	sort.Slice(report.Anomalies, func(i, j int) bool { return report.Anomalies[i].Score > report.Anomalies[j].Score })
	report.Status = "ready"
	report.Explanation = "Unsupervised robust anomaly detection on log payment amount and log invoice-to-reconciliation delay. Scores above 3.5 are review prompts, not fraud verdicts."
	return report
}

// SyntheticInvoices exists only to demonstrate the model before a recipient
// has enough real payment history. It is never mixed with live account data.
func SyntheticInvoices() []Invoice {
	base := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	out := make([]Invoice, 0, 25)
	for i := 0; i < 24; i++ {
		created := base.Add(time.Duration(i) * 24 * time.Hour)
		paid := created.Add(time.Duration(2+i%4) * time.Hour)
		milliMON := 20 + 2*(i%5)
		out = append(out, Invoice{ID: fmt.Sprintf("sample-%d", i+1), AmountMON: fmt.Sprintf("0.%03d", milliMON), AmountWei: fmt.Sprintf("%d000000000000000", milliMON), Status: "paid", CreatedAt: created, PaidAt: &paid})
	}
	created := base.Add(25 * 24 * time.Hour)
	paid := created.Add(48 * time.Hour)
	out = append(out, Invoice{ID: "sample-unusual", AmountMON: "1", AmountWei: "1000000000000000000", Status: "paid", CreatedAt: created, PaidAt: &paid})
	return out
}
