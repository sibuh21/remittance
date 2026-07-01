package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"remittance-service/internal/domain"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type Queries struct {
	db *sql.DB
}

func NewConnection(cfg domain.Config) (*Queries, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Connected to PostgreSQL successfully")

	return &Queries{db: db}, nil
}

func (q Queries) InitializeSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	first_name VARCHAR(100) NOT NULL,
	last_name VARCHAR(100) NOT NULL,
	email VARCHAR(100) NOT NULL,
	phone VARCHAR(50) NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

	CREATE TABLE IF NOT EXISTS sender_cards (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id VARCHAR(100) NOT NULL,
	token_id VARCHAR(100) NULL,
	card_bin VARCHAR(10),
	card_suffix VARCHAR(10),
	card_brand VARCHAR(50),
	expiration_month VARCHAR(10),
	expiration_year VARCHAR(10),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_sender_cards_user_id ON sender_cards(user_id);


CREATE TABLE IF NOT EXISTS remittances (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	cs_transaction_id VARCHAR(50) NULL,
	cs_authentication_transaction_id VARCHAR(50) NULL,
	status VARCHAR(50) NOT NULL,

	-- card id with which the transaction was initiated
	sender_card_id UUID REFERENCES sender_cards(id) NULL,
	
	sender_user_id VARCHAR(100) NOT NULL,
	sender_country VARCHAR(100) NOT NULL ,
	sender_state VARCHAR(100) NOT NULL,
	sender_city VARCHAR(100) NOT NULL,
	sender_address VARCHAR(255) NOT NULL,
	sender_postal_code VARCHAR(50) NOT NULL,
	source_amount DECIMAL(18, 8) NOT NULL DEFAULT 0,
	source_currency VARCHAR(20) NOT NULL,
	collection_status VARCHAR(50),
	exchange_rate DECIMAL(18, 8) DEFAULT 0,
	target_amount DECIMAL(18, 8) DEFAULT 0,
	target_currency VARCHAR(20),
	--  receiver part
	receiver_first_name VARCHAR(255) NOT NULL,
	receiver_last_name VARCHAR(255) NOT NULL,
	receiver_phone VARCHAR(50) NULL,
	receiver_email VARCHAR(255) NOT NULL,
	receiver_country VARCHAR(100) NOT NULL,
	receiver_state VARCHAR(100) NOT NULL,
	receiver_city VARCHAR(100) NOT NULL,
	receiver_address VARCHAR(255) NOT NULL,
	receiver_postal_code VARCHAR(50) NOT NULL,

	payout_type VARCHAR(50) NOT NULL,
	account_number VARCHAR(100),
	bank_id VARCHAR(50),
	boa_reference VARCHAR(100),
	payout_status VARCHAR(50),	
	
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_remittances_cs_transaction_id ON remittances(cs_transaction_id);
CREATE INDEX IF NOT EXISTS idx_remittances_sender_card_id ON remittances(sender_card_id);
CREATE INDEX IF NOT EXISTS idx_remittances_status ON remittances(status);
	`
	if _, err := q.db.Exec(query); err != nil {
		return err
	}
	return nil
}

const createUser = `-- name: CreateUser :one
INSERT INTO users (
	first_name,
	last_name,
	email,
	phone
) VALUES (
	$1, $2, $3, $4
) RETURNING id, first_name, last_name, email, phone, created_at, updated_at, deleted_at
`

type CreateUserParams struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(createUser, arg.FirstName, arg.LastName, arg.Email, arg.Phone)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Email,
		&i.Phone,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt)
	return i, err
}

const getUserByID = `-- name: GetUserByID :one
SELECT id, first_name, last_name, email, phone, created_at, updated_at, deleted_at
FROM users
WHERE id = $1
`

type GetUserByIDParams struct {
	ID uuid.UUID
}

func (q *Queries) GetUserByID(ctx context.Context, arg GetUserByIDParams) (User, error) {
	row := q.db.QueryRow(getUserByID, arg.ID)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Email,
		&i.Phone,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt)
	return i, err
}

const getUserByEmail = `-- name: GetUserByEmail :one
SELECT id, first_name, last_name, email, phone, created_at, updated_at, deleted_at FROM users WHERE email = $1 AND deleted_at IS NULL
`

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(getUserByEmail, email)
	var i User
	err := row.Scan(
		&i.ID,
		&i.FirstName,
		&i.LastName,
		&i.Email,
		&i.Phone,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt)
	return i, err
}

const createRemittance = `-- name: CreateRemittance :one
INSERT INTO remittances (
	id, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	status, 
	sender_user_id, 
	sender_address, 
	sender_city, 
	sender_state, 
	sender_postal_code, 
	sender_country,
	source_amount, 
	source_currency, 
	exchange_rate, 
	target_amount, 
	target_currency, 
	receiver_first_name, 
	receiver_last_name, 
	receiver_phone, 
	receiver_email,
	receiver_address, 
	receiver_city, 
	receiver_state, 
	receiver_postal_code, 
	receiver_country,
	payout_type, 
	account_number, 
	bank_id
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27
) RETURNING id, cs_transaction_id, cs_authentication_transaction_id, status, sender_card_id, sender_user_id, sender_country, sender_state, sender_city, sender_address, sender_postal_code, source_amount, source_currency, collection_status, exchange_rate, target_amount, target_currency, receiver_first_name, receiver_last_name, receiver_phone, receiver_email, receiver_country, receiver_state, receiver_city, receiver_address, receiver_postal_code, payout_type, account_number, bank_id, boa_reference, payout_status, created_at, updated_at, deleted_at
`

type CreateRemittanceParams struct {
	ID                            uuid.UUID
	CsTransactionID               sql.NullString
	CsAuthenticationTransactionID sql.NullString
	Status                        string
	SenderUserID                  string
	SenderAddress                 string
	SenderCity                    string
	SenderState                   string
	SenderPostalCode              string
	SenderCountry                 string
	SourceAmount                  decimal.Decimal
	SourceCurrency                string
	ExchangeRate                  decimal.NullDecimal
	TargetAmount                  decimal.NullDecimal
	TargetCurrency                sql.NullString
	ReceiverFirstName             string
	ReceiverLastName              string
	ReceiverPhone                 sql.NullString
	ReceiverEmail                 string
	ReceiverAddress               string
	ReceiverCity                  string
	ReceiverState                 string
	ReceiverPostalCode            string
	ReceiverCountry               string
	PayoutType                    string
	AccountNumber                 sql.NullString
	BankID                        sql.NullString
}

func (q *Queries) CreateRemittance(ctx context.Context, arg CreateRemittanceParams) (Remittance, error) {
	row := q.db.QueryRow(createRemittance,
		arg.ID,
		arg.CsTransactionID,
		arg.CsAuthenticationTransactionID,
		arg.Status,
		arg.SenderUserID,
		arg.SenderAddress,
		arg.SenderCity,
		arg.SenderState,
		arg.SenderPostalCode,
		arg.SenderCountry,
		arg.SourceAmount,
		arg.SourceCurrency,
		arg.ExchangeRate,
		arg.TargetAmount,
		arg.TargetCurrency,
		arg.ReceiverFirstName,
		arg.ReceiverLastName,
		arg.ReceiverPhone,
		arg.ReceiverEmail,
		arg.ReceiverAddress,
		arg.ReceiverCity,
		arg.ReceiverState,
		arg.ReceiverPostalCode,
		arg.ReceiverCountry,
		arg.PayoutType,
		arg.AccountNumber,
		arg.BankID,
	)
	var i Remittance
	err := row.Scan(
		&i.ID,
		&i.CsTransactionID,
		&i.CsAuthenticationTransactionID,
		&i.Status,
		&i.SenderCardID,
		&i.SenderUserID,
		&i.SenderCountry,
		&i.SenderState,
		&i.SenderCity,
		&i.SenderAddress,
		&i.SenderPostalCode,
		&i.SourceAmount,
		&i.SourceCurrency,
		&i.CollectionStatus,
		&i.ExchangeRate,
		&i.TargetAmount,
		&i.TargetCurrency,
		&i.ReceiverFirstName,
		&i.ReceiverLastName,
		&i.ReceiverPhone,
		&i.ReceiverEmail,
		&i.ReceiverCountry,
		&i.ReceiverState,
		&i.ReceiverCity,
		&i.ReceiverAddress,
		&i.ReceiverPostalCode,
		&i.PayoutType,
		&i.AccountNumber,
		&i.BankID,
		&i.BoaReference,
		&i.PayoutStatus,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const deleteSenderCard = `-- name: DeleteSenderCard :one
UPDATE sender_cards 
SET deleted_at = now()
WHERE token_id = $1 
	AND deleted_at IS NULL
RETURNING id, user_id, token_id, card_bin, card_suffix, card_brand, expiration_month, expiration_year, created_at, updated_at, deleted_at
`

func (q *Queries) DeleteSenderCard(ctx context.Context, tokenID sql.NullString) (SenderCard, error) {
	row := q.db.QueryRow(deleteSenderCard, tokenID)
	var i SenderCard
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.TokenID,
		&i.CardBin,
		&i.CardSuffix,
		&i.CardBrand,
		&i.ExpirationMonth,
		&i.ExpirationYear,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const getCardByID = `-- name: GetCardByID :one
SELECT id, user_id, token_id, card_bin, card_suffix, card_brand, expiration_month, expiration_year, created_at, updated_at, deleted_at from sender_cards WHERE id = $1 AND deleted_at IS NULL
`

func (q *Queries) GetCardByID(ctx context.Context, id uuid.UUID) (SenderCard, error) {
	row := q.db.QueryRow(getCardByID, id)
	var i SenderCard
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.TokenID,
		&i.CardBin,
		&i.CardSuffix,
		&i.CardBrand,
		&i.ExpirationMonth,
		&i.ExpirationYear,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const getCardByToken = `-- name: GetCardByToken :one
SELECT 
	id, 
	user_id, 
	token_id, 
	card_bin, 
	card_suffix, 
	card_brand, 
	COALESCE(expiration_month, '') as expiration_month, 
	COALESCE(expiration_year, '') as expiration_year, 
	created_at 
FROM sender_cards 
WHERE token_id = $1 
	AND deleted_at IS NULL
`

type GetCardByTokenRow struct {
	ID              uuid.UUID
	UserID          string
	TokenID         sql.NullString
	CardBin         sql.NullString
	CardSuffix      sql.NullString
	CardBrand       sql.NullString
	ExpirationMonth string
	ExpirationYear  string
	CreatedAt       sql.NullTime
}

func (q *Queries) GetCardByToken(ctx context.Context, tokenID sql.NullString) (GetCardByTokenRow, error) {
	row := q.db.QueryRow(getCardByToken, tokenID)
	var i GetCardByTokenRow
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.TokenID,
		&i.CardBin,
		&i.CardSuffix,
		&i.CardBrand,
		&i.ExpirationMonth,
		&i.ExpirationYear,
		&i.CreatedAt,
	)
	return i, err
}

const getCardsByUserID = `-- name: GetCardsByUserID :many
SELECT 
	id, 
	user_id, 
	token_id, 
	card_bin, 
	card_suffix, 
	card_brand, 
	COALESCE(expiration_month, '') as expiration_month, 
	COALESCE(expiration_year, '') as expiration_year, 
	created_at 
FROM sender_cards 
WHERE user_id = $1 
	AND deleted_at IS NULL
ORDER BY created_at DESC
`

type GetCardsByUserIDRow struct {
	ID              uuid.UUID
	UserID          string
	TokenID         sql.NullString
	CardBin         sql.NullString
	CardSuffix      sql.NullString
	CardBrand       sql.NullString
	ExpirationMonth string
	ExpirationYear  string
	CreatedAt       sql.NullTime
}

func (q *Queries) GetCardsByUserID(ctx context.Context, userID string) ([]GetCardsByUserIDRow, error) {
	rows, err := q.db.Query(getCardsByUserID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetCardsByUserIDRow
	for rows.Next() {
		var i GetCardsByUserIDRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.TokenID,
			&i.CardBin,
			&i.CardSuffix,
			&i.CardBrand,
			&i.ExpirationMonth,
			&i.ExpirationYear,
			&i.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getRemittanceByCSAuthenticationID = `-- name: GetRemittanceByCSAuthenticationID :one
SELECT 
	id, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	status, 
    sender_user_id, 
    sender_address, 
    sender_city, 
    sender_state, 
    sender_postal_code, 
    sender_country,
	source_amount, 
	source_currency, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	COALESCE(collection_status, '') as collection_status,
	exchange_rate, COALESCE(target_amount, '') as target_amount, 
	COALESCE(target_currency, '') as target_currency,
	COALESCE(receiver_first_name, '') as receiver_first_name, 
	COALESCE(receiver_last_name, '') as receiver_last_name, 
	COALESCE(receiver_phone, '') as receiver_phone, 
	COALESCE(receiver_email, '') as receiver_email,
	COALESCE(receiver_address, '') as receiver_address, 
	COALESCE(receiver_city, '') as receiver_city, 
	COALESCE(receiver_state, '') as receiver_state, 
	COALESCE(receiver_postal_code, '') as receiver_postal_code, 
	COALESCE(receiver_country, '') as receiver_country, 
	payout_type, COALESCE(account_number, '') as account_number,
	COALESCE(bank_id, '') as bank_id, COALESCE(boa_reference, '') as boa_reference, 
	COALESCE(payout_status, '') as payout_status,
	created_at, updated_at 
FROM remittances 
WHERE cs_authentication_transaction_id = $1
	AND deleted_at IS NULL
`

type GetRemittanceByCSAuthenticationIDRow struct {
	ID                              uuid.UUID
	CsTransactionID                 sql.NullString
	CsAuthenticationTransactionID   sql.NullString
	Status                          string
	SenderUserID                    string
	SenderAddress                   string
	SenderCity                      string
	SenderState                     string
	SenderPostalCode                string
	SenderCountry                   string
	SourceAmount                    decimal.Decimal
	SourceCurrency                  string
	CsTransactionID_2               sql.NullString
	CsAuthenticationTransactionID_2 sql.NullString
	CollectionStatus                string
	ExchangeRate                    decimal.NullDecimal
	TargetAmount                    decimal.Decimal
	TargetCurrency                  string
	ReceiverFirstName               string
	ReceiverLastName                string
	ReceiverPhone                   string
	ReceiverEmail                   string
	ReceiverAddress                 string
	ReceiverCity                    string
	ReceiverState                   string
	ReceiverPostalCode              string
	ReceiverCountry                 string
	PayoutType                      string
	AccountNumber                   string
	BankID                          string
	BoaReference                    string
	PayoutStatus                    string
	CreatedAt                       sql.NullTime
	UpdatedAt                       sql.NullTime
}

func (q *Queries) GetRemittanceByCSAuthenticationID(ctx context.Context, csAuthenticationTransactionID sql.NullString) (GetRemittanceByCSAuthenticationIDRow, error) {
	row := q.db.QueryRow(getRemittanceByCSAuthenticationID, csAuthenticationTransactionID)
	var i GetRemittanceByCSAuthenticationIDRow
	err := row.Scan(
		&i.ID,
		&i.CsTransactionID,
		&i.CsAuthenticationTransactionID,
		&i.Status,
		&i.SenderUserID,
		&i.SenderAddress,
		&i.SenderCity,
		&i.SenderState,
		&i.SenderPostalCode,
		&i.SenderCountry,
		&i.SourceAmount,
		&i.SourceCurrency,
		&i.CsTransactionID_2,
		&i.CsAuthenticationTransactionID_2,
		&i.CollectionStatus,
		&i.ExchangeRate,
		&i.TargetAmount,
		&i.TargetCurrency,
		&i.ReceiverFirstName,
		&i.ReceiverLastName,
		&i.ReceiverPhone,
		&i.ReceiverEmail,
		&i.ReceiverAddress,
		&i.ReceiverCity,
		&i.ReceiverState,
		&i.ReceiverPostalCode,
		&i.ReceiverCountry,
		&i.PayoutType,
		&i.AccountNumber,
		&i.BankID,
		&i.BoaReference,
		&i.PayoutStatus,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getRemittanceByID = `-- name: GetRemittanceByID :one
SELECT  id, cs_transaction_id, cs_authentication_transaction_id, status, sender_card_id, sender_user_id, sender_country, sender_state, sender_city, sender_address, sender_postal_code, source_amount, source_currency, collection_status, exchange_rate, target_amount, target_currency, receiver_first_name, receiver_last_name, receiver_phone, receiver_email, receiver_country, receiver_state, receiver_city, receiver_address, receiver_postal_code, payout_type, account_number, bank_id, boa_reference, payout_status, created_at, updated_at, deleted_at
FROM remittances 
WHERE id::text = $1
	AND deleted_at IS NULL
`

func (q *Queries) GetRemittanceByID(ctx context.Context, id uuid.UUID) (Remittance, error) {
	row := q.db.QueryRow(getRemittanceByID, id)
	var i Remittance
	err := row.Scan(
		&i.ID,
		&i.CsTransactionID,
		&i.CsAuthenticationTransactionID,
		&i.Status,
		&i.SenderCardID,
		&i.SenderUserID,
		&i.SenderCountry,
		&i.SenderState,
		&i.SenderCity,
		&i.SenderAddress,
		&i.SenderPostalCode,
		&i.SourceAmount,
		&i.SourceCurrency,
		&i.CollectionStatus,
		&i.ExchangeRate,
		&i.TargetAmount,
		&i.TargetCurrency,
		&i.ReceiverFirstName,
		&i.ReceiverLastName,
		&i.ReceiverPhone,
		&i.ReceiverEmail,
		&i.ReceiverCountry,
		&i.ReceiverState,
		&i.ReceiverCity,
		&i.ReceiverAddress,
		&i.ReceiverPostalCode,
		&i.PayoutType,
		&i.AccountNumber,
		&i.BankID,
		&i.BoaReference,
		&i.PayoutStatus,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const getRemittancesByReceiver = `-- name: GetRemittancesByReceiver :many
SELECT 
	id, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	status, 
    sender_user_id, 
    sender_address, 
    sender_city, 
    sender_state, 
    sender_postal_code, 
    sender_country,
	source_amount, 
	source_currency, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	COALESCE(collection_status, '') as collection_status,
	exchange_rate, 
	COALESCE(target_amount, '') as target_amount, 
	COALESCE(target_currency, '') as target_currency,
	COALESCE(receiver_first_name, '') as receiver_first_name, 
	COALESCE(receiver_last_name, '') as receiver_last_name, 
	COALESCE(receiver_phone, '') as receiver_phone, 
	COALESCE(receiver_email, '') as receiver_email,
	COALESCE(receiver_address, '') as receiver_address, 
	COALESCE(receiver_city, '') as receiver_city, 
	COALESCE(receiver_state, '') as receiver_state, 
	COALESCE(receiver_postal_code, '') as receiver_postal_code, 
	COALESCE(receiver_country, '') as receiver_country, 
	payout_type, 
	COALESCE(account_number, '') as account_number,
	COALESCE(bank_id, '') as bank_id, 
	COALESCE(boa_reference, '') as boa_reference, 
	COALESCE(payout_status, '') as payout_status,
	created_at, updated_at 
FROM remittances 
WHERE receiver_phone = $1
	AND deleted_at IS NULL
	AND ($2::text IS NULL OR status = $2::text)
ORDER BY created_at DESC
`

type GetRemittancesByReceiverParams struct {
	ReceiverPhone sql.NullString
	Status        sql.NullString
}

type GetRemittancesByReceiverRow struct {
	ID                              uuid.UUID
	CsTransactionID                 sql.NullString
	CsAuthenticationTransactionID   sql.NullString
	Status                          string
	SenderUserID                    string
	SenderAddress                   string
	SenderCity                      string
	SenderState                     string
	SenderPostalCode                string
	SenderCountry                   string
	SourceAmount                    decimal.Decimal
	SourceCurrency                  string
	CsTransactionID_2               sql.NullString
	CsAuthenticationTransactionID_2 sql.NullString
	CollectionStatus                string
	ExchangeRate                    decimal.NullDecimal
	TargetAmount                    decimal.Decimal
	TargetCurrency                  string
	ReceiverFirstName               string
	ReceiverLastName                string
	ReceiverPhone                   string
	ReceiverEmail                   string
	ReceiverAddress                 string
	ReceiverCity                    string
	ReceiverState                   string
	ReceiverPostalCode              string
	ReceiverCountry                 string
	PayoutType                      string
	AccountNumber                   string
	BankID                          string
	BoaReference                    string
	PayoutStatus                    string
	CreatedAt                       sql.NullTime
	UpdatedAt                       sql.NullTime
}

func (q *Queries) GetRemittancesByReceiver(ctx context.Context, arg GetRemittancesByReceiverParams) ([]GetRemittancesByReceiverRow, error) {
	rows, err := q.db.Query(getRemittancesByReceiver, arg.ReceiverPhone, arg.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetRemittancesByReceiverRow
	for rows.Next() {
		var i GetRemittancesByReceiverRow
		if err := rows.Scan(
			&i.ID,
			&i.CsTransactionID,
			&i.CsAuthenticationTransactionID,
			&i.Status,
			&i.SenderUserID,
			&i.SenderAddress,
			&i.SenderCity,
			&i.SenderState,
			&i.SenderPostalCode,
			&i.SenderCountry,
			&i.SourceAmount,
			&i.SourceCurrency,
			&i.CsTransactionID_2,
			&i.CsAuthenticationTransactionID_2,
			&i.CollectionStatus,
			&i.ExchangeRate,
			&i.TargetAmount,
			&i.TargetCurrency,
			&i.ReceiverFirstName,
			&i.ReceiverLastName,
			&i.ReceiverPhone,
			&i.ReceiverEmail,
			&i.ReceiverAddress,
			&i.ReceiverCity,
			&i.ReceiverState,
			&i.ReceiverPostalCode,
			&i.ReceiverCountry,
			&i.PayoutType,
			&i.AccountNumber,
			&i.BankID,
			&i.BoaReference,
			&i.PayoutStatus,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getRemittancesBySender = `-- name: GetRemittancesBySender :many
SELECT 
	id, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	status, 
    sender_user_id, 
    sender_address, 
    sender_city, 
    sender_state, 
    sender_postal_code, 
    sender_country,
	source_amount, 
	source_currency, 
	cs_transaction_id, 
	cs_authentication_transaction_id, 
	COALESCE(collection_status, '') as collection_status,
	exchange_rate, 
	COALESCE(target_amount, '') as target_amount, 
	COALESCE(target_currency, '') as target_currency,
	COALESCE(receiver_first_name, '') as receiver_first_name, 
	COALESCE(receiver_last_name, '') as receiver_last_name, 
	COALESCE(receiver_phone, '') as receiver_phone, 
	COALESCE(receiver_email, '') as receiver_email,
	COALESCE(receiver_address, '') as receiver_address, 
	COALESCE(receiver_city, '') as receiver_city, 
	COALESCE(receiver_state, '') as receiver_state, 
	COALESCE(receiver_postal_code, '') as receiver_postal_code, 
	COALESCE(receiver_country, '') as receiver_country, 
	payout_type, COALESCE(account_number, '') as account_number,
	COALESCE(bank_id, '') as bank_id, COALESCE(boa_reference, '') as boa_reference, 
	COALESCE(payout_status, '') as payout_status,
	created_at, updated_at 
FROM remittances 
WHERE sender_user_id = $1
	AND deleted_at IS NULL
	AND ($2::text IS NULL OR status = $2::text)
ORDER BY created_at DESC
`

type GetRemittancesBySenderParams struct {
	SenderUserID string
	Status       sql.NullString
}

type GetRemittancesBySenderRow struct {
	ID                              uuid.UUID
	CsTransactionID                 sql.NullString
	CsAuthenticationTransactionID   sql.NullString
	Status                          string
	SenderUserID                    string
	SenderAddress                   string
	SenderCity                      string
	SenderState                     string
	SenderPostalCode                string
	SenderCountry                   string
	SourceAmount                    decimal.Decimal
	SourceCurrency                  string
	CsTransactionID_2               sql.NullString
	CsAuthenticationTransactionID_2 sql.NullString
	CollectionStatus                string
	ExchangeRate                    decimal.NullDecimal
	TargetAmount                    decimal.Decimal
	TargetCurrency                  string
	ReceiverFirstName               string
	ReceiverLastName                string
	ReceiverPhone                   string
	ReceiverEmail                   string
	ReceiverAddress                 string
	ReceiverCity                    string
	ReceiverState                   string
	ReceiverPostalCode              string
	ReceiverCountry                 string
	PayoutType                      string
	AccountNumber                   string
	BankID                          string
	BoaReference                    string
	PayoutStatus                    string
	CreatedAt                       sql.NullTime
	UpdatedAt                       sql.NullTime
}

func (q *Queries) GetRemittancesBySender(ctx context.Context, arg GetRemittancesBySenderParams) ([]GetRemittancesBySenderRow, error) {
	rows, err := q.db.Query(getRemittancesBySender, arg.SenderUserID, arg.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetRemittancesBySenderRow
	for rows.Next() {
		var i GetRemittancesBySenderRow
		if err := rows.Scan(
			&i.ID,
			&i.CsTransactionID,
			&i.CsAuthenticationTransactionID,
			&i.Status,
			&i.SenderUserID,
			&i.SenderAddress,
			&i.SenderCity,
			&i.SenderState,
			&i.SenderPostalCode,
			&i.SenderCountry,
			&i.SourceAmount,
			&i.SourceCurrency,
			&i.CsTransactionID_2,
			&i.CsAuthenticationTransactionID_2,
			&i.CollectionStatus,
			&i.ExchangeRate,
			&i.TargetAmount,
			&i.TargetCurrency,
			&i.ReceiverFirstName,
			&i.ReceiverLastName,
			&i.ReceiverPhone,
			&i.ReceiverEmail,
			&i.ReceiverAddress,
			&i.ReceiverCity,
			&i.ReceiverState,
			&i.ReceiverPostalCode,
			&i.ReceiverCountry,
			&i.PayoutType,
			&i.AccountNumber,
			&i.BankID,
			&i.BoaReference,
			&i.PayoutStatus,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const saveSenderCard = `-- name: SaveSenderCard :one
INSERT INTO sender_cards (
	id, 
	user_id, 
	token_id, 
	card_bin, 
	card_suffix, 
	card_brand, 
	expiration_month, 
	expiration_year
) VALUES (	$1, $2, $3, $4, $5, $6, $7, $8) 
RETURNING id, user_id, token_id, card_bin, card_suffix, card_brand, expiration_month, expiration_year, created_at, updated_at, deleted_at
`

type SaveSenderCardParams struct {
	ID              uuid.UUID
	UserID          string
	TokenID         sql.NullString
	CardBin         sql.NullString
	CardSuffix      sql.NullString
	CardBrand       sql.NullString
	ExpirationMonth sql.NullString
	ExpirationYear  sql.NullString
}

func (q *Queries) SaveSenderCard(ctx context.Context, arg SaveSenderCardParams) (SenderCard, error) {
	row := q.db.QueryRow(saveSenderCard,
		arg.ID,
		arg.UserID,
		arg.TokenID,
		arg.CardBin,
		arg.CardSuffix,
		arg.CardBrand,
		arg.ExpirationMonth,
		arg.ExpirationYear,
	)
	var i SenderCard
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.TokenID,
		&i.CardBin,
		&i.CardSuffix,
		&i.CardBrand,
		&i.ExpirationMonth,
		&i.ExpirationYear,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const updateRemittance = `-- name: UpdateRemittance :one
UPDATE remittances 
SET cs_transaction_id = $1, 
	cs_authentication_transaction_id = $2, 
	collection_status = $3, 
	status = $4,
	sender_card_id = $5,
	updated_at = now()
WHERE id = $6
	AND deleted_at IS NULL
RETURNING id, cs_transaction_id, cs_authentication_transaction_id, status, sender_card_id, sender_user_id, sender_country, sender_state, sender_city, sender_address, sender_postal_code, source_amount, source_currency, collection_status, exchange_rate, target_amount, target_currency, receiver_first_name, receiver_last_name, receiver_phone, receiver_email, receiver_country, receiver_state, receiver_city, receiver_address, receiver_postal_code, payout_type, account_number, bank_id, boa_reference, payout_status, created_at, updated_at, deleted_at
`

type UpdateRemittanceParams struct {
	CsTransactionID               sql.NullString
	CsAuthenticationTransactionID sql.NullString
	CollectionStatus              sql.NullString
	Status                        string
	SenderCardID                  sql.NullString
	ID                            uuid.UUID
}

func (q *Queries) UpdateRemittance(ctx context.Context, arg UpdateRemittanceParams) (Remittance, error) {
	row := q.db.QueryRow(updateRemittance,
		arg.CsTransactionID,
		arg.CsAuthenticationTransactionID,
		arg.CollectionStatus,
		arg.Status,
		arg.SenderCardID,
		arg.ID,
	)
	var i Remittance
	err := row.Scan(
		&i.ID,
		&i.CsTransactionID,
		&i.CsAuthenticationTransactionID,
		&i.Status,
		&i.SenderCardID,
		&i.SenderUserID,
		&i.SenderCountry,
		&i.SenderState,
		&i.SenderCity,
		&i.SenderAddress,
		&i.SenderPostalCode,
		&i.SourceAmount,
		&i.SourceCurrency,
		&i.CollectionStatus,
		&i.ExchangeRate,
		&i.TargetAmount,
		&i.TargetCurrency,
		&i.ReceiverFirstName,
		&i.ReceiverLastName,
		&i.ReceiverPhone,
		&i.ReceiverEmail,
		&i.ReceiverCountry,
		&i.ReceiverState,
		&i.ReceiverCity,
		&i.ReceiverAddress,
		&i.ReceiverPostalCode,
		&i.PayoutType,
		&i.AccountNumber,
		&i.BankID,
		&i.BoaReference,
		&i.PayoutStatus,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const updatePayoutResult = `-- name: UpdatePayoutResult :one
UPDATE remittances 
SET boa_reference = $1, 
	payout_status = $2, 
	status = $3, 
	updated_at = now()
WHERE (id::text = $4::text OR cs_transaction_id = $4::text)
	AND deleted_at IS NULL
RETURNING id, cs_transaction_id, cs_authentication_transaction_id, status, sender_card_id, sender_user_id, sender_country, sender_state, sender_city, sender_address, sender_postal_code, source_amount, source_currency, collection_status, exchange_rate, target_amount, target_currency, receiver_first_name, receiver_last_name, receiver_phone, receiver_email, receiver_country, receiver_state, receiver_city, receiver_address, receiver_postal_code, payout_type, account_number, bank_id, boa_reference, payout_status, created_at, updated_at, deleted_at
`

type UpdatePayoutResultParams struct {
	BoaReference sql.NullString
	PayoutStatus sql.NullString
	Status       string
	IDOrRef      string
}

func (q *Queries) UpdatePayoutResult(ctx context.Context, arg UpdatePayoutResultParams) (Remittance, error) {
	row := q.db.QueryRow(updatePayoutResult,
		arg.BoaReference,
		arg.PayoutStatus,
		arg.Status,
		arg.IDOrRef,
	)
	var i Remittance
	err := row.Scan(
		&i.ID,
		&i.CsTransactionID,
		&i.CsAuthenticationTransactionID,
		&i.Status,
		&i.SenderCardID,
		&i.SenderUserID,
		&i.SenderCountry,
		&i.SenderState,
		&i.SenderCity,
		&i.SenderAddress,
		&i.SenderPostalCode,
		&i.SourceAmount,
		&i.SourceCurrency,
		&i.CollectionStatus,
		&i.ExchangeRate,
		&i.TargetAmount,
		&i.TargetCurrency,
		&i.ReceiverFirstName,
		&i.ReceiverLastName,
		&i.ReceiverPhone,
		&i.ReceiverEmail,
		&i.ReceiverCountry,
		&i.ReceiverState,
		&i.ReceiverCity,
		&i.ReceiverAddress,
		&i.ReceiverPostalCode,
		&i.PayoutType,
		&i.AccountNumber,
		&i.BankID,
		&i.BoaReference,
		&i.PayoutStatus,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const updateRemittanceSenderCardID = `-- name: UpdateRemittanceSenderCardID :one
UPDATE remittances 
SET sender_card_id = $1, 
	updated_at = now()
WHERE id = $2
	AND deleted_at IS NULL
RETURNING id, cs_transaction_id, cs_authentication_transaction_id, status, sender_card_id, sender_user_id, sender_country, sender_state, sender_city, sender_address, sender_postal_code, source_amount, source_currency, collection_status, exchange_rate, target_amount, target_currency, receiver_first_name, receiver_last_name, receiver_phone, receiver_email, receiver_country, receiver_state, receiver_city, receiver_address, receiver_postal_code, payout_type, account_number, bank_id, boa_reference, payout_status, created_at, updated_at, deleted_at
`

type UpdateRemittanceSenderCardIDParams struct {
	SenderCardID uuid.NullUUID
	ID           uuid.UUID
}

func (q *Queries) UpdateRemittanceSenderCardID(ctx context.Context, arg UpdateRemittanceSenderCardIDParams) (Remittance, error) {
	row := q.db.QueryRow(updateRemittanceSenderCardID, arg.SenderCardID, arg.ID)
	var i Remittance
	err := row.Scan(
		&i.ID,
		&i.CsTransactionID,
		&i.CsAuthenticationTransactionID,
		&i.Status,
		&i.SenderCardID,
		&i.SenderUserID,
		&i.SenderCountry,
		&i.SenderState,
		&i.SenderCity,
		&i.SenderAddress,
		&i.SenderPostalCode,
		&i.SourceAmount,
		&i.SourceCurrency,
		&i.CollectionStatus,
		&i.ExchangeRate,
		&i.TargetAmount,
		&i.TargetCurrency,
		&i.ReceiverFirstName,
		&i.ReceiverLastName,
		&i.ReceiverPhone,
		&i.ReceiverEmail,
		&i.ReceiverCountry,
		&i.ReceiverState,
		&i.ReceiverCity,
		&i.ReceiverAddress,
		&i.ReceiverPostalCode,
		&i.PayoutType,
		&i.AccountNumber,
		&i.BankID,
		&i.BoaReference,
		&i.PayoutStatus,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}

const updateSenderCardExpiration = `-- name: UpdateSenderCardExpiration :one
UPDATE sender_cards 
SET expiration_month = $1, 
	expiration_year = $2, 
	updated_at = now()
WHERE token_id = $3 
	AND deleted_at IS NULL
RETURNING id, user_id, token_id, card_bin, card_suffix, card_brand, expiration_month, expiration_year, created_at, updated_at, deleted_at
`

type UpdateSenderCardExpirationParams struct {
	ExpirationMonth sql.NullString
	ExpirationYear  sql.NullString
	TokenID         sql.NullString
}

func (q *Queries) UpdateSenderCardExpiration(ctx context.Context, arg UpdateSenderCardExpirationParams) (SenderCard, error) {
	row := q.db.QueryRow(updateSenderCardExpiration, arg.ExpirationMonth, arg.ExpirationYear, arg.TokenID)
	var i SenderCard
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.TokenID,
		&i.CardBin,
		&i.CardSuffix,
		&i.CardBrand,
		&i.ExpirationMonth,
		&i.ExpirationYear,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.DeletedAt,
	)
	return i, err
}
