package actions

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/subscriptions"
	"github.com/stripe/stripe-go/v72"
	portalsession "github.com/stripe/stripe-go/v72/billingportal/session"
	stripesession "github.com/stripe/stripe-go/v72/checkout/session"
	"net/http"
)

func HandleCheckoutSession(w http.ResponseWriter, r *http.Request) error {
	// Set your secret key. Remember to switch to your live secret key in production.
	// See your keys here: https://dashboard.stripe.com/account/apikeys
	stripe.Key = config.Get("stripe_secret")

	if r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil
	}
	sessionID := r.URL.Query().Get("sessionId")
	s, err := stripesession.Get(sessionID, nil)
	writeJSON(w, s, err)
	return err
}

func HandleCustomerPortal(w http.ResponseWriter, r *http.Request) error {
	// Set your secret key. Remember to switch to your live secret key in production.
	// See your keys here: https://dashboard.stripe.com/account/apikeys
	stripe.Key = config.Get("stripe_secret")

	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil
	}

	/*
		var req struct {
			AuthenticityToken string `json:"authenticityToken"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Error(log.V{"Portal json.NewDecoder.Decode: %v": err})
			return nil
		}*/

	// Check the authenticity token
	err := session.CheckAuthenticity(w, r)
	if err != nil {
		return err
	}

	// Authorise update user
	currentUser := session.CurrentUser(w, r)

	// The URL to which the user is redirected when they are done managing
	// billing in the portal.
	returnURL := config.Get("stripe_callback_domain") + "/" + "#user"

	subscription, err := subscriptions.FindCustomerId(currentUser.ID)

	if err == nil {
		params := &stripe.BillingPortalSessionParams{
			Customer:  stripe.String(subscription.CustomerId),
			ReturnURL: stripe.String(returnURL),
		}
		ps, err := portalsession.New(params)

		/*		writeJSON(w, struct {
					URL string `json:"url"`
				}{
					URL: ps.URL,
				}, nil)*/

		if err != nil {
			return server.InternalError(err)
		}

		// Redirect to the URL for the session
		http.Redirect(w, r, ps.URL, http.StatusSeeOther)

	} else {
		log.Error(log.V{"Portal, Error: ": err})
	}

	return err
}
