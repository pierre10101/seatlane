package domain

// Money is an amount in minor units (cents) and an ISO 4217 currency code.
//
// bridge-en: an amount of money
type Money struct {
	Cents    int64  `json:"cents"`
	Currency string `json:"currency"`
}
