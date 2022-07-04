package useractions

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
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

	reqDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Error(log.V{"Error in receiving HTTP get from Twitter": err})
	}

	log.Info(log.V{"REQUEST: ": string(reqDump)})

	state := params.Get("state")
	log.Info(log.V{"Url Param 'state' is: ": string(state)})

	code := params.Get("code")
	log.Info(log.V{"Url Param 'code' is: ": string(code)})

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
						log.Error(log.V{"Error: ": err})
					}
					//Convert the body to type string
					sb := string(body)
					log.Info(log.V{"Twitter Response: ": sb})

					token := Token{}
					json.Unmarshal(body, &token)

					if token.Error != "" {
						log.Error(log.V{"Twitter connect Error: ": token.Error, "Error description: ": token.ErrorDescription})
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

						log.Info(log.V{"ID: ": gotwi.StringValue(u.Data.ID)})
						log.Info(log.V{"Name: ": gotwi.StringValue(u.Data.Name)})
						log.Info(log.V{"Username: ": gotwi.StringValue(u.Data.Username)})
						log.Info(log.V{"CreatedAt: ": u.Data.CreatedAt})

						userParams := make(map[string]string)

						userParams["twitter_connected"] = "True"
						userParams["twitter_id"] = gotwi.StringValue(u.Data.ID)
						userParams["twitter_username"] = gotwi.StringValue(u.Data.Username)
						userParams["twitter_access_token"] = token.AccessToken
						userParams["twitter_refresh_token"] = token.RefreshToken
						userParams["twitter_token_expiry_time"] = query.TimeString(token.ExpiryTime)

						err = currentUser.Update(userParams)
						if err != nil {
							log.Error(log.V{"Error updating twitter details in user": err})
							return server.InternalError(err)
						}

						/*					f := &followType.ListFollowersInput{
												ID:              gotwi.StringValue(u.Data.ID),
												MaxResults:      1000,
												PaginationToken: "",
												Expansions:      nil,
												TweetFields:     nil,
												UserFields: fields.UserFieldList{
													fields.UserFieldID,
												},
											}

											followers, err := follow.ListFollowers(context.Background(), c, f)
											if err != nil {
												log.Println(err)
											}

											listCreateInput := types.CreateInput{
												Name:        "EngageWithFollowers#1",
												Description: gotwi.String("A List containing my followers"),
												Private:     gotwi.Bool(true),
											}

											listCreateOutput, err := managelist.Create(context.Background(), c, &listCreateInput)

											if err == nil {
												listID := listCreateOutput.Data.ID

												log.Printf("followers: %v, listID: %s", followers, listID)

												for _, follower := range followers.Data {

													listCreateInput := &listType.CreateInput{
														ID:     listID,
														UserID: gotwi.StringValue(follower.ID),
													}

													listMemberCreateOutput, err := listmember.Create(context.Background(), c, listCreateInput)

													if err == nil {
														log.Printf("List member added %t", listMemberCreateOutput.Data.IsMember)
													} else {
														log.Println("Error adding member to the list", err)
														getDetailedError(err)
													}

												}
											} else {
												log.Println("Error creating list", err)
												getDetailedError(err)
											}*/

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
			log.Error(log.V{"Twitter API error message: ": ae.Message})
			log.Error(log.V{"Twitter API error label: ": ae.Label})
			log.Error(log.V{"Twitter API error parameters: ": ae.Parameters})
			log.Error(log.V{"Twitter API error code: ": ae.Code})
			log.Error(log.V{"Twitter API error code detail: ": ae.Code.Detail()})
		}

		if ge.RateLimitInfo != nil {
			log.Error(log.V{"Twitter Rate Limit info limit : ": ge.RateLimitInfo.Limit})
			log.Error(log.V{"Twitter Rate Limit info remaining : ": ge.RateLimitInfo.Remaining})
			log.Error(log.V{"Twitter Rate Limit info reset at : ": ge.RateLimitInfo.ResetAt})
		}
	}

	return ge.StatusCode
}
