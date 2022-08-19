package useractions

import (
	"fmt"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth/can"
	"github.com/abishekmuthian/engagefollowers/src/lib/mailchimp"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/lib/status"
	"github.com/abishekmuthian/engagefollowers/src/users"
	"net/http"
	"regexp"
)

// HandleCreate handles the POST of the create form for users
func HandleCreate(w http.ResponseWriter, r *http.Request) error {

	user := users.New()

	// Check the authenticity token
	err := session.CheckAuthenticity(w, r)
	if err != nil {
		return err
	}

	// Authorise
	err = can.Create(user, session.CurrentUser(w, r))
	if err != nil {
		return server.NotAuthorizedError(err)
	}

	// Check they're not logged in already if so redirect.
	if !session.CurrentUser(w, r).Anon() {
		return server.Redirect(w, r, "/?warn=already_logged_in")
	}

	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	// Check a user doesn't exist with this name or email already
	name := params.Get("name")
	email := params.Get("email")
	pass := params.Get("password")
	redirectURL := params.Get("redirectURL")

	// Name must be at least 2 characters
	/*	if len(name) < 2 {
		return server.InternalError(err, "Name too short", "Sorry, names must be at least 2 characters long")
	}*/

	// Name must contain only alphanumeric and underscrore
	re := regexp.MustCompile("^[a-zA-Z ]+$")
	if len(name) > 70 || !(re.MatchString(name)) {
		return server.Redirect(w, r, "/?error=not_a_valid_name_register#register")
	}

	// Email is not optional, so not allowing duplicates
	if email != "" {
		duplicates, err := users.FindAll(users.Where("email=?", email))
		if err != nil {
			return server.InternalError(err)
		}
		if len(duplicates) > 0 {
			return server.Redirect(w, r, "/?error=duplicate_email_register#register")
		}
	} else {
		return server.Redirect(w, r, "/?error=not_a_valid_email_register#register")
	}

	// Password must be at least 8 characters
	if len(pass) < 8 {
		return server.Redirect(w, r, "/?error=not_a_valid_password_register#register")
	}

	// Name is not username so not checking for duplicates
	/*	duplicates, err := users.FindAll(users.Where("name ILIKE ?", name+"%"))
		if err != nil {
			return server.InternalError(err)
		}
		if len(duplicates) > 0 {
			return server.Redirect(w, r, "/users/create?error=duplicate_name&redirecturl="+redirectURL)
		}*/

	// Set the password hash from the password
	hash, err := auth.HashPassword(pass)
	if err != nil {
		return server.InternalError(err)
	}
	params.SetString("password_hash", hash)

	// Validate the params, removing any we don't accept
	userParams := user.ValidateParams(params.Map(), users.AllowedParams())

	// Set some defaults for the new user
	userParams["notification"] = "true"
	userParams["subscription"] = "false"
	userParams["status"] = fmt.Sprintf("%d", status.Published)
	userParams["role"] = fmt.Sprintf("%d", users.Reader)

	id, err := user.Create(userParams)
	if err != nil {
		return server.InternalError(err)
	}

	// Redirect to the new user
	user, err = users.Find(id)
	if err != nil {
		return server.InternalError(err)
	}

	// Log in automatically as the new user they have just created
	session, err := auth.Session(w, r)
	if err != nil {
		log.Info(log.V{"msg": "login failed", "email": user.Email, "user_id": user.ID, "status": http.StatusInternalServerError})
	}

	// Success, log it and set the cookie with user id
	session.Set(auth.SessionUserKey, fmt.Sprintf("%d", user.ID))
	session.Save(w)

	// If email id is available add to the mailchimp list
	if user.Email != "" {
		// Add to the mailchimp list
		audience := mailchimp.Audience{
			MergeFields: mailchimp.Merge{FirstName: user.Name},
			Email:       user.Email,
			Status:      "subscribed",
		}
		go mailchimp.AddToAudience(audience, config.Get("mailchimp_users_list_id"), mailchimp.GetMD5Hash(user.Email), config.Get("mailchimp_token"))
	}

	// Log action
	log.Info(log.V{"msg": "login success", "user_email": user.Email, "user_id": user.ID})

	// Redirect - ideally here we'd redirect to their original request path
	if redirectURL == "" {
		redirectURL = "/"
	}

	return server.Redirect(w, r, redirectURL)
}
