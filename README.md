![NomadPay logo](web/logo-lockup.svg)

# NomadPay

**Create a shareable invoice, pay it on Monad, automatically reconcile the onchain payment, and inspect unusual payment patterns.**

NomadPay is a solo hackathon project for independent workers. It demonstrates a complete payment flow and a Go backend that indexes contract events, survives restarts, and handles duplicate events safely. It targets the **Consumer Products & Payments** track of Monad Metropolis.

## How it works

1. A freelancer creates an invoice with a recipient wallet and MON amount.
2. The app returns a shareable invoice link.
3. A client connects an EVM wallet and pays through `NomadPay.pay`.
4. The contract sends MON directly to the recipient and emits `InvoicePaid`.
5. The Go indexer reads confirmed events in bounded block ranges and matches invoice ID, recipient, and exact amount. It saves payment state and the next block cursor together.
6. An optional data science view learns a robust baseline from a recipient's paid invoices and highlights unusual amounts or reconciliation delays for review.

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
- `internal/nomadpay/insights.go`: unsupervised payment anomaly analysis
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
$env:MONAD_RPC_URL = 'https://rpc.ankr.com/monad_testnet'
$env:DEPLOYER_PRIVATE_KEY = '<testnet-private-key>'
npm run deploy:contract
```

The deploy command prints `CONTRACT_ADDRESS`, `START_BLOCK`, and `EVENT_TOPIC`. Set these before starting the server:

```powershell
$env:CONTRACT_ADDRESS = '<deployed-address>'
$env:START_BLOCK = '<deployment-block>'
$env:EVENT_TOPIC = '<printed-topic>'
$env:MONAD_RPC_URL = 'https://rpc.ankr.com/monad_testnet'
go run ./cmd/server
```

Open `http://localhost:8080`. The server defaults to the deployed Monad Testnet contract listed below; environment variables override those values. The `.env.example` file lists all settings, but the server reads environment variables directly. Never commit `DEPLOYER_PRIVATE_KEY` or a filled `.env` file.

To use a different testnet deployment, override `CONTRACT_ADDRESS`, `START_BLOCK`, and `EVENT_TOPIC` with the values printed by the deployment page.

## Verified testnet deployment

- Contract: [`0x0ab5ED99aA3fB5cfF20cF91bCcE50A4856150958`](https://testnet.monadscan.com/address/0x0ab5ED99aA3fB5cfF20cF91bCcE50A4856150958)
- Deployment block: `66910792`; [deployment transaction](https://testnet.monadscan.com/tx/0x724fca884068f8284c073c21557e4d7e3965eb94a1cdd141d57a5d574522d4a1)
- [End-to-end test payment](https://testnet.monadscan.com/tx/0x4fbb21c39d7b3bdb49e71de8b572d923865b060608cc8928a55382e5c2d305c0): `0.001` test MON sent to the invoice recipient; the Go indexer marked the invoice `paid` after confirmations.
- Chain ID: `10143` (Monad Testnet). Testnet MON has no monetary value.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/invoices` | Create an invoice with `recipient`, `amountMon`, `description` |
| `GET` | `/api/invoices/{id}` | Read one invoice and its payment status |
| `GET` | `/api/invoices?recipient=0x...` | List invoices for a recipient |
| `GET` | `/api/config` | Return public network and contract configuration |
| `GET` | `/api/insights?recipient=0x...` | Analyze a recipient's paid invoice history |
| `GET` | `/api/insights/demo` | Preview the model on explicitly synthetic sample data |

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
- A fresh, empty store starts at the current confirmed block; existing invoices retain their saved sync cursor so restarts do not lose payments.
- RPC failures leave the cursor unchanged so the range is retried on the next poll.

## Data science: payment anomalies

The insights endpoint fits a separate **unsupervised robust baseline** for each recipient using two features: log payment amount and log invoice-to-reconciliation delay. It centers each feature on its median, scales by median absolute deviation, and flags points with a combined standardized distance of at least `3.5`. No labels, model API, or private key are needed. Scores are prompts to inspect an invoice, **not fraud verdicts**.

The model abstains until there are 20 paid invoices for the recipient. Our live test currently has only one payment, so the product correctly returns `insufficient_data`. The “Preview sample data” button uses 25 synthetic invoices, clearly identified as such, to show the scoring behavior. Synthetic records are never added to the user's history.

`paidAt` currently records when the backend reconciles an event, so the delay feature includes RPC or server downtime as well as customer payment time. A future version should store the chain block timestamp and train on a larger set of real invoices before treating scores as operational alerts.

## Current limits

This is a hackathon demo, not a production payment service. It uses one server and a JSON file, has no account authentication, and currently supports native MON only. For production use, add authentication, database transactions, monitoring, stronger reorganization handling, and a security review. A recipient can create invoices without proving wallet ownership, so the UI should be used with a trusted invoice link. The data science preview is not evidence that the model performs well on real payment data.

## Validation

Run `go test ./...` for payment matching, duplicate event handling, restart recovery, and a mocked RPC sync. Compile the contract with `npm run compile:contract`.

## References

- [Monad developer portal](https://developers.monad.xyz/)
- [Monad documentation](https://docs.monad.xyz/)
