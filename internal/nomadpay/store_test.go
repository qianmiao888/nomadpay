package nomadpay

import (
	"path/filepath"
	"testing"
)

func TestPaymentReconciliationAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	recipient := "0x1111111111111111111111111111111111111111"
	invoice, err := store.Create(recipient, "10000000000000000", "0.01", "Design work")
	if err != nil {
		t.Fatal(err)
	}
	wrong := Payment{InvoiceID: invoice.ID, Recipient: recipient, Payer: "0x2222222222222222222222222222222222222222", AmountWei: "1", TxHash: "0xabc", EventKey: "wrong:0"}
	if err := store.ApplyBlock(11, []Payment{wrong}); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(invoice.ID)
	if got.Status != "pending" {
		t.Fatal("wrong amount marked invoice paid")
	}
	right := wrong
	right.AmountWei = invoice.AmountWei
	right.TxHash = "0xdef"
	right.EventKey = "right:0"
	if err := store.ApplyBlock(12, []Payment{right, right}); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Get(invoice.ID)
	if got.Status != "paid" || got.TxHash != "0xdef" {
		t.Fatalf("unexpected paid invoice: %+v", got)
	}
	restarted, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.NextBlock(0) != 12 {
		t.Fatal("cursor lost after restart")
	}
	restored, _ := restarted.Get(invoice.ID)
	if restored.Status != "paid" {
		t.Fatal("payment lost after restart")
	}
	if err := restarted.ApplyBlock(13, []Payment{right}); err != nil {
		t.Fatal(err)
	}
	restored, _ = restarted.Get(invoice.ID)
	if restored.TxHash != "0xdef" {
		t.Fatal("replay changed payment")
	}
}

func TestFreshStoreSkipsPastBlocksButExistingInvoiceDoesNot(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "empty.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitializeCursorIfEmpty(900); err != nil {
		t.Fatal(err)
	}
	if got := store.NextBlock(100); got != 900 {
		t.Fatalf("fresh cursor = %d", got)
	}
	other, err := OpenStore(filepath.Join(t.TempDir(), "invoice.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Create("0x1111111111111111111111111111111111111111", "1", "0.000000000000000001", "Existing invoice"); err != nil {
		t.Fatal(err)
	}
	if err := other.InitializeCursorIfEmpty(900); err != nil {
		t.Fatal(err)
	}
	if got := other.NextBlock(100); got != 100 {
		t.Fatalf("existing invoice skipped history: %d", got)
	}
}
