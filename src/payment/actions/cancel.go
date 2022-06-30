package paymentactions

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/lib/stats"
	"github.com/abishekmuthian/engagefollowers/src/lib/view"
	"net/http"
)

// HandlePaymentFailure handles the success routine of the payment
func HandlePaymentCancel(w http.ResponseWriter, r *http.Request) error {
	stats.RegisterHit(r)

	// Authorise
	currentUser := session.CurrentUser(w, r)
	log.Info(log.V{"Payment Cancelled, User ID: ": currentUser.UserID()})

	// Render the template
	view := view.NewRenderer(w, r)
	view.AddKey("currentUser", currentUser)

	return server.Redirect(w, r, "/?notice=payment_failure")
}
