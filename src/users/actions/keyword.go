package useractions

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
)

// HandleKeyword handles the POST to add keywords
func HandleKeyword(w http.ResponseWriter, r *http.Request) error {

	// Check the authenticity token
	err := session.CheckAuthenticity(w, r)
	if err != nil {
		return err
	}

	// Check if logged in
	currentUser := session.CurrentUser(w, r)
	// Check they're not logged in already if so redirect.
	if session.CurrentUser(w, r).Anon() {
		return server.NotAuthorizedError(err)
	}

	// Enable for Trial
	/* 	if !currentUser.TrialEnd.IsZero() && !currentUser.Subscription {
	   		return server.Redirect(w, r, "/?error=not_subscribed")
	   	}
	*/
	// Fetch the params
	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	// Using turnstile to verify users
	if len(params.Get("cf-turnstile-response")) > 0 {
		if string(params.Get("cf-turnstile-response")) != "" {

			type turnstileResponse struct {
				Success      bool     `json:"success"`
				Challenge_ts string   `json:"challenge_ts"`
				Hostname     string   `json:"hostname"`
				ErrorCodes   []string `json:"error-codes"`
				Action       string   `json:"login"`
				Cdata        string   `json:"cdata"`
			}

			var remoteIP string
			var siteVerify turnstileResponse

			if config.Production() {
				// Get the IP from Cloudflare
				remoteIP = r.Header.Get("CF-Connecting-IP")

			} else {
				// Extract the IP from the address
				remoteIP = r.RemoteAddr
				forward := r.Header.Get("X-Forwarded-For")
				if len(forward) > 0 {
					remoteIP = forward
				}
			}

			postBody := url.Values{}
			postBody.Set("secret", config.Get("turnstile_secret_key"))
			postBody.Set("response", string(params.Get("cf-turnstile-response")))
			postBody.Set("remoteip", remoteIP)

			resp, err := http.Post("https://challenges.cloudflare.com/turnstile/v0/siteverify", "application/x-www-form-urlencoded", strings.NewReader(postBody.Encode()))
			if err != nil {
				log.Info(log.V{"Upload, An error occurred while sending the request to the siteverify": err})
				return server.InternalError(err)
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Error(log.V{"Upload, An error occurred while reading the response from the siteverify": err})
				return server.InternalError(err)
			}

			json.Unmarshal(body, &siteVerify)

			if !siteVerify.Success {
				// Security challenge failed
				log.Error(log.V{"Upload, Security challenge failed": siteVerify.ErrorCodes[0]})
				return server.Redirect(w, r, "/?error=security_challenge_failed_topics#topics")
			}
		} else {
			log.Error(log.V{"Upload, Security challenge unable to process": "response not received from user"})
			return server.Redirect(w, r, "/?error=security_challenge_not_completed_topics#topics")
		}
	} else {
		// Security challenge not completed
		return server.Redirect(w, r, "/?error=security_challenge_not_completed_topics#topics")
	}

	keywordsParam := params.Get("keywords")
	// Disabling Auto Like to not violate Twitter guidelines even though its not aggressive
	// autoLikeParam := params.Get("autolike")

	emailParam := params.Get("email")

	// log.Info(log.V{"Auto Like": autoLikeParam})
	log.Info(log.V{"Email": emailParam})

	// Stop the attack
	if len(keywordsParam) > 1000 {
		return server.Redirect(w, r, "/?error=not_a_valid_topic")
	}

	keywordsParam = strings.TrimSpace(keywordsParam)

	var keywords []string

	if len(keywordsParam) == 0 {
		UpdateKeywords(keywords, currentUser.ID)
		return server.Redirect(w, r, "/?notice=topics_removed")
	} else {
		re := regexp.MustCompile("^([\\w\\s,]*[^\\s,])*$")
		if !(re.MatchString(keywordsParam)) {
			return server.Redirect(w, r, "/?error=not_a_valid_topic")
		}
	}

	keywordsTemp := strings.Split(keywordsParam, ",")

	if len(keywordsTemp) > 10 {
		return server.Redirect(w, r, "/?error=not_a_valid_topic")
	}

	for _, keyword := range keywordsTemp {
		if len(keyword) > 1000 {
			return server.Redirect(w, r, "/?error=not_a_valid_topic")
		}
		keywords = append(keywords, strings.TrimSpace(keyword))
	}

	if len(keywords) > 25 {
		return server.InternalError(err)
	}

	log.Info(log.V{"Keywords": keywords})

	// Store the keywords (Topics) in the user database
	UpdateKeywords(keywords, currentUser.ID)

	userParams := make(map[string]string)

	// Disabling Auto Like to not violate Twitter guidelines
	/* 	if len(autoLikeParam) > 0 && autoLikeParam == "True" {
	   		userParams["auto_like"] = autoLikeParam
	   	} else {
	   		userParams["auto_like"] = "False"
	   	} */

	if len(emailParam) > 0 && emailParam == "True" {
		userParams["notification"] = emailParam
	} else {
		userParams["notification"] = "False"
	}

	err = currentUser.Update(userParams)
	if err != nil {
		log.Error(log.V{"Error updating twitter token in user": err})
	}

	return server.Redirect(w, r, "/?notice=topics_added")
}
