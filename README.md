# Remittance Service

A production-grade remittance service that combines **CyberSource REST API** for inbound payment collection (international cards) with **Bank of Abyssinia (BoA)** for outbound payout disbursement (ETB) to Ethiopian recipients.

This version implements **CyberSource Flex Microform** for secure card capture and **3DS 2.x** for advanced fraud protection and AFT compliance.

## Architecture

```
┌─────────────────┐                     ┌─────────────────────┐
│   Sender (Intl) │                     │  Receiver (Ethiopia) │
│  (Flex Microform)│                     │  Bank/Wallet Account │
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
│  │  Service (REST) │    │                          │  │        │
│  │                 │    │  • Validate Beneficiary   │  │        │
│  │  ② Tokenize     │    │  ⑤ Transfer Within BoA   │  │        │
│  │  ③ 3DS Auth     │    │    Transfer Other Bank    │  │        │
│  │  ④ Authorize    │    │    Transfer Wallet        │  │        │
│  │    (AFT)        │    │                          │  │        │
│  └────────┬────────┘    └──────────▲───────────────┘  │        │
│           │                        │                   │        │
│      Result State           On SUCCESS                 │        │
│      Machine                trigger payout             │        │
└───────────────────────────────────────────────────────┘        │
         │                                         │             │
    CyberSource                              Bank of            │
    REST API                                Abyssinia           │
                                             API                │
```

## Key Features
*   **PCI-DSS SAQ A Compliance**: Sensitive card data is tokenized via **Flex Microform** iframes; card details never touch your server.
*   **AFT (Account Funding Transaction)**: Fully compliant with international remittance regulations, sending mandatory sender/recipient data to card schemes.
*   **3DS 2.x Integration**: Native orchestration of Device Data Collection (DDC) and Identity Verification (Challenge) flows.
*   **Multi-Payout Options**: Real-time disbursement via BoA internal transfer, EthSwitch (other banks), or Mobile Wallets (Telebirr/M-Pesa).

## Flow
1.  **Sender initiates remittance** → Application creates a record and generates a `CaptureContext`.
2.  **Card entry** → Frontend uses `CaptureContext` to initialize Flex Microform iframes.
3.  **Tokenization** → Card data is exchanged for a `Transient Token` via CyberSource.
4.  **Payer Auth (3DS)** → System performs background profiling (DDC) and handle challenges if required.
5.  **Authorization** → System calls the REST API with the token and AFT fields to capture funds.
6.  **Disbursement** → On successful authorization, the system triggers the BoA payout leg.

## API Endpoints

### Collection (CyberSource REST & Flex)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/collection/capture-context` | Generate JWT for Flex Microform initialization |
| POST | `/api/collection/pa-setup` | Initiate 3DS 2.x Payer Authentication |
| POST | `/api/collection/authorize` | Execute payment authorization (with AFT) |
| POST | `/api/collection/validate` | Validate 3DS challenge result and authorize |
| POST | `/api/collection/return` | 3DS challenge callback handler |

### Payout (Bank of Abyssinia)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/payout/validate` | Validate beneficiary account/wallet |
| GET | `/api/payout/rate/:currency` | Get exchange rate (USD, EUR, etc.) |
| GET | `/api/payout/banks` | List available banks for other-bank transfer |
| GET | `/api/payout/status/:id` | Check payout transaction status |

## Getting Started

### Prerequisites
*   Go 1.21+
*   Docker & Docker-compose (for PostgreSQL)
*   CyberSource REST API Credentials (MID, Key ID, Shared Secret)

### Setup
```bash
# 1. Environment and DB
docker-compose up -d

# 2. Configuration
cp config/example_config.yaml config/config.yaml
# Update config/config.yaml with your REST API keys

# 3. Start Server
go run cmd/server/main.go
```

## Testing
1.  Visit `http://localhost:8090`.
2.  Fill the form and click "Continue to Payment".
3.  Use card `4111 1111 1111 1111` for a successful test transaction.
4.  Use card `4000 0000 0000 0002` to test the 3DS 2.x challenge flow.

## Project Structure
```
remittance/
├── cmd/server/main.go            # Entrypoint & routing
├── config/                       # YAML Configuration
├── frontend/                     # Vanilla JS + CSS Client
├── internal/
│   ├── cybersource/              # REST API Client & Signature logic
│   ├── boa/                      # Bank of Abyssinia Client
│   ├── service/                  # Business logic (Collection, Payout, Remittance)
│   ├── handler/                  # HTTP Handlers
│   └── domain/                   # Models, Interfaces, Enums
└── docker-compose.yml            # Infrastructure (PostgreSQL)
```

## Security
*   **REST Signature**: All API requests use HMAC-SHA256 HTTP Signature authentication.
*   **3DS 2.x**: Mandatory for all European and high-risk transactions.
*   **Data Minimization**: Cards are not stored; tokens are transient.
