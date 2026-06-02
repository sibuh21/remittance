# Remittance Service

A production-grade remittance service that combines **CyberSource** for inbound payment collection (international cards) with **Bank of Abyssinia (BoA)** for outbound payout disbursement to Ethiopian recipients.

## Architecture

```
┌─────────────────┐                     ┌─────────────────────┐
│   Sender (Intl) │                     │  Receiver (Ethiopia) │
│   Credit/Debit  │                     │  Bank/Wallet Account │
└────────┬────────┘                     └──────────▲───────────┘
         │                                         │
    ① Initiate                               ⑥ Disbursement
    Remittance                                 (ETB)
         │                                         │
┌────────▼─────────────────────────────────────────┤───────────┐
│                 REMITTANCE SERVICE                │           │
│                                                   │           │
│  ┌────────────────┐    ┌─────────────────────────┤──┐        │
│  │  Collection     │    │  Payout Service          │  │        │
│  │  Service        │    │                          │  │        │
│  │                 │    │  • Validate Beneficiary   │  │        │
│  │  ② CyberSource  │    │  ⑤ Transfer Within BoA   │  │        │
│  │  Hosted Checkout│    │    Transfer Other Bank    │  │        │
│  │  (AFT)          │    │    Transfer Wallet        │  │        │
│  └────────┬────────┘    └──────────▲───────────────┘  │        │
│           │                        │                   │        │
│      ③ Card Payment          ④ On ACCEPT              │        │
│        Result                 trigger payout           │        │
└───────────────────────────────────────────────────────┘        │
         │                                         │             │
    CyberSource                              Bank of            │
    Gateway                                 Abyssinia           │
                                             API                │
```

## Flow

1. **Sender initiates remittance** → `POST /api/remittance`
2. System **validates beneficiary** via BoA name-fetch API
3. System **fetches exchange rate** from BoA
4. System generates **CyberSource signed checkout fields** → redirects sender to pay
5. CyberSource processes card → **webhook** back with result
6. On `ACCEPT` → system **disburses funds** via BoA (within-bank / other-bank / wallet)

## API Endpoints

### Remittance (End-to-End)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/remittance` | Initiate remittance (validate + checkout fields) |
| POST | `/api/remittance/callback` | CyberSource collection callback |

### Collection (CyberSource Inbound)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/checkout` | Get signed fields for CyberSource hosted checkout |
| POST | `/api/response` | CyberSource return URL (customer redirect) |
| POST | `/api/webhook` | CyberSource silent POST notification |

### Payout (Bank of Abyssinia Outbound)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/payout/validate` | Validate beneficiary account/wallet |
| GET | `/api/payout/rate/:currency` | Get exchange rate (USD, EUR, etc.) |
| GET | `/api/payout/banks` | List available banks for other-bank transfer |
| GET | `/api/payout/balance` | Get settlement account balance |
| GET | `/api/payout/status/:id` | Check payout transaction status |

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Service health check |

## Getting Started

### Prerequisites
- Go 1.23+
- CyberSource Secure Acceptance credentials
- Bank of Abyssinia API access (IPSec VPN + API Key + OAuth credentials)

### Setup
```bash
# 1. Copy and configure
cp config/example_config.yaml config/config.yaml
# Edit config/config.yaml with your credentials

# 2. Build
go build -o bin/server ./cmd/server

# 3. Run
./bin/server
```

### Configuration
See `config/example_config.yaml` for all available configuration options.

## Project Structure
```
remittance/
├── cmd/server/main.go            # Application entrypoint
├── config/
│   └── example_config.yaml       # Config template
├── internal/
│   ├── domain/                   # Domain models, interfaces, errors
│   │   ├── models.go             # All domain types
│   │   ├── errors.go             # Structured error types
│   │   └── enums.go              # Status enums & BoA error codes
│   ├── cybersource/              # CyberSource client (HMAC, checkout)
│   │   └── client.go
│   ├── boa/                      # Bank of Abyssinia client (OAuth, transfers)
│   │   └── client.go
│   ├── service/                  # Business logic layer
│   │   ├── collection.go         # Inbound collection service
│   │   ├── payout.go             # Outbound payout service
│   │   └── remittance.go         # End-to-end orchestrator
│   └── handler/                  # HTTP handlers (Echo)
│       ├── collection.go         # CyberSource endpoints
│       ├── payout.go             # BoA payout endpoints
│       └── remittance.go         # Remittance flow endpoints
├── go.mod
└── go.sum
```

## Security
- **CyberSource**: HMAC-SHA256 signature verification on all webhooks
- **BoA**: OAuth 2.0 with automatic token refresh, x-api-key header, TLS + IPSec VPN
- All secrets in `config.yaml` (gitignored)
