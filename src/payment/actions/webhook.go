package paymentactions

import (
	"encoding/json"
	"fmt"
	m "github.com/abishekmuthian/engagefollowers/src/lib/mandrill"
	"github.com/abishekmuthian/engagefollowers/src/lib/query"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/payment"
	"github.com/abishekmuthian/engagefollowers/src/subscriptions"
	"github.com/abishekmuthian/engagefollowers/src/users"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/customer"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/paymentintent"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/sub"
	"github.com/stripe/stripe-go/v72/webhook"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func HandleWebhook(w http.ResponseWriter, r *http.Request) error {

	// Set your secret key. Remember to switch to your live secret key in production.
	// See your keys here: https://dashboard.stripe.com/account/apikeys
	stripe.Key = config.Get("stripe_secret")

	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error(log.V{"ioutil.ReadAll: %v": err})
		return err
	}

	webhookEvent, err := webhook.ConstructEvent(b, r.Header.Get("Stripe-Signature"), config.Get("stripe_webhook_secret"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error(log.V{"webhook.ConstructEvent: ": err})
		return err
	}

	var event payment.Event

	err = json.Unmarshal(b, &event)
	if err != nil {
		log.Error(log.V{"Webhook JSON Unmarshall": err})
	}

	log.Info(log.V{"Webhook event parsed": event})

	switch webhookEvent.Type {
	case "checkout.session.completed":
		// Payment is successful and the subscription is created.
		// You should provision the subscription.
		log.Info(log.V{"Stripe": "Checkout session completed"})

		subscriptionId := event.Data.Object.Subscription

		subscription, err := subscriptions.Find(subscriptionId)
		if err != nil {
			log.Error(log.V{"Webhook, error finding subscription": err})
		}

		if subscription == nil {
			subscription := subscriptions.New()
			err := recordSubscriptionPaymentTransaction(event, subscription)

			if err == nil {
				userID, err := strconv.ParseInt(event.Data.Object.MetaData.UserID, 10, 64)
				if err != nil {
					log.Error(log.V{"Webhook, error converting string user_Id to int64": err})
				} else {
					user, err := users.Find(userID)
					if err != nil {
						log.Error(log.V{"Webhook, error finding user": err})
					} else {
						userParams := make(map[string]string)
						userParams["subscription"] = "true"
						userParams["plan"] = event.Data.Object.MetaData.Plan
						userParams["personal_email"] = event.Data.Object.CustomerDetails.Email

						err = user.Update(userParams)
						if err != nil {
							log.Error(log.V{"webhook user update error": err})
						}
					}

				}
			} else {
				log.Error(log.V{"Webhook, transaction couldn't be recorded": err})
				http.Error(w, "Not ready to accept the webhook, Transaction couldn't be recorded", http.StatusServiceUnavailable)
			}

		} else {
			log.Info(log.V{"Webhook subscription already present in the DB": subscription.ID})
		}

	case "charge.succeeded":
		// Charge suceeded during onetime payment
		log.Info(log.V{"Stripe": "Charge succeeded"})

		var invoice string

		subscription, err := subscriptions.FindPayment(event.Data.Object.PaymentIntent)
		if err != nil {
			log.Error(log.V{"Webhook, error finding subscription": err})
		}

		if subscription != nil {
			if subscription.Invoice == "" {
				user, err := users.Find(subscription.UserId)
				if err != nil {
					log.Error(log.V{"Webhook, error finding use": err})
				} else {

					q := subscriptions.Query()
					q.Where("(invoice <> '') IS TRUE")
					q.Order("updated_at DESC")
					q.Limit(1)

					previousSubscriptions, err := subscriptions.FindAll(q)

					if err == nil && len(previousSubscriptions) > 0 && previousSubscriptions[0].Invoice != "" {
						invoiceNumber, err := strconv.Atoi(strings.TrimPrefix(previousSubscriptions[0].Invoice, config.Get("invoice_prefix")))
						if err != nil {
							log.Error(log.V{"Webhook, error generating invoiceNumber": err})
						} else {
							invoiceNumber = invoiceNumber + 1

							invoice = config.Get("invoice_prefix") + fmt.Sprintf("%04d", invoiceNumber)
						}

					} else {
						invoiceNumber, err := strconv.Atoi(config.Get("invoice_start_sequence"))
						if err != nil {
							log.Error(log.V{"Webhook, error generating invoiceNumber": err})
						} else {
							invoiceNumber = invoiceNumber + 1

							invoice = config.Get("invoice_prefix") + fmt.Sprintf("%04d", invoiceNumber)
						}
					}

					// Send the invoice for one time payment

					fromEmail := config.Get("invoice_email_id")
					fromName := "engage followers"
					subject := "Invoice " + invoice + " for your engage followers subscription"

					// Mandrill implementation
					client := m.ClientWithKey(config.Get("mandrill_key"))
					message := &m.Message{}
					message.FromEmail = fromEmail
					message.FromName = fromName
					message.Subject = subject

					message.AddRecipient(event.Data.Object.BillingDetails.Email, user.Name, "to")

					tm := time.Now()

					loc, err := time.LoadLocation("Asia/Kolkata")
					if err != nil {
						log.Error(log.V{"Webhook, Error loading time location": err})
					}

					year, month, day := tm.In(loc).Date()

					date := strconv.Itoa(day) + "-" + month.String()[:3] + "-" + strconv.Itoa(year)

					// One time
					period := "LIFETIME"

					var paymentMethod string

					var paymentCard string
					if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandVisa {
						paymentCard = "VISA"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandMastercard {
						paymentCard = "MASTERCARD"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandAmex {
						paymentCard = "AMEX"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandDiners {
						paymentCard = "DINERS"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandDiscover {
						paymentCard = "DISCOVER"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandJCB {
						paymentCard = "JCB"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandUnionpay {
						paymentCard = "UNIONPAY"
					} else if event.Data.Object.PaymentMethodDetails.Card.Brand == stripe.PaymentMethodCardBrandUnknown {
						paymentCard = "UNKNOWN"
					}

					paymentMethod = paymentCard + "-" + event.Data.Object.PaymentMethodDetails.Card.Last4

					if event.Data.Object.Currency == "inr" {

						tax := subscription.AmountSubTotal * 9 / 100

						// Global vars
						message.GlobalMergeVars = m.MapToVars(map[string]interface{}{
							"INVOICENO":       invoice,
							"DATE":            date,
							"PM":              paymentMethod,
							"CUSTOMERSTATE":   event.Data.Object.BillingDetails.Address.State,
							"CUSTOMERNAME":    event.Data.Object.BillingDetails.Name,
							"CUSTOMERLINE1":   event.Data.Object.BillingDetails.Address.Line1,
							"CUSTOMERLINE2":   event.Data.Object.BillingDetails.Address.Line2,
							"CUSTOMERCITY":    event.Data.Object.BillingDetails.Address.City,
							"CUSTOMERPINCODE": event.Data.Object.BillingDetails.Address.PostalCode,
							"CUSTOMERCOUNTRY": event.Data.Object.BillingDetails.Address.Country,
							"CUSTOMEREMAIL":   event.Data.Object.BillingDetails.Email,
							"CURRENCY":        "INR",
							"PRICE":           fmt.Sprintf("%.2f", subscription.AmountSubTotal/100),
							"PERIOD":          period,
							"CGST":            fmt.Sprintf("%.2f", tax/100),
							"SGST":            fmt.Sprintf("%.2f", tax/100),
							"SUBTOTAL":        fmt.Sprintf("%.2f", subscription.AmountSubTotal/100),
							"TOTAL":           fmt.Sprintf("%.2f", subscription.AmountTotal/100),
						})
						templateContent := map[string]string{}

						response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_invoice_template_india"), templateContent)
						if err != nil {
							log.Error(log.V{"msg": "Invoice email, error sending invoice email", "error": err})
						} else {
							log.Info(log.V{"msg": "Invoice email, response from the server", "response": response})
						}
					} else {

						// Global vars
						message.GlobalMergeVars = m.MapToVars(map[string]interface{}{
							"INVOICENO":       invoice,
							"DATE":            date,
							"PM":              paymentMethod,
							"CUSTOMERSTATE":   event.Data.Object.BillingDetails.Address.State,
							"CUSTOMERNAME":    event.Data.Object.BillingDetails.Name,
							"CUSTOMERLINE1":   event.Data.Object.BillingDetails.Address.Line1,
							"CUSTOMERLINE2":   event.Data.Object.BillingDetails.Address.Line2,
							"CUSTOMERCITY":    event.Data.Object.BillingDetails.Address.City,
							"CUSTOMERPINCODE": event.Data.Object.BillingDetails.Address.PostalCode,
							"CUSTOMERCOUNTRY": event.Data.Object.BillingDetails.Address.Country,
							"CUSTOMEREMAIL":   event.Data.Object.BillingDetails.Email,
							"CURRENCY":        strings.ToUpper(string(event.Data.Object.Currency)),
							"PERIOD":          period,
							"SUBTOTAL":        fmt.Sprintf("%.2f", subscription.AmountSubTotal/100),
							"TOTAL":           fmt.Sprintf("%.2f", subscription.AmountTotal/100),
						})
						templateContent := map[string]string{}

						response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_invoice_template_outside_india"), templateContent)
						if err != nil {
							log.Error(log.V{"msg": "Invoice email, error sending invoice email", "error": err})
						} else {
							log.Info(log.V{"msg": "Invoice email, response from the server", "response": response})
						}
					}
				}
				recordChargePaymentTransaction(invoice, subscription)

			} else {
				log.Info(log.V{"Webhook, invoice already present": "Success"})
			}
		} else {
			log.Error(log.V{"Webhook, subscription not found for generating invoice": "Failure"})
			http.Error(w, "Not ready to accept the webhook, Subscription not found", http.StatusServiceUnavailable)
		}
	case "payment_method.attached":
		// Payment method attached trying to get address invoice one time payment
		log.Info(log.V{"Stripe": "Payment method attached"})
		params := &stripe.CustomerParams{
			Name: stripe.String(event.Data.Object.BillingDetails.Name),
			Address: &stripe.AddressParams{
				City:       stripe.String(event.Data.Object.BillingDetails.Address.City),
				Country:    stripe.String(event.Data.Object.BillingDetails.Address.Country),
				Line1:      stripe.String(event.Data.Object.BillingDetails.Address.Line1),
				Line2:      stripe.String(event.Data.Object.BillingDetails.Address.Line2),
				PostalCode: stripe.String(event.Data.Object.BillingDetails.Address.PostalCode),
				State:      stripe.String(event.Data.Object.BillingDetails.Address.State),
			},
			// Custom Fields for the Customer
			// Use this with custom flow when using stripe elements
			/*			InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
						Stripe		CustomFields: []*stripe.CustomerInvoiceCustomFieldParams{
									{
										Name:  stripe.String("HSN"),
										Value: stripe.String("9983"),
									},

								},
								Footer: stripe.String("SUPPLY MEANT FOR EXPORT UNDER BOND OR LETTER OF UNDERTAKING WITHOUT PAYMENT OF INTEGRATED TAX"),
							},*/
		}

		c, err := customer.Update(
			event.Data.Object.Customer,
			params,
		)

		if err == nil {
			log.Info(log.V{"Stripe, Updated Customer": c})
		} else {
			log.Error(log.V{"Stripe, Error updating customer": err})
		}
	case "invoice.paid":
		// Continue to provision the subscription as payments continue to be made.
		// Store the status in your database and check when a user accesses your service.
		// This approach helps you avoid hitting rate limits.
		log.Info(log.V{"Stripe": "Invoice paid"})

		//Retrieve invoice
		in, err := invoice.Get(event.Data.Object.ID, nil)

		if err != nil {
			log.Error(log.V{"Webhook": "Error retrieving stripe invoice"})
		} else {
			log.Info(log.V{"Webhook, Invoice retrieved": in})
			// Get the product description and price
			subtotal := in.Subtotal / 100
			total := fmt.Sprintf("%.2f", float64(in.Total)/100)

			//Retrieve customer
			c, err := customer.Get(in.Customer.ID, nil)
			if err != nil {
				log.Error(log.V{"Webhook": "Error retrieving customer"})
			} else {
				log.Info(log.V{"Webhook, Customer retrieved": c})
				//Send invoice to the customer
				fromEmail := config.Get("invoice_email_id")
				fromName := "engagefollowers"
				subject := "Invoice " + in.Number + " for your engagefollowers subscription"

				// Mandrill implementation
				client := m.ClientWithKey(config.Get("mandrill_key"))
				message := &m.Message{}
				message.FromEmail = fromEmail
				message.FromName = fromName
				message.Subject = subject

				message.AddRecipient(in.CustomerEmail, c.Name, "to")

				tm := time.Unix(in.Created, 0)

				loc, err := time.LoadLocation("Asia/Kolkata")
				if err != nil {
					log.Error(log.V{"Webhook, Error loading time location": err})
				}

				year, month, day := tm.In(loc).Date()

				date := strconv.Itoa(day) + "-" + month.String()[:3] + "-" + strconv.Itoa(year)

				s, err := sub.Get(in.Subscription.ID, nil)
				if err != nil {
					log.Error(log.V{"Webhook, Error retrieving subscription": err})
				} else {
					log.Info(log.V{"Webhook, Retrieved subscription": s})
				}

				// Subscription
				/*				startingTime := time.Unix(s.CurrentPeriodStart, 0)
								year, month, day = startingTime.In(loc).Date()
								periodStart := strconv.Itoa(day) + "-" + month.String()[:3] + "-" + strconv.Itoa(year)

								endingTime := time.Unix(s.CurrentPeriodEnd, 0)
								year, month, day = endingTime.In(loc).Date()
								periodEnd := strconv.Itoa(day) + "-" + month.String()[:3] + "-" + strconv.Itoa(year)
								period := periodStart + " - " + periodEnd
				*/

				// One time
				period := "LIFETIME"

				var paymentMethod string

				// Retrieve payment intent
				params := &stripe.PaymentIntentParams{}
				params.AddExpand("payment_method")

				pi, err := paymentintent.Get(
					in.PaymentIntent.ID,
					params,
				)

				if err != nil {
					log.Error(log.V{"Webhook, Error retrieving payment intent": err})
				} else {
					log.Info(log.V{"Webhook, Payment Intent retrieved": pi})
					pm, err := paymentmethod.Get(pi.PaymentMethod.ID, nil)

					if err != nil {
						log.Error(log.V{"Webhook, Error retrieving payment method": err})
					} else {
						log.Info(log.V{"Webhook, Payment Method retrieved": pm})

						var paymentCard string
						if pm.Card.Brand == stripe.PaymentMethodCardBrandVisa {
							paymentCard = "VISA"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandMastercard {
							paymentCard = "MASTERCARD"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandAmex {
							paymentCard = "AMEX"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandDiners {
							paymentCard = "DINERS"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandDiscover {
							paymentCard = "DISCOVER"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandJCB {
							paymentCard = "JCB"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandUnionpay {
							paymentCard = "UNIONPAY"
						} else if pm.Card.Brand == stripe.PaymentMethodCardBrandUnknown {
							paymentCard = "UNKNOWN"
						}

						paymentMethod = paymentCard + "-" + pm.Card.Last4
					}
				}

				if in.Currency == "inr" {

					// Global vars
					message.GlobalMergeVars = m.MapToVars(map[string]interface{}{
						"INVOICENO":       in.Number,
						"DATE":            date,
						"PM":              paymentMethod,
						"CUSTOMERSTATE":   c.Address.State,
						"CUSTOMERNAME":    c.Name,
						"CUSTOMERLINE1":   c.Address.Line1,
						"CUSTOMERLINE2":   c.Address.Line2,
						"CUSTOMERCITY":    c.Address.City,
						"CUSTOMERPINCODE": c.Address.PostalCode,
						"CUSTOMERCOUNTRY": c.Address.Country,
						"CUSTOMEREMAIL":   c.Email,
						"CURRENCY":        "INR",
						"PERIOD":          period,
					})
					templateContent := map[string]string{}

					response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_invoice_template_india"), templateContent)
					if err != nil {
						log.Error(log.V{"msg": "Invoice email, error sending invoice email", "error": err})
					} else {
						log.Info(log.V{"msg": "Invoice email, response from the server", "response": response})
					}
				} else {

					// Global vars
					message.GlobalMergeVars = m.MapToVars(map[string]interface{}{
						"INVOICENO":       in.Number,
						"DATE":            date,
						"PM":              paymentMethod,
						"CUSTOMERSTATE":   c.Address.State,
						"CUSTOMERNAME":    c.Name,
						"CUSTOMERLINE1":   c.Address.Line1,
						"CUSTOMERLINE2":   c.Address.Line2,
						"CUSTOMERCITY":    c.Address.City,
						"CUSTOMERPINCODE": c.Address.PostalCode,
						"CUSTOMERCOUNTRY": c.Address.Country,
						"CUSTOMEREMAIL":   c.Email,
						"CURRENCY":        strings.ToUpper(string(in.Currency)),
						"PERIOD":          period,
						"SUBTOTAL":        subtotal,
						"TOTAL":           total,
					})
					templateContent := map[string]string{}

					response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_invoice_template_outside_india"), templateContent)
					if err != nil {
						log.Error(log.V{"msg": "Invoice email, error sending invoice email", "error": err})
					} else {
						log.Info(log.V{"msg": "Invoice email, response from the server", "response": response})
					}
				}

			}
		}

	case "invoice.payment_failed":
		// The payment failed or the customer does not have a valid payment method.
		// The subscription becomes past_due. Notify your customer and send them to the
		// customer portal to update their payment information.
		log.Info(log.V{"Stripe": "Invoice failed"})
	case "customer.subscription.deleted":
		// Subscription cancelled
		log.Info(log.V{"Stripe": "Subscription cancelled"})
		subscriptionId := event.Data.Object.ID
		subscription, err := subscriptions.Find(subscriptionId)
		if err != nil {
			log.Error(log.V{"Webhook, Error finding subscription": err})
		}

		if subscription == nil {
			log.Error(log.V{"Webhook, customer.subscription.deleted": "Subscription not found"})
		} else {
			user, err := users.Find(subscription.UserId)
			if err != nil {
				log.Error(log.V{"Webhook, error finding use": err})
			} else {
				userParams := make(map[string]string)
				userParams["subscription"] = "false"

				err = user.Update(userParams)
				if err != nil {
					log.Error(log.V{"webhook user update error": err})
				}
			}
		}
	default:
		// unhandled event type
		log.Error(log.V{"Stripe": "Webhook default case"})
	}

	return err
}

// recordSubscriptionPaymentTransaction adds the transaction to database
func recordSubscriptionPaymentTransaction(event payment.Event, subscription *subscriptions.Subscription) error {
	// Params not validated using ValidateParams as user did not create these?
	transactionParams := make(map[string]string)

	transactionParams["txn_id"] = event.Data.Object.PaymentIntent
	transactionParams["payment_date"] = query.TimeString(event.Created.Time.UTC())
	transactionParams["receipt_id"] = event.Data.Object.ID
	transactionParams["mc_gross"] = strconv.FormatFloat(event.Data.Object.AmountSubTotal, 'E', -1, 64)
	transactionParams["payment_gross"] = strconv.FormatFloat(event.Data.Object.AmountTotal, 'E', -1, 64)
	transactionParams["mc_currency"] = event.Data.Object.Currency
	transactionParams["payer_id"] = event.Data.Object.Customer
	transactionParams["payer_email"] = event.Data.Object.CustomerDetails.Email
	transactionParams["txn_type"] = event.Data.Object.Mode
	transactionParams["payment_status"] = event.Data.Object.PaymentStatus
	transactionParams["subscr_id"] = event.Data.Object.Subscription
	transactionParams["tax"] = strconv.FormatFloat(event.Data.Object.TotalDetails.AmountTax, 'E', -1, 64)
	transactionParams["user_id"] = event.Data.Object.MetaData.UserID
	transactionParams["transaction_subject"] = event.Data.Object.MetaData.Plan
	transactionParams["item_name"] = event.Data.Object.MetaData.Plan

	if strings.Contains(event.Data.Object.ID, "cs_test") {
		transactionParams["test_pdt"] = strconv.FormatInt(1, 10)
	}

	dbId, err := subscription.Create(transactionParams)

	if err == nil {
		log.Info(log.V{"Webhook transaction added to db, ID: ": dbId})
	}

	return err
}

// recordChargePaymentTransaction adds the transaction to database
func recordChargePaymentTransaction(invoice string, subscription *subscriptions.Subscription) error {
	// Params not validated using ValidateParams as user did not create these?
	transactionParams := make(map[string]string)

	transactionParams["invoice"] = invoice

	err := subscription.Update(transactionParams)
	if err != nil {
		log.Info(log.V{"Webhook transaction error updating to db": err})
	} else {
		log.Info(log.V{"Webhook charge payment transaction added to db": "success"})
	}

	return err
}
