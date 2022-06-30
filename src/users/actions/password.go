package useractions

import (
	"fmt"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth/can"
	m "github.com/abishekmuthian/engagefollowers/src/lib/mandrill"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/query"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/users"
	"net/http"
	"time"
)

const (
	// ResetLifetime is the maximum time reset tokens are valid for
	ResetLifetime = time.Hour
)

// HandlePasswordResetSend responds to POST /users/password/reset
// by sending a password reset email.
func HandlePasswordResetSend(w http.ResponseWriter, r *http.Request) error {

	// No authorisation required
	// Check the authenticity token
	err := session.CheckAuthenticity(w, r)
	if err != nil {
		return server.NotAuthorizedError(err, "Invalid authenticity token")
	}

	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	// Find the user by email (if not found let them know)
	// Find the user by hex token in the db
	email := params.Get("email")
	user, err := users.FindFirst("email=?", email)
	if err != nil {
		//return server.Redirect(w, r, "/users/password/reset?message=invalid_email")
		//return server.NotAuthorizedError(err, "Invalid email id, please enter the email id used with this account.")
		return server.Redirect(w, r, "/?error=not_a_valid_account#forgot_password")
	}

	// Generate a random token and url for the email
	token := auth.BytesToHex(auth.RandomToken(32))

	// Update the user record with with this token
	userParams := map[string]string{
		"password_reset_token": token,
		"password_reset_at":    query.TimeString(time.Now().UTC()),
	}
	// Direct access to the user columns, bypassing validation
	user.Update(userParams)

	// Generate the url to use in our email
	url := fmt.Sprintf("%s/users/password?token=%s", config.Get("root_url"), token)

	// Send a password reset email out to this user
	// (sendgrid implementation)
	/*emailContext := map[string]interface{}{
		"url":  url,
		"name": user.Name,
	}

	log.Info(log.V{"msg": "sending reset email", "user_email": user.Email, "user_id": user.ID})

	e := mail.New(user.Email)
	e.Subject = "Reset Password"
	e.Template = "users/views/password_reset_mail.html.got"
	err = mail.Send(e, emailContext)
	if err != nil {
		return err
	}*/

	// Mandrill implementation
	client := m.ClientWithKey(config.Get("mandrill_key"))

	message := &m.Message{}
	message.AddRecipient(user.Email, user.Name, "to")
	message.FromEmail = config.Get("password_reset_email")
	message.FromName = config.Get("email_from_name")
	message.Subject = config.Get("email_password_reset_subject")
	//message.HTML = "<h1> Click this url " + url + " to reset the password in your account.</h1>"
	//message.Text = "Click this url " + url + " to reset the password in your account."

	// Global vars
	message.GlobalMergeVars = m.MapToVars(map[string]interface{}{"FNAME": user.Name, "TEXT:LINK": url, "LINK": url})
	templateContent := map[string]string{}

	response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_password_reset_template"), templateContent)
	if err != nil {
		log.Error(log.V{"msg": "Password reset email, error sending password reset email", "error": err})
	} else {
		log.Info(log.V{"msg": "Password reset email, response from the server", "response": response})
	}

	// Tell the user what we have done
	return server.Redirect(w, r, "/?notice=password_reset_sent#forgot_password")
}

// HandlePasswordReset responds to GET /users/password?token=DEADFISH
// by logging the user in, removing the token
// and allowing them to set their password.
func HandlePasswordReset(w http.ResponseWriter, r *http.Request) error {

	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	// Note we have no authenticity check, just a random token to check
	token := params.Get("token")
	if len(token) < 10 || len(token) > 64 {
		return server.NotAuthorizedError(fmt.Errorf("Invalid reset token"), "Invalid Token")
	}

	// Find the user by hex token in the db
	user, err := users.FindFirst("password_reset_token=?", token)
	if err != nil {
		return server.NotAuthorizedError(err)
	}

	// Make sure the reset at time is less expire time
	if time.Since(user.PasswordResetAt) > ResetLifetime {
		return server.NotAuthorizedError(nil, "Token invalid", "Your password reset token has expired. Please request another by clicking Log in, FORGOT in home.")
	}

	// Remove the reset token from this user
	// using direct access, bypassing validation
	user.Update(map[string]string{"password_reset_token": ""})

	// Log in the user and store in the session
	// Now save the user details in a secure cookie, so that we remember the next request
	// Build the session from the secure cookie, or create a new one
	session, err := auth.Session(w, r)
	if err != nil {
		return server.NotAuthorizedError(err)
	}

	// Save login the session cookie
	session.Set(auth.SessionUserKey, fmt.Sprintf("%d", user.ID))
	session.Save(w)

	// Log action
	log.Info(log.V{"msg": "reset password", "user_email": user.Email, "user_id": user.ID})

	// Redirect to the user update page so that they can change their password
	//return server.Redirect(w, r, fmt.Sprintf("/users/%d/update", user.ID))

	// Redirect to change password
	// Tell the user what we have done
	return server.Redirect(w, r, "/?show_reset_password=true#reset_password")
}

// HandlePasswordChange responds to  gets the new password, validates it and updates it in the db
func HandlePasswordChange(w http.ResponseWriter, r *http.Request) error {
	// Fetch the  params
	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	// Find the user
	user, err := users.Find(params.GetInt(users.KeyName))
	if err != nil {
		return server.NotFoundError(err)
	}

	// Check the authenticity token
	err = session.CheckAuthenticity(w, r)
	if err != nil {
		return err
	}

	// Authorise update user
	err = can.Update(user, session.CurrentUser(w, r))
	if err != nil {
		return server.NotAuthorizedError(err)
	}

	// Get the password
	pass := params.Get("password")

	// Password must be at least 8 characters
	if len(pass) < 8 {
		return server.Redirect(w, r, "/?error=not_a_valid_password_reset#reset_password")
	}

	// Set the password hash from the password
	hash, err := auth.HashPassword(pass)
	if err != nil {
		return server.InternalError(err)
	}

	//Set the hashed password in the params
	params.SetString("password_hash", hash)

	// Validate the params, removing any we don't accept
	userParams := user.ValidateParams(params.Map(), users.AllowedParams())

	// Update in database
	err = user.Update(userParams)
	if err != nil {
		return server.InternalError(err)
	}
	//Logout the user
	return HandleLogout(w, r)
}
