package useractions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/abishekmuthian/engagefollowers/src/lib/auth"
	"github.com/abishekmuthian/engagefollowers/src/lib/auth/can"
	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/query"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/lib/session"
	"github.com/michimani/gotwi"
	"github.com/michimani/gotwi/fields"
	"github.com/michimani/gotwi/user/userlookup"
	userType "github.com/michimani/gotwi/user/userlookup/types"
)

// Token stores twitter token
type Token struct {
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	AccessToken      string `json:"access_token"`
	Scope            string `json:"scope"`
	RefreshToken     string `json:"refresh_token"`
	ExpiryTime       time.Time
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// HandleConnect handles the GET of the twitter redirect for users
func HandleConnect(w http.ResponseWriter, r *http.Request) error {

	currentUser := session.CurrentUser(w, r)

	// Authorise
	err := can.Connect(currentUser, session.CurrentUser(w, r))
	if err != nil {
		return server.NotAuthorizedError(err)
	}

	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	oauth_token := params.Get("oauth_token")
	oauth_verifier := params.Get("oauth_verifier")

	state := params.Get("state")
	log.Info(log.V{"Url Param 'state' is": string(state)})

	code := params.Get("code")
	log.Info(log.V{"Url Param 'code' is": string(code)})

	// Oauth 1
	if oauth_token != "" && oauth_verifier != "" {
		log.Info(log.V{"Twitter Connect": "Oauth1.0a callback"})

		if oauth_token == currentUser.TwitterOauthToken {
			apiUrl := "https://api.twitter.com/oauth/access_token"
			data := url.Values{}
			data.Set("oauth_token", oauth_token)
			data.Set("oauth_verifier", oauth_verifier)

			u, err := url.ParseRequestURI(apiUrl)

			if err != nil {
				log.Error(log.V{"Twitter Connect Oauth1.0, Error in parse request URI": err})
				server.InternalError(err)
			}

			urlStr := u.String()

			client := &http.Client{}
			r, _ := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(data.Encode())) // URL-encoded payload
			r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			resp, err := client.Do(r)

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

			oAuth1Token := Oauth1Token{}

			params, err := url.ParseQuery(sb)
			if err != nil {
				log.Error(log.V{"requestToken, Error parsing response": err})
				server.InternalError(err)
			}

			for key, value := range params {
				fmt.Printf("  %v = %v\n", key, value)
				if key == "oauth_token" {
					oAuth1Token.OauthToken = value[0]
				} else if key == "oauth_token_secret" {
					oAuth1Token.OauthTokenSecret = value[0]
				} else if key == "screen_name" {
					oAuth1Token.ScreenName = value[0]
				} else if key == "user_id" {
					oAuth1Token.UserId = value[0]
				}
			}

			if oAuth1Token.UserId != currentUser.TwitterId {
				return server.Redirect(w, r, "/?error=incorrect_twitter_account_banner#profile-banner")
			}

			if oAuth1Token.OauthToken != "" && oAuth1Token.OauthTokenSecret != "" {
				userParams := make(map[string]string)

				userParams["twitter_oauth_token"] = oAuth1Token.OauthToken
				userParams["twitter_oauth_token_secret"] = oAuth1Token.OauthTokenSecret
				userParams["twitter_oauth_connected"] = "True"

				err = currentUser.Update(userParams)
				if err != nil {
					log.Error(log.V{"Error updating twitter oauth1.0a details in user": err})
					server.InternalError(err)
				}
			}

		}

		return server.Redirect(w, r, "/#profile-banner")

	} else if state != "" && code != "" {
		log.Info(log.V{"Twitter Connect": "Oauth2.0 callback"})
		reqDump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Error(log.V{"Error in receiving HTTP get from Twitter": err})
		}

		log.Info(log.V{"REQUEST": string(reqDump)})

		err = auth.CheckAuthenticityToken(state, r)

		if err == nil {

			nonceToken, err := auth.NonceToken(w, r)
			if err == nil {
				encodedClientCreds := base64.StdEncoding.EncodeToString([]byte(config.Get("client_Id") + ":" + config.Get("client_secret")))

				apiUrl := "https://api.twitter.com/2/oauth2/token"
				data := url.Values{}
				data.Set("code", code)
				data.Set("grant_type", "authorization_code")
				data.Set("redirect_uri", config.Get("twitter_redirect_uri"))
				data.Set("code_verifier", nonceToken)

				u, err := url.ParseRequestURI(apiUrl)

				if err == nil {
					urlStr := u.String()

					client := &http.Client{}
					r, _ := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(data.Encode())) // URL-encoded payload
					r.Header.Add("Authorization", "Basic "+encodedClientCreds)
					r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

					resp, err := client.Do(r)

					if err == nil {
						body, err := ioutil.ReadAll(resp.Body)
						if err != nil {
							log.Error(log.V{"Connect, Error in parsing response": err})
						}
						//Convert the body to type string
						sb := string(body)
						log.Info(log.V{"Twitter Response": sb})

						token := Token{}
						json.Unmarshal(body, &token)

						if token.Error != "" {
							log.Error(log.V{"Twitter connect Error": token.Error, "Error description": token.ErrorDescription})
							return server.Redirect(w, r, "/?error=failed_twitter_connect#connect")
						}

						t := time.Now().UTC()

						if token.ExpiresIn == 7200 {
							token.ExpiryTime = t.Add(time.Second * 7200)
						} else {
							// TODO: Implement routine to know if the expiry has been changed
							log.Error(log.V{"Twitter Expiry token": "Token expiry time changed"})
						}

						in := &gotwi.NewClientWithAccessTokenInput{
							AccessToken: token.AccessToken,
						}

						c, err := gotwi.NewClientWithAccessToken(in)
						if err != nil {
							log.Error(log.V{"Error in Gotwi client": err})
							getDetailedError(err)
						}

						p := &userType.GetMeInput{
							Expansions: fields.ExpansionList{
								fields.ExpansionPinnedTweetID,
							},
							UserFields: fields.UserFieldList{
								fields.UserFieldCreatedAt,
							},
							TweetFields: fields.TweetFieldList{
								fields.TweetFieldCreatedAt,
							},
						}

						u, err := userlookup.GetMe(context.Background(), c, p)

						if err == nil {

							log.Info(log.V{"ID": gotwi.StringValue(u.Data.ID)})
							log.Info(log.V{"Name": gotwi.StringValue(u.Data.Name)})
							log.Info(log.V{"Username": gotwi.StringValue(u.Data.Username)})
							log.Info(log.V{"CreatedAt": u.Data.CreatedAt})

							userParams := make(map[string]string)

							userParams["twitter_connected"] = "True"
							userParams["twitter_id"] = gotwi.StringValue(u.Data.ID)
							userParams["twitter_username"] = gotwi.StringValue(u.Data.Username)
							userParams["twitter_name"] = gotwi.StringValue(u.Data.Name)
							userParams["twitter_access_token"] = token.AccessToken
							userParams["twitter_refresh_token"] = token.RefreshToken
							userParams["twitter_token_expiry_time"] = query.TimeString(token.ExpiryTime)

							err = currentUser.Update(userParams)
							if err != nil {
								log.Error(log.V{"Error updating twitter details in user": err})
								return server.InternalError(err)
							}
						} else {
							log.Error(log.V{"Error in Twitter user lookup": err})
							getDetailedError(err)
						}

					} else {
						log.Error(log.V{"Error in Twitter post request": err})
						getDetailedError(err)
					}

				}

			} else {
				log.Error(log.V{"Error fetching Nonce token": nonceToken})
			}
		} else {
			log.Error(log.V{"Error authenticating authenticity token": err})
			return server.Redirect(w, r, "/?error=failed_twitter_connect#connect")
		}

		return server.Redirect(w, r, "/#topics")
	}

	return server.Redirect(w, r, "/")

}

func getDetailedError(err error) int {
	// more error information
	ge := err.(*gotwi.GotwiError)
	if ge.OnAPI {
		log.Error(log.V{"Twitter error title ": ge.Title})
		log.Error(log.V{"Twitter error detail ": ge.Detail})
		log.Error(log.V{"Twitter error type ": ge.Type})
		log.Error(log.V{"Twitter error status ": ge.Status})
		log.Error(log.V{"Twitter error status code ": ge.StatusCode})

		for _, ae := range ge.APIErrors {
			log.Error(log.V{"Twitter API error message": ae.Message})
			log.Error(log.V{"Twitter API error label": ae.Label})
			log.Error(log.V{"Twitter API error parameters": ae.Parameters})
			log.Error(log.V{"Twitter API error code": ae.Code})
			log.Error(log.V{"Twitter API error code detail": ae.Code.Detail()})
		}

		if ge.RateLimitInfo != nil {
			log.Error(log.V{"Twitter Rate Limit info limit": ge.RateLimitInfo.Limit})
			log.Error(log.V{"Twitter Rate Limit info remaining": ge.RateLimitInfo.Remaining})
			log.Error(log.V{"Twitter Rate Limit info reset at": ge.RateLimitInfo.ResetAt})
		}
	}

	return ge.StatusCode
}
