package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"nomadpay/internal/nomadpay"
)

func TestInvoiceAPI(t *testing.T) {
	store, err := nomadpay.OpenStore(filepath.Join(t.TempDir(), "invoices.json"))
	if err != nil {
		t.Fatal(err)
	}
	api := &server{store: store}
	request := func(amount string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"recipient": "0x1111111111111111111111111111111111111111", "amountMon": amount, "description": "Design work"})
		r := httptest.NewRequest(http.MethodPost, "/api/invoices", bytes.NewReader(body))
		w := httptest.NewRecorder()
		api.create(w, r)
		return w
	}
	for _, amount := range []string{"1/2", "0", "0.0000000000000000001"} {
		if got := request(amount).Code; got != 400 {
			t.Errorf("amount %s: got status %d", amount, got)
		}
	}
	w := request("0.01")
	if w.Code != 201 {
		t.Fatalf("create: %s", w.Body.String())
	}
	var invoice nomadpay.Invoice
	if err := json.Unmarshal(w.Body.Bytes(), &invoice); err != nil {
		t.Fatal(err)
	}
	if invoice.AmountWei != "10000000000000000" {
		t.Fatalf("wrong wei: %s", invoice.AmountWei)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/invoices/"+invoice.ID, nil)
	r.SetPathValue("id", invoice.ID)
	w = httptest.NewRecorder()
	api.get(w, r)
	if w.Code != 200 {
		t.Fatalf("get: %s", w.Body.String())
	}
}
