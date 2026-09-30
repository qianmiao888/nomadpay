package main

import (
	"context"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nomadpay/internal/nomadpay"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func envInt(key string, fallback uint64) uint64 {
	v, err := strconv.ParseUint(os.Getenv(key), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func main() {
	store, err := nomadpay.OpenStore(env("DATA_FILE", "data/nomadpay.json"))
	if err != nil {
		log.Fatal(err)
	}
	cfg := nomadpay.Config{
		RPCURL:        env("MONAD_RPC_URL", "https://rpc.ankr.com/monad_testnet"),
		ChainID:       envInt("CHAIN_ID", 10143),
		Contract:      strings.ToLower(env("CONTRACT_ADDRESS", "0x0ab5ED99aA3fB5cfF20cF91bCcE50A4856150958")),
		EventTopic:    strings.ToLower(env("EVENT_TOPIC", "0x787fcac3b50ab9534e1ef2e289dfa2a75eb9481de9f9061d4773232074751582")),
		StartBlock:    envInt("START_BLOCK", 66910792),
		Confirmations: envInt("CONFIRMATIONS", 12),
		ExplorerURL:   env("EXPLORER_URL", "https://testnet.monadscan.com"),
	}
	if cfg.Contract != "" {
		if !nomadpay.ValidAddress(cfg.Contract) {
			log.Fatal("invalid CONTRACT_ADDRESS")
		}
		if len(cfg.EventTopic) != 66 || !strings.HasPrefix(cfg.EventTopic, "0x") {
			log.Fatal("set valid EVENT_TOPIC")
		}
		poll := time.Duration(envInt("POLL_INTERVAL_SECONDS", 8)) * time.Second
		if poll < time.Second {
			poll = time.Second
		}
		watcher := nomadpay.NewWatcher(cfg, store, &http.Client{Timeout: 15 * time.Second})
		go func() {
			ticker := time.NewTicker(poll)
			defer ticker.Stop()
			for {
				if err := watcher.Sync(context.Background()); err != nil {
					log.Printf("sync: %v", err)
				}
				<-ticker.C
			}
		}()
	} else {
		log.Print("CONTRACT_ADDRESS unset; invoice API available, payment watcher disabled")
	}
	api := &server{store: store, cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", api.config)
	mux.HandleFunc("POST /api/invoices", api.create)
	mux.HandleFunc("GET /api/invoices", api.list)
	mux.HandleFunc("GET /api/invoices/{id}", api.get)
	mux.Handle("/", http.FileServer(http.Dir("web")))
	addr := ":" + env("PORT", "8080")
	log.Printf("NomadPay listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

type server struct {
	store *nomadpay.Store
	cfg   nomadpay.Config
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func apiError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *server) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"chainId": s.cfg.ChainID, "contractAddress": s.cfg.Contract, "explorerUrl": s.cfg.ExplorerURL, "paymentsEnabled": s.cfg.Contract != ""})
}
func (s *server) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Recipient   string `json:"recipient"`
		AmountMON   string `json:"amountMon"`
		Description string `json:"description"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		apiError(w, 400, "invalid JSON")
		return
	}
	in.Recipient = strings.ToLower(strings.TrimSpace(in.Recipient))
	in.Description = strings.TrimSpace(in.Description)
	if !nomadpay.ValidAddress(in.Recipient) {
		apiError(w, 400, "invalid recipient address")
		return
	}
	if in.Description == "" || len(in.Description) > 140 {
		apiError(w, 400, "description must be 1 to 140 characters")
		return
	}
	amount, ok := new(big.Rat).SetString(strings.TrimSpace(in.AmountMON))
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]{1,18})?$`).MatchString(in.AmountMON) || !ok || amount.Sign() <= 0 {
		apiError(w, 400, "invalid MON amount")
		return
	}
	wei := new(big.Rat).Mul(amount, big.NewRat(1_000_000_000_000_000_000, 1))
	if !wei.IsInt() || wei.Num().BitLen() > 256 {
		apiError(w, 400, "amount must have at most 18 decimal places")
		return
	}
	invoice, err := s.store.Create(in.Recipient, wei.Num().String(), in.AmountMON, in.Description)
	if err != nil {
		apiError(w, 500, "could not save invoice")
		return
	}
	writeJSON(w, 201, invoice)
}
func (s *server) list(w http.ResponseWriter, r *http.Request) {
	recipient := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("recipient")))
	if !nomadpay.ValidAddress(recipient) {
		apiError(w, 400, "valid recipient required")
		return
	}
	writeJSON(w, 200, s.store.List(recipient))
}
func (s *server) get(w http.ResponseWriter, r *http.Request) {
	invoice, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		apiError(w, 404, "invoice not found")
		return
	}
	writeJSON(w, 200, invoice)
}
