package nomadpay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Invoice struct {
	ID          string     `json:"id"`
	Recipient   string     `json:"recipient"`
	AmountWei   string     `json:"amountWei"`
	AmountMON   string     `json:"amountMon"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	PaidWei     string     `json:"paidWei,omitempty"`
	Payer       string     `json:"payer,omitempty"`
	TxHash      string     `json:"txHash,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	PaidAt      *time.Time `json:"paidAt,omitempty"`
}

type state struct {
	Invoices  map[string]Invoice `json:"invoices"`
	Seen      map[string]bool    `json:"seen"`
	NextBlock uint64             `json:"nextBlock"`
}
type Store struct {
	mu    sync.Mutex
	path  string
	state state
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, state: state{Invoices: map[string]Invoice{}, Seen: map[string]bool{}}}
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &s.state); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.state.Invoices == nil {
		s.state.Invoices = map[string]Invoice{}
	}
	if s.state.Seen == nil {
		s.state.Seen = map[string]bool{}
	}
	return s, nil
}
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
func (s *Store) Create(recipient, amountWei, amountMON, description string) (Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idBytes := make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return Invoice{}, err
	}
	v := Invoice{ID: "0x" + hex.EncodeToString(idBytes), Recipient: recipient, AmountWei: amountWei, AmountMON: amountMON, Description: description, Status: "pending", CreatedAt: time.Now().UTC()}
	s.state.Invoices[v.ID] = v
	if err := s.save(); err != nil {
		delete(s.state.Invoices, v.ID)
		return Invoice{}, err
	}
	return v, nil
}
func (s *Store) Get(id string) (Invoice, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.state.Invoices[strings.ToLower(id)]
	return v, ok
}
func (s *Store) List(recipient string) []Invoice {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Invoice{}
	for _, v := range s.state.Invoices {
		if v.Recipient == recipient {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
func (s *Store) NextBlock(start uint64) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.NextBlock == 0 {
		return start
	}
	return s.state.NextBlock
}

type Payment struct{ InvoiceID, Payer, Recipient, AmountWei, TxHash, EventKey string }

func (s *Store) ApplyBlock(nextBlock uint64, payments []Payment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state
	copyState := state{Invoices: make(map[string]Invoice, len(old.Invoices)), Seen: make(map[string]bool, len(old.Seen)), NextBlock: nextBlock}
	for k, v := range old.Invoices {
		copyState.Invoices[k] = v
	}
	for k, v := range old.Seen {
		copyState.Seen[k] = v
	}
	for _, p := range payments {
		if copyState.Seen[p.EventKey] {
			continue
		}
		copyState.Seen[p.EventKey] = true
		v, ok := copyState.Invoices[strings.ToLower(p.InvoiceID)]
		if !ok || v.Status != "pending" || v.Recipient != strings.ToLower(p.Recipient) || v.AmountWei != p.AmountWei {
			continue
		}
		now := time.Now().UTC()
		v.Status = "paid"
		v.PaidWei = p.AmountWei
		v.Payer = strings.ToLower(p.Payer)
		v.TxHash = p.TxHash
		v.PaidAt = &now
		copyState.Invoices[v.ID] = v
	}
	s.state = copyState
	if err := s.save(); err != nil {
		s.state = old
		return err
	}
	return nil
}

func ValidAddress(s string) bool {
	if len(s) != 42 || !strings.HasPrefix(s, "0x") {
		return false
	}
	if s == "0x0000000000000000000000000000000000000000" {
		return false
	}
	_, err := hex.DecodeString(s[2:])
	return err == nil
}
