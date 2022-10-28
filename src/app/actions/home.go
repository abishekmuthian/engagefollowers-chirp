package appactions

import (
	"net/http"

	"github.com/abishekmuthian/engagefollowers/src/lib/auth"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/abishekmuthian/engagefollowers/src/lib/view"
	useractions "github.com/abishekmuthian/engagefollowers/src/users/actions"

	"github.com/abishekmuthian/engagefollowers/src/lib/stats"
)

// HandleHome displays the home page
// responds to GET /
func HandleHome(w http.ResponseWriter, r *http.Request) error {
	stats.RegisterHit(r)

	currentUser := session.CurrentUser(w, r)

	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	// Render the template
	view := view.NewRenderer(w, r)
	view.AddKey("home", 1)

	view.AddKey("meta_title", config.Get("meta_title"))
	view.AddKey("meta_url", config.Get("meta_url"))
	view.AddKey("meta_image", config.Get("meta_image"))
	view.AddKey("meta_title", config.Get("meta_title"))
	view.AddKey("meta_desc", config.Get("meta_desc"))
	view.AddKey("meta_keywords", config.Get("meta_keywords"))
	view.AddKey("meta_twitter", config.Get("meta_twitter"))

	view.Template("app/views/home.html.got")

	view.AddKey("currentUser", currentUser)

	view.AddKey("error", params.Get("error"))
	view.AddKey("notice", params.Get("notice"))
	view.AddKey("show_reset_password", params.Get("show_reset_password"))
	view.AddKey("email", params.Get("email"))

	view.AddKey("userCount", stats.UserCount())

	view.AddKey("itemId", params.Get("itemId"))

	if currentUser.Anon() {
		view.AddKey("loggedIn", false)
	} else {
		view.AddKey("loggedIn", true)
		view.AddKey("clientID", config.Get("client_Id"))
		view.AddKey("redirectURI", config.Get("twitter_redirect_uri"))
		view.AddKey("twitterScopes", config.Get("twitter_scopes"))

		nonceToken, err := auth.NonceToken(w, r)

		if err == nil {
			view.AddKey("code", nonceToken)
		}

		// Oauth1.0a flow for banner image update
		// TODO: Implement only If the user hasn't already authenticated oauth1.0

		if !currentUser.TwitterOauthConnected {
			oauthToken, err := useractions.GenerateRequestToken(w, r)

			if err != nil {
				// Don't show Twitter dynamic banner flow
			} else {
				view.AddKey("oauthToken", oauthToken.OauthToken)
			}
		}

	}

	//view.AddKey("validationDeadline", math.Round(time.Date(2021, time.June, 30, 0, 0, 0, 0, time.UTC).Sub(time.Now()).Hours()/24))

	if currentUser.Anon() {
		view.AddKey("disableSubmit", false)
		view.AddKey("disableCopy", true)
	} else {
		view.AddKey("disableSubmit", true)
		view.AddKey("disableCopy", false)

		clientCountry := r.Header.Get("CF-IPCountry")
		log.Info(log.V{"Subscription, Client Country": clientCountry})
		if !config.Production() {
			// There will be no CF request header in the development/test
			clientCountry = config.Get("subscription_client_country")
		}

		if clientCountry == "IN" {
			view.AddKey("priceId", config.Get("stripe_price_id_ideator_IN"))
			view.AddKey("price", config.Get("stripe_price_IN"))
		} else if clientCountry == "GB" {
			view.AddKey("priceId", config.Get("stripe_price_id_ideator_GB"))
			view.AddKey("price", config.Get("stripe_price_GB"))
		} else if clientCountry == "CA" {
			view.AddKey("priceId", config.Get("stripe_price_id_ideator_CA"))
			view.AddKey("price", config.Get("stripe_price_CA"))
		} else if clientCountry == "AU" {
			view.AddKey("priceId", config.Get("stripe_price_id_ideator_AU"))
			view.AddKey("price", config.Get("stripe_price_AU"))
		} else if clientCountry == "DE" || clientCountry == "FR" {
			view.AddKey("priceId", config.Get("stripe_price_id_ideator_EU"))
			view.AddKey("price", config.Get("stripe_price_EU"))
		} else {
			view.AddKey("priceId", config.Get("stripe_price_id_ideator_US"))
			view.AddKey("price", config.Get("stripe_price_US"))
		}
	}

	// Test profile banner update

	// Read the entire file into a byte slice
	// bytes, err := ioutil.ReadFile("./public/assets/images/app/wordcloud.png")
	// if err != nil {
	// 	log.Error(log.V{"Update profile banner": err})
	// }

	// var base64Encoding string
	// /*
	// 			// Prepend the appropriate URI scheme header depending
	// 			// on the MIME type
	// 	        // Mime type results in media error on Twitter
	// 			// Determine the content type of the image file
	// 			    mimeType := http.DetectContentType(bytes)
	// 			    switch mimeType {
	// 			   	case "image/jpeg":
	// 			   		base64Encoding += "data:image/jpeg;base64,"
	// 			   	case "image/png":
	// 			   		base64Encoding += "data:image/png;base64,"
	// 			   	} */

	// // Append the base64 encoded output
	// base64Encoding = base64.StdEncoding.EncodeToString(bytes)

	// useractions.UpdateProfileBanner(currentUser, base64Encoding)

	return view.Render()
}
