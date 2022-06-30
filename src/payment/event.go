package payment

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/stripe/stripe-go/v72"
	"net/mail"
	"strconv"
	"time"
)

// Stripe event object
type Event struct {
	ID           string `json:"id"`
	Data         Data   `json:"data"`
	Created      Time   `json:"created"`
	Subscription string `json:"subscription"`
}

type Data struct {
	Object Object `json:"object"`
}

type Object struct {
	ID                   string               `json:"id"`
	Amount               float64              `json:"amount"`
	AmountSubTotal       float64              `json:"amount_subtotal"`
	AmountTotal          float64              `json:"amount_total"`
	Currency             string               `json:"currency"`
	Customer             string               `json:"customer"`
	CustomerDetails      CustomerDetails      `json:"customer_details"`
	CustomerEmail        string               `json:"customer_email"`
	Subscription         string               `json:"subscription"`
	MetaData             MetaData             `json:"metadata"`
	Mode                 string               `json:"mode"`
	PaymentStatus        string               `json:"payment_status"`
	TotalDetails         TotalDetails         `json:"total_details"`
	PaymentIntent        string               `json:"payment_intent"`
	PaymentMethodDetails PaymentMethodDetails `json:"payment_method_details"`
	BillingDetails       BillingDetails       `json:"billing_details"`
}

type PaymentMethodDetails struct {
	Card Card `json:"card"`
}

type Card struct {
	Brand   stripe.PaymentMethodCardBrand `json:"brand"`
	Country string                        `json:"country"`
	Last4   string                        `json:"last4"`
}

type CustomerDetails struct {
	Email string `json:"email"`
}

type BillingDetails struct {
	Address Address `json:"address"`
	Email   string  `json:"email"`
	Name    string  `json:"name"`
}

type Address struct {
	City       string `json:"city"`
	Country    string `json:"country"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	PostalCode string `json:"postal_code"`
	State      string `json:"state"`
}

type MetaData struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Plan      string `json:"plan"`
	ProblemID string `json:"problem_id"`
}

type TotalDetails struct {
	AmountDiscount float64 `json:"amount_discount"`
	AmountTax      float64 `json:"amount_tax"`
}

type Time struct {
	Time *time.Time
}

type Email struct {
	Email *mail.Address
}

// UnmarshalJSON returns time.Now() no matter what!
func (t *Time) UnmarshalJSON(b []byte) error {

	i, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		log.Error(log.V{"Webhook UnmarshallJSON": err})
	}
	time := time.Unix(i, 0)
	t.Time = &time

	return nil
}
