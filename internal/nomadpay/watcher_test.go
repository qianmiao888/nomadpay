package nomadpay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatcherSyncConfirmedPayment(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	recipient := "0x1111111111111111111111111111111111111111"
	invoice, err := store.Create(recipient, "42", "0.000000000000000042", "Test")
	if err != nil {
		t.Fatal(err)
	}
	contract := "0x3333333333333333333333333333333333333333"
	topic := "0x" + strings.Repeat("a", 64)
	payer := "0x" + strings.Repeat("0", 24) + strings.Repeat("2", 40)
	recipientTopic := "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40)
	log := rpcLog{Address: contract, Topics: []string{topic, invoice.ID, payer, recipientTopic}, Data: "0x" + strings.Repeat("0", 62) + "2a", TransactionHash: "0x" + strings.Repeat("b", 64), LogIndex: "0x0"}
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		var result any
		switch request.Method {
		case "eth_chainId":
			result = "0x279f"
		case "eth_blockNumber":
			result = "0x16"
		case "eth_getLogs":
			result = []rpcLog{log}
		default:
			t.Errorf("unexpected RPC method %s", request.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer rpc.Close()
	watcher := NewWatcher(Config{RPCURL: rpc.URL, ChainID: 10143, Contract: contract, EventTopic: topic, StartBlock: 10, Confirmations: 12}, store, rpc.Client())
	if err := watcher.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(invoice.ID)
	if got.Status != "paid" {
		t.Fatalf("invoice not paid: %+v", got)
	}
	if store.NextBlock(0) != 11 {
		t.Fatalf("unexpected cursor %d", store.NextBlock(0))
	}
}
