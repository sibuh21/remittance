package database

import (
	"database/sql"
	"fmt"
	"log"

	"remittance-service/internal/domain"

	_ "github.com/lib/pq"
)

type DB struct {
	Conn *sql.DB
}

func NewConnection(cfg domain.Config) (*DB, error) {
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

	return &DB{Conn: db}, nil
}

func (db *DB) InitializeSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS transactions (
		id UUID PRIMARY KEY,
		remittance_id VARCHAR(50) UNIQUE NOT NULL,
		status VARCHAR(50) NOT NULL,
		
		sender_name VARCHAR(255) NOT NULL,
		sender_email VARCHAR(255),
		source_amount VARCHAR(50) NOT NULL,
		source_currency VARCHAR(10) NOT NULL,
		cybersource_ref VARCHAR(100),
		collection_status VARCHAR(50),
		
		exchange_rate DECIMAL(18, 8) DEFAULT 0,
		target_amount VARCHAR(50),
		target_currency VARCHAR(10),
		
		receiver_name VARCHAR(255) NOT NULL,
		receiver_phone VARCHAR(50),
		payout_type VARCHAR(50) NOT NULL,
		account_number VARCHAR(100),
		bank_id VARCHAR(50),
		boa_reference VARCHAR(100),
		payout_status VARCHAR(50),
		
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sender_cards (
		id UUID PRIMARY KEY,
		sender_email VARCHAR(255) NOT NULL,
		token_id VARCHAR(100) UNIQUE NOT NULL,
		card_bin VARCHAR(10),
		card_suffix VARCHAR(10),
		card_brand VARCHAR(50),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_sender_cards_email ON sender_cards(sender_email);
	`
	_, err := db.Conn.Exec(query)
	return err
}

func (db *DB) CreateTransaction(t *domain.Transaction) error {
	query := `
	INSERT INTO transactions (
		id, remittance_id, status, sender_name, sender_email, source_amount, 
		source_currency, exchange_rate, target_amount, target_currency, 
		receiver_name, receiver_phone, payout_type, account_number, bank_id
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := db.Conn.Exec(query, 
		t.ID, t.RemittanceID, t.Status, t.SenderName, t.SenderEmail, t.SourceAmount, 
		t.SourceCurrency, t.ExchangeRate, t.TargetAmount, t.TargetCurrency, 
		t.ReceiverName, t.ReceiverPhone, t.PayoutType, t.AccountNumber, t.BankID)
	return err
}

func (db *DB) UpdateCollectionResult(remittanceID, cybersourceRef, collectionStatus, status string) error {
	query := `
	UPDATE transactions 
	SET cybersource_ref = $2, collection_status = $3, status = $4, updated_at = CURRENT_TIMESTAMP
	WHERE remittance_id = $1
	`
	_, err := db.Conn.Exec(query, remittanceID, cybersourceRef, collectionStatus, status)
	return err
}

func (db *DB) UpdatePayoutResult(remittanceID, boaRef, payoutStatus, status string) error {
	query := `
	UPDATE transactions 
	SET boa_reference = $2, payout_status = $3, status = $4, updated_at = CURRENT_TIMESTAMP
	WHERE remittance_id = $1
	`
	_, err := db.Conn.Exec(query, remittanceID, boaRef, payoutStatus, status)
	return err
}

func (db *DB) GetTransactionByRef(ref string) (*domain.Transaction, error) {
	query := `
	SELECT 
		id, remittance_id, status, sender_name, COALESCE(sender_email, ''),
		source_amount, source_currency, COALESCE(cybersource_ref, ''), COALESCE(collection_status, ''),
		exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
		receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
		COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
		created_at, updated_at 
	FROM transactions 
	WHERE remittance_id = $1 OR cybersource_ref = $1 OR id::text = $1`
	
	row := db.Conn.QueryRow(query, ref)

	var t domain.Transaction
	err := row.Scan(
		&t.ID, &t.RemittanceID, &t.Status, &t.SenderName, &t.SenderEmail, 
		&t.SourceAmount, &t.SourceCurrency, &t.CybersourceRef, &t.CollectionStatus, 
		&t.ExchangeRate, &t.TargetAmount, &t.TargetCurrency, &t.ReceiverName, 
		&t.ReceiverPhone, &t.PayoutType, &t.AccountNumber, &t.BankID, 
		&t.BoAReference, &t.PayoutStatus, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) GetTransactionsBySender(email string, status string) ([]*domain.Transaction, error) {
	query := `
	SELECT 
		id, remittance_id, status, sender_name, COALESCE(sender_email, ''),
		source_amount, source_currency, COALESCE(cybersource_ref, ''), COALESCE(collection_status, ''),
		exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
		receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
		COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
		created_at, updated_at 
	FROM transactions 
	WHERE sender_email = $1`
	
	args := []any{email}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []*domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		err := rows.Scan(
			&t.ID, &t.RemittanceID, &t.Status, &t.SenderName, &t.SenderEmail, 
			&t.SourceAmount, &t.SourceCurrency, &t.CybersourceRef, &t.CollectionStatus, 
			&t.ExchangeRate, &t.TargetAmount, &t.TargetCurrency, &t.ReceiverName, 
			&t.ReceiverPhone, &t.PayoutType, &t.AccountNumber, &t.BankID, 
			&t.BoAReference, &t.PayoutStatus, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		txns = append(txns, &t)
	}
	return txns, nil
}

func (db *DB) GetTransactionsByReceiver(phone string, status string) ([]*domain.Transaction, error) {
	query := `
	SELECT 
		id, remittance_id, status, sender_name, COALESCE(sender_email, ''),
		source_amount, source_currency, COALESCE(cybersource_ref, ''), COALESCE(collection_status, ''),
		exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
		receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
		COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
		created_at, updated_at 
	FROM transactions 
	WHERE receiver_phone = $1`
	
	args := []any{phone}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []*domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		err := rows.Scan(
			&t.ID, &t.RemittanceID, &t.Status, &t.SenderName, &t.SenderEmail, 
			&t.SourceAmount, &t.SourceCurrency, &t.CybersourceRef, &t.CollectionStatus, 
			&t.ExchangeRate, &t.TargetAmount, &t.TargetCurrency, &t.ReceiverName, 
			&t.ReceiverPhone, &t.PayoutType, &t.AccountNumber, &t.BankID, 
			&t.BoAReference, &t.PayoutStatus, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		txns = append(txns, &t)
	}
	return txns, nil
}
func (db *DB) SaveSenderCard(card *domain.SenderCard) error {
	// Check if token already exists to avoid duplicates
	var exists bool
	_ = db.Conn.QueryRow("SELECT EXISTS(SELECT 1 FROM sender_cards WHERE token_id = $1)", card.TokenID).Scan(&exists)
	if exists {
		return nil
	}

	query := `
	INSERT INTO sender_cards (id, sender_email, token_id, card_bin, card_suffix, card_brand, expiration_month, expiration_year)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := db.Conn.Exec(query, card.ID, card.SenderEmail, card.TokenID, card.CardBIN, card.CardSuffix, card.CardBrand, card.ExpirationMonth, card.ExpirationYear)
	return err
}

func (db *DB) GetCardsBySenderEmail(email string) ([]*domain.SenderCard, error) {
	query := `SELECT id, sender_email, token_id, card_bin, card_suffix, card_brand, 
	          COALESCE(expiration_month, '') as expiration_month, COALESCE(expiration_year, '') as expiration_year, created_at 
	          FROM sender_cards WHERE sender_email = $1 ORDER BY created_at DESC`
	rows, err := db.Conn.Query(query, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []*domain.SenderCard
	for rows.Next() {
		var c domain.SenderCard
		err := rows.Scan(&c.ID, &c.SenderEmail, &c.TokenID, &c.CardBIN, &c.CardSuffix, &c.CardBrand, &c.ExpirationMonth, &c.ExpirationYear, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		cards = append(cards, &c)
	}
	return cards, nil
}
