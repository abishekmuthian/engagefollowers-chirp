package useractions

import (
	"fmt"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/lib/status"
	"github.com/abishekmuthian/engagefollowers/src/users"
	"net/http"
)

// HandleLogin responds to POST /users/login
// by setting a cookie on the request with encrypted user data.
// HandleLogin responds to POST /users/login
// by setting a cookie on the request with encrypted user data.
func HandleLogin(w http.ResponseWriter, r *http.Request) error {

	// Check the authenticity token
	err := session.CheckAuthenticity(w, r)
	if err != nil {
		return err
	}

	// Check they're not logged in already if so redirect.
	if !session.CurrentUser(w, r).Anon() {
		return server.Redirect(w, r, "/?warn=already_logged_in")
	}

	// Get the user details from the database
	params, err := mux.Params(r)
	if err != nil {
		return server.NotFoundError(err)
	}

	// Fetch the first user by EMAIL or username
	email := params.Get("email")

	// Get the redirect URL
	redirectURL := params.Get("redirectURL")

	// Find the user with this email
	user, err := users.FindFirst("email=?", email)
	/*	if err != nil {
		// If not found try by user.Name instead, error checked below
		user, err = users.FindFirst("name=?", email)
	}*/

	if err != nil {
		log.Info(log.V{"msg": "login failed", "email": email, "status": http.StatusNotFound})
		return server.Redirect(w, r, "/?error=not_a_valid_login#login")
	}

	// Check password against the stored password
	err = auth.CheckPassword(params.Get("password"), user.PasswordHash)
	if err != nil {
		log.Info(log.V{"msg": "login failed", "error": err, "email": email, "user_id": user.ID, "status": http.StatusUnauthorized})
		return server.Redirect(w, r, "/?error=not_a_valid_login#login")
	}

	// Checking status action
	if user.Status == status.Suspended {
		return server.NotAuthorizedError(nil, "Account suspended", "Your account has been suspended for policy violations. Reach out to support if you think this was a mistake.")
	}

	// Now save the user details in a secure cookie,
	// so that we remember the next request
	session, err := auth.Session(w, r)
	if err != nil {
		log.Info(log.V{"msg": "login failed", "email": email, "user_id": user.ID, "status": http.StatusInternalServerError})
		return server.InternalError(err)
	}

	// Success, log it and set the cookie with user id
	session.Set(auth.SessionUserKey, fmt.Sprintf("%d", user.ID))
	session.Save(w)

	// Log action
	log.Info(log.V{"msg": "login", "user_email": user.Email, "user_name": user.Name, "user_id": user.ID})

	// Redirect - ideally here we'd redirect to their original request path
	if redirectURL == "" {
		if user.TwitterConnected {
			redirectURL = "/" + "#topics"
		} else {
			redirectURL = "/" + "#connect"
		}
	}

	return server.Redirect(w, r, redirectURL)
}
