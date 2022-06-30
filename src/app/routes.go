package app

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux/middleware/gzip"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux/middleware/secure"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	paymentactions "github.com/abishekmuthian/engagefollowers/src/payment/actions"
	subscriberactions "github.com/abishekmuthian/engagefollowers/src/subscriptions/actions"
	useractions "github.com/abishekmuthian/engagefollowers/src/users/actions"

	// Resource Actions
	appactions "github.com/abishekmuthian/engagefollowers/src/app/actions"
)

// SetupRoutes creates a new router and adds the routes for this app to it.
func SetupRoutes() *mux.Mux {
	router := mux.New()
	mux.SetDefault(router)

	// Add the home page route
	router.Get("/", appactions.HandleHome)

	// Add user actions

	router.Post("/users/login", useractions.HandleLogin)
	router.Post("/users/create", useractions.HandleCreate)
	router.Post("/users/keyword", useractions.HandleKeyword)
	router.Get("/users/connect", useractions.HandleConnect)
	router.Post("/users/logout", useractions.HandleLogout)
	router.Post("/users/password/reset", useractions.HandlePasswordResetSend)
	router.Get("/users/password", useractions.HandlePasswordReset)
	router.Post("/users/password/change", useractions.HandlePasswordChange)

	// Add the legal page route
	router.Get("/legal", HandleLegal)

	// Add a route to handle static files
	router.Get("/favicon.ico", fileHandler)
	router.Get("/icons/{path:.*}", fileHandler)
	router.Get("/files/{path:.*}", fileHandler)
	router.Get("/assets/{path:.*}", fileHandler)

	// Add subscription routes for Stripe
	router.Post("/subscriptions/create-checkout-session", subscriberactions.HandleCreateCheckoutSession)
	router.Get("/payment/success", paymentactions.HandlePaymentSuccess)
	router.Get("/payment/cancel", paymentactions.HandlePaymentCancel)
	router.Post("/payment/webhook", paymentactions.HandleWebhook)
	router.Post("/subscriptions/manage-billing", subscriberactions.HandleCustomerPortal)

	// Set the default file handler
	router.FileHandler = fileHandler
	router.ErrorHandler = errHandler

	// Add middleware
	router.AddMiddleware(log.Middleware)
	router.AddMiddleware(session.Middleware)
	router.AddMiddleware(gzip.Middleware)
	router.AddMiddleware(secure.Middleware)

	return router
}
