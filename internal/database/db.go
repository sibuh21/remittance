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
	CREATE TABLE IF NOT EXISTS remittances (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		cs_transaction_id VARCHAR(50) NULL,
		cs_authentication_transaction_id VARCHAR(50) NULL,
		status VARCHAR(50) NOT NULL,
		
		sender_name VARCHAR(255) NOT NULL,
		sender_first_name VARCHAR(100),
		sender_last_name VARCHAR(100),
		sender_email VARCHAR(255),
		sender_address VARCHAR(255),
		sender_city VARCHAR(100),
		sender_state VARCHAR(100),
		sender_postal_code VARCHAR(50),
		sender_country VARCHAR(100),
		source_amount VARCHAR(50) NOT NULL,
		source_currency VARCHAR(20) NOT NULL,
		collection_status VARCHAR(50),
		payment_token_id VARCHAR(100),
		transient_token_jwt VARCHAR,
		exchange_rate DECIMAL(18, 8) DEFAULT 0,
		target_amount VARCHAR(50),
		target_currency VARCHAR(20),
		
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
		expiration_month VARCHAR(10),
		expiration_year VARCHAR(10),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_sender_cards_email ON sender_cards(sender_email);
	`
	if _, err := db.Conn.Exec(query); err != nil {
		return err
	}
	return nil
}

func (db *DB) CreateRemittance(t *domain.Remittance) error {
	query := `
	INSERT INTO remittances (
		id, status, sender_name, sender_first_name, sender_last_name, sender_email, 
		sender_address, sender_city, sender_state, sender_postal_code, sender_country,
		source_amount, source_currency, exchange_rate, target_amount, target_currency, 
		receiver_name, receiver_phone, payout_type, account_number, bank_id, payment_token_id, transient_token_jwt
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
	`
	_, err := db.Conn.Exec(query,
		t.ID, t.Status, t.SenderName, t.SenderFirstName, t.SenderLastName, t.SenderEmail,
		t.SenderAddress, t.SenderCity, t.SenderState, t.SenderPostalCode, t.SenderCountry,
		t.SourceAmount, t.SourceCurrency, t.ExchangeRate, t.TargetAmount, t.TargetCurrency,
		t.ReceiverName, t.ReceiverPhone, t.PayoutType, t.AccountNumber, t.BankID, t.PaymentTokenID, t.TransientTokenJWT)
	if err != nil {
		log.Printf("ERROR: Failed to create remittance in DB: %v", err)
	}
	return err
}

func (db *DB) UpdateCollectionResult(ID, csTransactionID, csAuthTransactionID, collectionStatus, status, paymentTokenID, transientTokenJWT string) error {
	query := `
	UPDATE remittances 
	SET cs_transaction_id = $2, cs_authentication_transaction_id = $3, 
	    collection_status = $4, status = $5, payment_token_id = $6,
	    transient_token_jwt = $7, updated_at = CURRENT_TIMESTAMP
	WHERE id = $1
	`
	_, err := db.Conn.Exec(query, ID, csTransactionID, csAuthTransactionID, collectionStatus, status, paymentTokenID, transientTokenJWT)
	if err != nil {
		log.Printf("ERROR: Failed to update collection result in DB for %s: %v", ID, err)
	}
	return err
}

func (db *DB) UpdatePayoutResult(ID, boaRef, payoutStatus, status string) error {
	query := `
	UPDATE remittances 
	SET boa_reference = $2, payout_status = $3, status = $4, updated_at = CURRENT_TIMESTAMP
	WHERE id = $1
	`
	_, err := db.Conn.Exec(query, ID, boaRef, payoutStatus, status)
	return err
}

func (db *DB) GetRemittanceByID(id string) (*domain.Remittance, error) {
	row := db.Conn.QueryRow(`
		SELECT 
			id, COALESCE(cs_transaction_id, ''), COALESCE(cs_authentication_transaction_id, ''), status, 
			sender_name, COALESCE(sender_first_name, ''), COALESCE(sender_last_name, ''), COALESCE(sender_email, ''),
			COALESCE(sender_address, ''), COALESCE(sender_city, ''), COALESCE(sender_state, ''), 
			COALESCE(sender_postal_code, ''), COALESCE(sender_country, ''),
			source_amount, source_currency, COALESCE(collection_status, ''), COALESCE(payment_token_id, ''),
			COALESCE(transient_token_jwt, ''),
			exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
			receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
			COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
			created_at, updated_at 
		FROM remittances 
		WHERE id::text = $1`, id)

	var t domain.Remittance
	err := row.Scan(
		&t.ID, &t.CsTransactionID, &t.CsAuthenticationTransactionID, &t.Status,
		&t.SenderName, &t.SenderFirstName, &t.SenderLastName, &t.SenderEmail,
		&t.SenderAddress, &t.SenderCity, &t.SenderState,
		&t.SenderPostalCode, &t.SenderCountry,
		&t.SourceAmount, &t.SourceCurrency, &t.CollectionStatus, &t.PaymentTokenID, &t.TransientTokenJWT,
		&t.ExchangeRate, &t.TargetAmount, &t.TargetCurrency,
		&t.ReceiverName, &t.ReceiverPhone, &t.PayoutType, &t.AccountNumber,
		&t.BankID, &t.BoAReference, &t.PayoutStatus,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) GetRemittanceByCSAuthenticationID(authID string) (*domain.Remittance, error) {
	query := `
	SELECT 
		id, COALESCE(cs_transaction_id, ''), COALESCE(cs_authentication_transaction_id, ''), status, sender_name, COALESCE(sender_email, ''),
		source_amount, source_currency, COALESCE(collection_status, ''),
		exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
		receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
		COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
		created_at, updated_at 
	FROM remittances 
	WHERE cs_authentication_transaction_id = $1`

	row := db.Conn.QueryRow(query, authID)

	var t domain.Remittance
	err := row.Scan(
		&t.ID, &t.CsTransactionID, &t.CsAuthenticationTransactionID, &t.Status, &t.SenderName, &t.SenderEmail,
		&t.SourceAmount, &t.SourceCurrency, &t.CollectionStatus,
		&t.ExchangeRate, &t.TargetAmount, &t.TargetCurrency, &t.ReceiverName,
		&t.ReceiverPhone, &t.PayoutType, &t.AccountNumber, &t.BankID,
		&t.BoAReference, &t.PayoutStatus, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) GetRemittancesBySender(email string, status string) ([]*domain.Remittance, error) {
	query := `
	SELECT 
		id, COALESCE(cs_transaction_id, ''), COALESCE(cs_authentication_transaction_id, ''), status, sender_name, COALESCE(sender_email, ''),
		source_amount, source_currency, COALESCE(collection_status, ''),
		exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
		receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
		COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
		created_at, updated_at 
	FROM remittances 
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

	var txns []*domain.Remittance
	for rows.Next() {
		var t domain.Remittance
		err := rows.Scan(
			&t.ID, &t.CsTransactionID, &t.CsAuthenticationTransactionID, &t.Status, &t.SenderName, &t.SenderEmail,
			&t.SourceAmount, &t.SourceCurrency, &t.CollectionStatus,
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

func (db *DB) GetRemittancesByReceiver(phone string, status string) ([]*domain.Remittance, error) {
	query := `
	SELECT 
		id, COALESCE(cs_transaction_id, ''), COALESCE(cs_authentication_transaction_id, ''), status, sender_name, COALESCE(sender_email, ''),
		source_amount, source_currency, COALESCE(collection_status, ''),
		exchange_rate, COALESCE(target_amount, ''), COALESCE(target_currency, ''),
		receiver_name, COALESCE(receiver_phone, ''), payout_type, COALESCE(account_number, ''),
		COALESCE(bank_id, ''), COALESCE(boa_reference, ''), COALESCE(payout_status, ''),
		created_at, updated_at 
	FROM remittances 
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

	var txns []*domain.Remittance
	for rows.Next() {
		var t domain.Remittance
		err := rows.Scan(
			&t.ID, &t.CsTransactionID, &t.CsAuthenticationTransactionID, &t.Status, &t.SenderName, &t.SenderEmail,
			&t.SourceAmount, &t.SourceCurrency, &t.CollectionStatus,
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
func (db *DB) GetCardByToken(tokenID string) (*domain.SenderCard, error) {
	query := `
	SELECT id, sender_email, token_id, COALESCE(card_bin, ''), COALESCE(card_suffix, ''), 
	       COALESCE(card_brand, ''), COALESCE(expiration_month, ''), COALESCE(expiration_year, ''), created_at
	FROM sender_cards
	WHERE token_id = $1
	`
	row := db.Conn.QueryRow(query, tokenID)

	var c domain.SenderCard
	err := row.Scan(&c.ID, &c.SenderEmail, &c.TokenID, &c.CardBIN, &c.CardSuffix, &c.CardBrand, &c.ExpirationMonth, &c.ExpirationYear, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
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

func (db *DB) DeleteSenderCard(tokenID string) error {
	_, err := db.Conn.Exec("DELETE FROM sender_cards WHERE token_id = $1", tokenID)
	return err
}

func (db *DB) UpdateSenderCardExpiration(tokenID, month, year string) error {
	_, err := db.Conn.Exec("UPDATE sender_cards SET expiration_month = $1, expiration_year = $2 WHERE token_id = $3", month, year, tokenID)
	return err
}
