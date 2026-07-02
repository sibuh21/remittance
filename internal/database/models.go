package database

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Remittance struct {
	ID                            uuid.UUID
	CsTransactionID               sql.NullString
	CsAuthenticationTransactionID sql.NullString
	Status                        string
	SenderCardID                  uuid.NullUUID
	SenderUserID                  string
	SenderCountry                 string
	SenderState                   string
	SenderCity                    string
	SenderAddress                 string
	SenderPostalCode              string
	SourceAmount                  decimal.Decimal
	SourceCurrency                string
	CollectionStatus              sql.NullString
	ExchangeRate                  decimal.NullDecimal
	TargetAmount                  decimal.NullDecimal
	TargetCurrency                sql.NullString
	ReceiverFirstName             string
	ReceiverLastName              string
	ReceiverPhone                 sql.NullString
	ReceiverEmail                 string
	ReceiverCountry               string
	ReceiverState                 string
	ReceiverCity                  string
	ReceiverAddress               string
	ReceiverPostalCode            string
	PayoutType                    string
	AccountNumber                 sql.NullString
	BankID                        sql.NullString
	BoaReference                  sql.NullString
	PayoutStatus                  sql.NullString
	CreatedAt                     sql.NullTime
	UpdatedAt                     sql.NullTime
	DeletedAt                     sql.NullTime
}
type SenderCard struct {
	ID              uuid.UUID
	UserID          string
	TokenID         sql.NullString
	CardBin         sql.NullString
	CardSuffix      sql.NullString
	CardBrand       sql.NullString
	ExpirationMonth sql.NullString
	ExpirationYear  sql.NullString
	CreatedAt       sql.NullTime
	UpdatedAt       sql.NullTime
	DeletedAt       sql.NullTime
}

type User struct {
	ID        uuid.UUID
	FirstName string
	LastName  string
	Email     string
	Phone     string
	CreatedAt sql.NullTime
	UpdatedAt sql.NullTime
	DeletedAt sql.NullTime
}
