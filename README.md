# NomadPay

**Create a shareable invoice, pay it on Monad, and automatically reconcile the onchain payment.**

NomadPay is a solo hackathon project for independent workers. It demonstrates a complete payment flow and a Go backend that indexes contract events, survives restarts, and handles duplicate events safely. It targets the **Consumer Products & Payments** track of Monad Metropolis.

## How it works

1. A freelancer creates an invoice with a recipient wallet and MON amount.
2. The app returns a shareable invoice link.
3. A client connects an EVM wallet and pays through `NomadPay.pay`.
4. The contract sends MON directly to the recipient and emits `InvoicePaid`.
5. The Go indexer reads confirmed events in bounded block ranges and matches invoice ID, recipient, and exact amount. It saves payment state and the next block cursor together.

```text
Browser ── invoice API ──> Go server ──> JSON state file
   │                         ▲
   └── wallet transaction ──> Monad contract ──> recipient wallet
                             │
                             └── InvoicePaid event ──> Go indexer
```

The contract never holds customer funds after a successful payment. The server does **not** custody private keys. The demo stores invoices in a local JSON file; anyone with an invoice link can view it.

## Repository layout

- `cmd/server`: HTTP API and static web server
- `internal/nomadpay`: durable invoice store and RPC event indexer
- `contracts/NomadPay.sol`: direct payment contract
- `web`: responsive invoice and wallet UI
- `scripts`: compile and deploy scripts

## Run locally

Requirements: Go 1.23+, Node.js 20+, npm, and an EVM wallet configured for Monad Testnet.

If you do not have a wallet yet, [install MetaMask from its official site](https://metamask.io/) and create a new wallet. Keep its recovery phrase private. Add Monad Testnet using the [Monad Developer Hub](https://monad.xyz/developers) (chain ID `10143`), then request development MON from the [official faucet](https://faucet.monad.xyz/). Testnet MON has no real value.

```powershell
npm install
npm run compile:contract
npm run build:web
go test ./...
```

The easiest way to deploy is with a browser wallet. Start the server with `go run ./cmd/server`, open `http://localhost:8080/deploy.html`, connect a wallet on Monad Testnet with test MON, and approve deployment. The page prints `CONTRACT_ADDRESS`, `START_BLOCK`, and `EVENT_TOPIC` without asking for your private key. Restart the server with those environment variables set.

For command-line deployment from a **testnet-only wallet**, set the key locally:

```powershell
$env:MONAD_RPC_URL = 'https://rpc.testnet.monad.xyz'
$env:DEPLOYER_PRIVATE_KEY = '<testnet-private-key>'
npm run deploy:contract
```

The deploy command prints `CONTRACT_ADDRESS`, `START_BLOCK`, and `EVENT_TOPIC`. Set these before starting the server:

```powershell
$env:CONTRACT_ADDRESS = '<deployed-address>'
$env:START_BLOCK = '<deployment-block>'
$env:EVENT_TOPIC = '<printed-topic>'
$env:MONAD_RPC_URL = 'https://rpc.testnet.monad.xyz'
go run ./cmd/server
```

Open `http://localhost:8080`. The `.env.example` file lists all settings; the server reads environment variables directly. Never commit `DEPLOYER_PRIVATE_KEY` or a filled `.env` file.

If no contract address is set, invoice creation still works locally, but wallet payments are disabled.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/invoices` | Create an invoice with `recipient`, `amountMon`, `description` |
| `GET` | `/api/invoices/{id}` | Read one invoice and its payment status |
| `GET` | `/api/invoices?recipient=0x...` | List invoices for a recipient |
| `GET` | `/api/config` | Return public network and contract configuration |

Example:

```powershell
$body = @{ recipient = '0x1111111111111111111111111111111111111111'; amountMon = '0.01'; description = 'Design work' } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/api/invoices' -ContentType 'application/json' -Body $body
```

## Reliability choices

- The indexer waits for configurable confirmations before considering a payment final.
- It requests at most 50 blocks per `eth_getLogs` call, within Monad public RPC range limits.
- Payment matching checks the contract address, event topic, invoice ID, recipient, and exact amount.
- Transaction hash plus log index provides an idempotency key. The cursor and invoice updates are saved in one atomic file replacement.
- RPC failures leave the cursor unchanged so the range is retried on the next poll.

## Current limits

This is a hackathon demo, not a production payment service. It uses one server and a JSON file, has no account authentication, and currently supports native MON only. For production use, add authentication, database transactions, monitoring, stronger reorganization handling, and a security review. A recipient can create invoices without proving wallet ownership, so the UI should be used with a trusted invoice link.

## Validation

Run `go test ./...` for payment matching, duplicate event handling, restart recovery, and a mocked RPC sync. Compile the contract with `npm run compile:contract`; a testnet deployment and its explorer URL should be added here after deployment.

## References

- [Monad developer portal](https://developers.monad.xyz/)
- [Monad documentation](https://docs.monad.xyz/)
