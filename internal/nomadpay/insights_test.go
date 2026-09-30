package nomadpay

import "testing"

func TestAnomalyModelAbstainsWithoutHistory(t *testing.T) {
	report := AnalyzePayments(SyntheticInvoices()[:1], false)
	if report.Status != "insufficient_data" || report.SampleCount != 1 || len(report.Anomalies) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestAnomalyModelFindsUnusualSyntheticPayment(t *testing.T) {
	report := AnalyzePayments(SyntheticInvoices(), true)
	if report.Status != "ready" || report.SampleCount != 25 || !report.Synthetic {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Anomalies) != 1 || report.Anomalies[0].InvoiceID != "sample-unusual" {
		t.Fatalf("unexpected anomalies: %+v", report.Anomalies)
	}
}
