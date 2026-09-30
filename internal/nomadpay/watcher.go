package nomadpay

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

type Config struct {
	RPCURL        string
	ChainID       uint64
	Contract      string
	EventTopic    string
	StartBlock    uint64
	Confirmations uint64
	ExplorerURL   string
}
type Watcher struct {
	cfg    Config
	store  *Store
	client *http.Client
}

func NewWatcher(cfg Config, store *Store, client *http.Client) *Watcher {
	return &Watcher{cfg, store, client}
}

func (w *Watcher) rpc(ctx context.Context, method string, params any, out any) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, err := http.NewRequestWithContext(ctx, "POST", w.cfg.RPCURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("RPC HTTP %d", res.StatusCode)
	}
	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return err
	}
	if response.Error != nil {
		return errors.New(response.Error.Message)
	}
	if len(response.Result) == 0 {
		return errors.New("RPC result missing")
	}
	return json.Unmarshal(response.Result, out)
}
func hexUint(v uint64) string { return fmt.Sprintf("0x%x", v) }
func parseHexUint(s string) (uint64, error) {
	return strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 64)
}

type rpcLog struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	BlockNumber     string   `json:"blockNumber"`
	TransactionHash string   `json:"transactionHash"`
	LogIndex        string   `json:"logIndex"`
	Removed         bool     `json:"removed"`
}

func decodePayment(log rpcLog, contract, eventTopic string) (Payment, error) {
	if log.Removed || !strings.EqualFold(log.Address, contract) || len(log.Topics) != 4 || !strings.EqualFold(log.Topics[0], eventTopic) {
		return Payment{}, errors.New("unrecognized event")
	}
	for _, topic := range log.Topics {
		if len(topic) != 66 {
			return Payment{}, errors.New("invalid topic")
		}
		if _, err := hex.DecodeString(topic[2:]); err != nil {
			return Payment{}, err
		}
	}
	if len(log.Data) != 66 {
		return Payment{}, errors.New("invalid amount")
	}
	amount, ok := new(big.Int).SetString(log.Data[2:], 16)
	if !ok {
		return Payment{}, errors.New("invalid amount")
	}
	payer := "0x" + strings.ToLower(log.Topics[2][26:])
	recipient := "0x" + strings.ToLower(log.Topics[3][26:])
	if !ValidAddress(payer) || !ValidAddress(recipient) {
		return Payment{}, errors.New("invalid address")
	}
	if len(log.TransactionHash) != 66 {
		return Payment{}, errors.New("invalid transaction hash")
	}
	idx, err := parseHexUint(log.LogIndex)
	if err != nil {
		return Payment{}, err
	}
	return Payment{InvoiceID: strings.ToLower(log.Topics[1]), Payer: payer, Recipient: recipient, AmountWei: amount.String(), TxHash: strings.ToLower(log.TransactionHash), EventKey: fmt.Sprintf("%s:%d", strings.ToLower(log.TransactionHash), idx)}, nil
}

// Sync only indexes blocks with the configured confirmation depth. The cursor is saved
// together with payment updates, so restarts can replay the last unsaved range safely.
func (w *Watcher) Sync(ctx context.Context) error {
	if w.cfg.Contract == "" || w.cfg.EventTopic == "" {
		return errors.New("contract and event topic required")
	}
	var chainHex string
	if err := w.rpc(ctx, "eth_chainId", []any{}, &chainHex); err != nil {
		return err
	}
	chain, err := parseHexUint(chainHex)
	if err != nil {
		return err
	}
	if chain != w.cfg.ChainID {
		return fmt.Errorf("RPC chain ID %d does not match configured %d", chain, w.cfg.ChainID)
	}
	var headHex string
	if err := w.rpc(ctx, "eth_blockNumber", []any{}, &headHex); err != nil {
		return err
	}
	head, err := parseHexUint(headHex)
	if err != nil {
		return err
	}
	if head < w.cfg.Confirmations {
		return nil
	}
	safe := head - w.cfg.Confirmations
	initial := safe + 1
	if initial < w.cfg.StartBlock {
		initial = w.cfg.StartBlock
	}
	if err := w.store.InitializeCursorIfEmpty(initial); err != nil {
		return err
	}
	from := w.store.NextBlock(w.cfg.StartBlock)
	if from > safe {
		return nil
	}
	// Public Monad RPCs limit eth_getLogs range. Keep requests small and bounded.
	for from <= safe {
		to := from + 49
		if to < from || to > safe {
			to = safe
		}
		var logs []rpcLog
		filter := map[string]any{"fromBlock": hexUint(from), "toBlock": hexUint(to), "address": w.cfg.Contract, "topics": []string{w.cfg.EventTopic}}
		if err := w.rpc(ctx, "eth_getLogs", []any{filter}, &logs); err != nil {
			return err
		}
		payments := make([]Payment, 0, len(logs))
		for _, entry := range logs {
			p, err := decodePayment(entry, w.cfg.Contract, w.cfg.EventTopic)
			if err != nil {
				return fmt.Errorf("decode log: %w", err)
			}
			payments = append(payments, p)
		}
		if err := w.store.ApplyBlock(to+1, payments); err != nil {
			return err
		}
		from = to + 1
	}
	return nil
}
