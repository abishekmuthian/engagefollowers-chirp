package useractions

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/abishekmuthian/engagefollowers/src/lib/auth/can"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	serverconfig "github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/dghubble/oauth1"
)

type Oauth1Token struct {
	OauthToken             string `json:"oauth_token"`
	OauthTokenSecret       string `json:"oauth_token_secret"`
	OauthCallbackConfirmed bool   `json:"oauth_callback_confirmed"`
	UserId                 string `json:"user_id"`
	ScreenName             string `json:"screen_name"`
}

// GenerateRequestToken sends the request to Twitter Oauth1.0a 3-legged flow and returns the temporary tokens
func GenerateRequestToken(w http.ResponseWriter, r *http.Request) (Oauth1Token, error) {

	oAuth1Token := Oauth1Token{}

	currentUser := session.CurrentUser(w, r)

	// Authorise
	err := can.Connect(currentUser, session.CurrentUser(w, r))
	if err != nil {
		return oAuth1Token, server.NotAuthorizedError(err)
	}

	config := oauth1.NewConfig(config.Current.Get("twitter_api_key"), config.Current.Get("twitter_api_key_secret"))
	token := oauth1.NewToken(serverconfig.Current.Get("twitter_access_token"), serverconfig.Current.Get("twitter_access_token_secret"))
	client := config.Client(oauth1.NoContext, token)

	urlEncodedCallbackURL := url.QueryEscape(serverconfig.Get("twitter_redirect_uri"))

	apiUrl := "https://api.twitter.com/oauth/request_token"

	data := url.Values{}
	data.Set("oauth_callback", urlEncodedCallbackURL)

	u, err := url.ParseRequestURI(apiUrl)

	if err != nil {
		log.Error(log.V{"Twitter update banner, Error in parse request URI": err})
		server.InternalError(err)
	}

	urlStr := u.String()

	req, err := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(data.Encode())) // URL-encoded payload

	if err != nil {
		log.Error(log.V{"requestToken, Error creating request": err})
		return oAuth1Token, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)

	if err != nil {
		log.Error(log.V{"Twitter Connect Oauth1.0, Error in getting response": err})
		server.InternalError(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(log.V{"Twitter Connect Oauth1.0, Error in parsing response": err})
		server.InternalError(err)
	}

	//Convert the body to type string
	sb := string(body)
	log.Info(log.V{"Twitter Response": sb})

	params, err := url.ParseQuery(sb)
	if err != nil {
		log.Error(log.V{"requestToken, Error parsing response": err})
		return oAuth1Token, err
	}

	for key, value := range params {
		if key == "oauth_token" {
			oAuth1Token.OauthToken = value[0]
		} else if key == "oauth_token_secret" {
			oAuth1Token.OauthTokenSecret = value[0]
		} else if key == "oauth_callback_confirmed" {
			oAuth1Token.OauthCallbackConfirmed, err = strconv.ParseBool(value[0])

			if err != nil {
				log.Error(log.V{"requestToken, Error parsing response": err})
				return oAuth1Token, err
			}
		}
	}

	if oAuth1Token.OauthCallbackConfirmed {
		userParams := make(map[string]string)

		userParams["twitter_oauth_token"] = oAuth1Token.OauthToken
		userParams["twitter_oauth_token_secret"] = oAuth1Token.OauthTokenSecret

		err = currentUser.Update(userParams)
		if err != nil {
			log.Error(log.V{"Error updating twitter oauth1.0a details in user": err})
			return oAuth1Token, err
		}
	}

	return oAuth1Token, nil
}
