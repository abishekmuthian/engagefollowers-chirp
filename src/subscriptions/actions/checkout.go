package actions

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth/can"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/subscriptions"
	"github.com/stripe/stripe-go/v72"
	stripesession "github.com/stripe/stripe-go/v72/checkout/session"
	"io"
	"net/http"
	"strconv"
)

func HandleCreateCheckoutSession(w http.ResponseWriter, r *http.Request) error {

	// Set your secret key. Remember to switch to your live secret key in production.
	// See your keys here: https://dashboard.stripe.com/account/apikeys
	stripe.Key = config.Get("stripe_secret")

	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil
	}

	var req struct {
		Price string `json:"priceId"`
	}
	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	req.Price = params.Get("priceId")

	/*	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(log.V{"Checkout json.NewDecoder.Decode: %v": err})
		if e, ok := err.(*json.SyntaxError); ok {
			log.Error(log.V{"syntax error at byte offset %d": e.Offset})
		}
		log.Error(log.V{"Response %q": r.Body})
		return nil
	}*/

	// Check the authenticity token
	err = session.CheckAuthenticity(w, r)
	if err != nil {
		return err
	}

	// Authorise
	currentUser := session.CurrentUser(w, r)

	subscription := subscriptions.New()

	err = can.Create(subscription, currentUser)
	if err != nil {
		// FIXME: Redirection to to error page not working
		return server.NotAuthorizedError(err)
	}

	// See https://stripe.com/docs/api/checkout/sessions/create
	// for additional parameters to pass.
	// {CHECKOUT_SESSION_ID} is a string literal; do not change it!
	// the actual Session ID is returned in the query parameter when your customer
	// is redirected to the success page.

	// If the customer has email
	var email *string = nil
	if currentUser.PersonalEmail != "" {
		email = stripe.String(currentUser.PersonalEmail)
	}

	clientCountry := r.Header.Get("CF-IPCountry")
	log.Info(log.V{"Subscription, Client Country": clientCountry})
	if !config.Production() {
		// There will be no CF request header in the development/test
		clientCountry = config.Get("subscription_client_country")
	}

	if clientCountry == "IN" {
		// If India, add tax ID
		params := &stripe.CheckoutSessionParams{
			// Enable if Billing Address is required
			BillingAddressCollection: stripe.String("required"),
			SuccessURL:               stripe.String(config.Get("stripe_callback_domain") + "/payment/success?session_id={CHECKOUT_SESSION_ID}"),
			CancelURL:                stripe.String(config.Get("stripe_callback_domain") + "/payment/cancel"),
			PaymentMethodTypes: stripe.StringSlice([]string{
				"card",
			}),
			Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					Price: stripe.String(req.Price),
					// For metered billing, do not pass quantity
					Quantity: stripe.Int64(1),
					TaxRates: stripe.StringSlice([]string{
						config.Get("stripe_tax_rate_IN"),
					}),
				},
			},
			CustomerEmail: email,
			// For subscription only, Not needed for direct payment
			/*			SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
						DefaultTaxRates: stripe.StringSlice([]string{
							config.Get("stripe_tax_rate_IN"),
						}),
					},*/
		}
		params.AddMetadata("user_id", strconv.FormatInt(currentUser.ID, 10))
		params.AddMetadata("user_name", currentUser.Name)

		if req.Price == config.Get("stripe_price_id_ideator_IN") {
			params.AddMetadata("plan", "Basic")
		}

		s, err := stripesession.New(params)
		if err != nil {
			return server.InternalError(err)
			// Needed when using stripe JS
			/*			w.WriteHeader(http.StatusBadRequest)
						writeJSON(w, nil, err)
						return nil*/
		}
		// Needed when using stripe JS
		/*		writeJSON(w, struct {
					SessionID string `json:"sessionId"`
				}{
					SessionID: s.ID,
				}, nil)*/

		// Then redirect to the URL on the Checkout Session
		http.Redirect(w, r, s.URL, http.StatusSeeOther)
	} else {
		// No Tax ID for rest of the world
		params := &stripe.CheckoutSessionParams{
			BillingAddressCollection: stripe.String("required"),
			SuccessURL:               stripe.String(config.Get("stripe_callback_domain") + "/payment/success?session_id={CHECKOUT_SESSION_ID}"),
			CancelURL:                stripe.String(config.Get("stripe_callback_domain") + "/payment/cancel"),
			PaymentMethodTypes: stripe.StringSlice([]string{
				"card",
			}),
			Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					Price: stripe.String(req.Price),
					// For metered billing, do not pass quantity
					Quantity: stripe.Int64(1),
				},
			},
			CustomerEmail: email,
		}
		params.AddMetadata("user_id", strconv.FormatInt(currentUser.ID, 10))
		params.AddMetadata("user_name", currentUser.Name)

		if req.Price == config.Get("stripe_price_id_ideator_US") ||
			req.Price == config.Get("stripe_price_id_ideator_GB") || req.Price == config.Get("stripe_price_id_ideator_CA") ||
			req.Price == config.Get("stripe_price_id_ideator_EU") || req.Price == config.Get("stripe_price_id_ideator_AU") {
			params.AddMetadata("plan", "Basic")
		}

		s, err := stripesession.New(params)
		if err != nil {
			/*			w.WriteHeader(http.StatusBadRequest)
						writeJSON(w, nil, err)
						return nil*/
			return server.InternalError(err)
		}
		// Needed when using stripe JS
		/*		writeJSON(w, struct {
					SessionID string `json:"sessionId"`
				}{
					SessionID: s.ID,
				}, nil)*/
		// Then redirect to the URL on the Checkout Session
		http.Redirect(w, r, s.URL, http.StatusSeeOther)
	}

	return err
}

type errResp struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, v interface{}, err error) {
	var respVal interface{}
	if err != nil {
		msg := err.Error()
		var serr *stripe.Error
		if errors.As(err, &serr) {
			msg = serr.Msg
		}
		w.WriteHeader(http.StatusBadRequest)
		var e errResp
		e.Error.Message = msg
		respVal = e
	} else {
		respVal = v
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(respVal); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error(log.V{"json.NewEncoder.Encode: %v": err})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		log.Error(log.V{"io.Copy: %v": err})
		return
	}
}
