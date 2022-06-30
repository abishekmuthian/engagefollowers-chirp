package useractions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	m "github.com/abishekmuthian/engagefollowers/src/lib/mandrill"
	"github.com/abishekmuthian/engagefollowers/src/lib/query"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	userModel "github.com/abishekmuthian/engagefollowers/src/users"
	"github.com/go-redis/redis/v8"
	"github.com/michimani/gotwi"
	"github.com/michimani/gotwi/fields"
	lists "github.com/michimani/gotwi/list/listlookup"
	listLookupType "github.com/michimani/gotwi/list/listlookup/types"
	"github.com/michimani/gotwi/list/listmember"
	listType "github.com/michimani/gotwi/list/listmember/types"
	"github.com/michimani/gotwi/list/listtweetlookup"
	listTweetLookupInput "github.com/michimani/gotwi/list/listtweetlookup/types"
	"github.com/michimani/gotwi/list/managelist"
	manageListType "github.com/michimani/gotwi/list/managelist/types"
	"github.com/michimani/gotwi/tweet/like"
	tweetLikeType "github.com/michimani/gotwi/tweet/like/types"
	"github.com/michimani/gotwi/user/follow"
	followType "github.com/michimani/gotwi/user/follow/types"
	"github.com/michimani/gotwi/user/userlookup"
	userType "github.com/michimani/gotwi/user/userlookup/types"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func getFollowers(c *gotwi.Client, user *userModel.User) {
	var getFollowersFlag bool

	// Get twitter followers
	getFollowersFlag = true
	var followerIDs []string
	var paginationToken string
	for getFollowersFlag == true {
		followers, err := getTwitterFollowers(user, c, paginationToken)
		if err == nil {
			for _, follower := range followers.Data {
				followerIDs = append(followerIDs, gotwi.StringValue(follower.ID))
				if len(followerIDs) >= 15000 {
					getFollowersFlag = false
				}
			}
			if gotwi.StringValue(followers.Meta.NextToken) == "" {
				getFollowersFlag = false
			} else {
				paginationToken = gotwi.StringValue(followers.Meta.NextToken)
			}
		} else {
			log.Error(log.V{"Error retrieving twitter followers": err})
			getFollowersFlag = false
		}
	}

	log.Info(log.V{"Retrieved Followers: ": len(followerIDs)})

	if len(followerIDs) > 0 {
		err := UpdateFollowers(followerIDs, user.ID)

		if err != nil {
			log.Error(log.V{"Error updating followers in DB": err})
		}
	}

}

func createList(c *gotwi.Client, user *userModel.User) {

	listCreateInput := manageListType.CreateInput{
		Name:        config.Get("twitter_list_name"),
		Description: gotwi.String(config.Get("twitter_list_description")),
		Private:     gotwi.Bool(true),
	}

	listCreateOutput, err := managelist.Create(context.Background(), c, &listCreateInput)

	if err == nil {

		listID := listCreateOutput.Data.ID

		log.Info(log.V{"listID: ": listID})

		userParams := make(map[string]string)
		userParams["twitter_list_id"] = listID
		userParams["twitter_list_creation_time"] = query.TimeString(time.Now().UTC())

		err = user.Update(userParams)
		if err != nil {
			log.Error(log.V{"Error updating twitter list ID in user": err})
		} else {
			// Delete existing twitter list
			if user.TwitterListID != "" {
				listDeleteInput := manageListType.DeleteInput{
					ID: user.TwitterListID,
				}

				listDeleteOutput, err := managelist.Delete(context.Background(), c, &listDeleteInput)

				if err == nil {
					log.Info(log.V{"Existing Twitter List deleted: ": listDeleteOutput.Data.Deleted})
				} else {
					log.Error(log.V{"Error deleting existing Twitter list": err})
					getDetailedError(err)
				}
			}
		}

	} else {
		log.Error(log.V{"Error creating list": err})
		getDetailedError(err)
	}

}

func addMembersToList(c *gotwi.Client, user *userModel.User) {

	log.Info(log.V{"User Followers: ": len(user.TwitterFollowers)})

	var limitedFollowers []string
	if len(user.TwitterFollowers) > 100 {
		// Limiting the followers to 100 due to rate limit
		Shuffle(user.TwitterFollowers)
		limitedFollowers = user.TwitterFollowers[:100]
	} else {
		limitedFollowers = user.TwitterFollowers
	}

	if len(limitedFollowers) > 0 {

		listID := user.TwitterListID

		listExists, err := checkIfListExists(listID, c)

		if err == nil && listExists {
			for _, follower := range limitedFollowers {

				listCreateInput := listType.CreateInput{
					ID:     listID,
					UserID: follower,
				}

				listMemberCreateOutput, err := listmember.Create(context.Background(), c, &listCreateInput)

				if err == nil {
					log.Info(log.V{"List member: ": follower, "Added: ": listMemberCreateOutput.Data.IsMember})
				} else {
					log.Info(log.V{"Error adding member to the list: ": follower, "Error": err})
					getDetailedError(err)
				}

			}
		} else {
			log.Error(log.V{"Error in accessing the list for adding members, so continuing: ": err})

		}

	} else {
		log.Error(log.V{"No followers retrieved: ": "Length of limited followers is 0"})
	}

}

func checkIfListExists(listID string, c *gotwi.Client) (bool, error) {

	listLookupInput := listLookupType.GetInput{
		ID:         listID,
		Expansions: nil,
		ListFields: nil,
		UserFields: nil,
	}

	listLookupOutput, err := lists.Get(context.Background(), c, &listLookupInput)

	if err == nil {
		if gotwi.StringValue(listLookupOutput.Data.ID) == listID {
			log.Info(log.V{"List exists: ": listID})
			return true, err
		} else {
			log.Error(log.V{"List does not exist: ": listID})
			return false, err
		}
	} else {
		log.Error(log.V{"Error in List Lookup: ": err})
	}
	return false, err
}

func GetTweetsOfFollowers() {
	// Initialize redis
	var ctx = context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Get("redis_server"),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	q := userModel.Query()

	// User is not suspended
	q.Where("status=100")

	// User should not have trial ended or subscription should be true
	q.Where("trial_end IS NULL OR subscription is TRUE")

	// Fetch the userModel
	users, err := userModel.FindAll(q)
	if err != nil {
		log.Error(log.V{"message": "email: error getting users for checking tweets", "error": err})
		return
	}

	if len(users) > 0 {
		for _, user := range users {
			twitterAccessToken := user.TwitterAccessToken

			if user.TwitterAccessToken == "" {
				askUserToConnectTwitter(rdb, ctx, user)
				continue
			}

			expiryTime := user.TwitterTokenExpiryTime
			currentTime := time.Now().UTC()

			elapsedTime := currentTime.Sub(expiryTime).Hours()

			if elapsedTime >= 0 && user.TwitterConnected {
				log.Info(log.V{"Twitter Access Token: ": "expired"})

				token, err := getTwitterAccessToken(user.TwitterRefreshToken)
				twitterAccessToken = token.AccessToken

				if err == nil && token.AccessToken != "" {
					t := time.Now().UTC()

					if token.ExpiresIn == 7200 {
						token.ExpiryTime = t.Add(time.Second * 7200)
					} else {
						// TODO: Implement routine to know if the expiry has been changed
						log.Error(log.V{"Twitter Expiry token": "Token expiry time changed"})
					}

					userParams := make(map[string]string)
					userParams["twitter_connected"] = "True"
					userParams["twitter_access_token"] = token.AccessToken
					userParams["twitter_refresh_token"] = token.RefreshToken
					userParams["twitter_token_expiry_time"] = query.TimeString(token.ExpiryTime)

					err = user.Update(userParams)
					if err != nil {
						log.Error(log.V{"Error updating twitter token in user": err})
					}

				} else {
					log.Error(log.V{"Error retrieving access token from refresh token: ": err})
					askUserToConnectTwitter(rdb, ctx, user)
					continue
				}
			}

			// Check if the user is connected to Twitter
			if !user.TwitterConnected {
				askUserToConnectTwitter(rdb, ctx, user)
				continue
			}

			in := &gotwi.NewClientWithAccessTokenInput{
				AccessToken: twitterAccessToken,
			}

			c, err := gotwi.NewClientWithAccessToken(in)

			if err == nil {

				p := &userType.GetMeInput{
					UserFields: fields.UserFieldList{
						fields.UserFieldCreatedAt,
						fields.UserFieldPublicMetrics,
					},
				}

				u, err := userlookup.GetMe(context.Background(), c, p)

				if err == nil {
					log.Info(log.V{"ID: ": gotwi.StringValue(u.Data.ID)})
					log.Info(log.V{"Name: ": gotwi.StringValue(u.Data.Name)})
					log.Info(log.V{"Username: ": gotwi.StringValue(u.Data.Username)})
					log.Info(log.V{"Followers Count: ": gotwi.IntValue(u.Data.PublicMetrics.FollowersCount)})

					// Check if the list is older than 24 hours
					currentTime := time.Now().UTC()

					year, month, day := currentTime.Date()

					redisFollowerCountKey := fmt.Sprintf("%d-%s-%d", day, month, year)

					// Set follower count in redis for generating insights
					rdb.HSet(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_total_follower_count_suffix"), redisFollowerCountKey, gotwi.IntValue(u.Data.PublicMetrics.FollowersCount))

					listCreationTime := user.TwitterListCreationTime

					elapsedTime := currentTime.Sub(listCreationTime).Hours()

					if elapsedTime > 24 {
						// Get the followers, Create list, Add members to the list
						getFollowers(c, user)
						createList(c, user)

						// Get updated user data
						user, err = userModel.Find(user.ID)

						if err == nil {
							addMembersToList(c, user)
						} else {
							log.Error(log.V{"Error retrieving updated user data": err})
						}

					}

					listExists, err := checkIfListExists(user.TwitterListID, c)

					if err == nil && listExists && (user.AutoLike || user.Notification) {
						listTweetLookupInput := listTweetLookupInput.ListInput{
							ID:              user.TwitterListID,
							MaxResults:      5,
							PaginationToken: "",
							Expansions: fields.ExpansionList{
								fields.ExpansionAuthorID,
							},
							TweetFields: fields.TweetFieldList{
								fields.TweetFieldID,
								fields.TweetFieldAuthorID,
								fields.TweetFieldPossiblySensitive,
								fields.TweetFieldText,
							},
							UserFields: fields.UserFieldList{
								fields.UserFieldID,
								fields.UserFieldUsername,
								fields.UserFieldName,
							},
						}

						listTweetLookupOutput, err := listtweetlookup.List(context.Background(), c, &listTweetLookupInput)

						if err == nil {

							for _, tweet := range listTweetLookupOutput.Data {
								if !rdb.SIsMember(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_tweet_ids_suffix"), gotwi.StringValue(tweet.ID)).Val() {
									log.Info(log.V{"Twitter retrieved for user: ": user.TwitterUsername})
									log.Info(log.V{"Tweet ID: ": gotwi.StringValue(tweet.ID)})
									log.Info(log.V{"Tweet Field Author ID: ": gotwi.StringValue(tweet.AuthorID)})
									authorID := gotwi.StringValue(tweet.AuthorID)
									log.Info(log.V{"Tweet Possibly Sensitive: ": gotwi.BoolValue(tweet.PossiblySensitive)})
									log.Info(log.V{"Tweet Text: ": gotwi.StringValue(tweet.Text)})

									if gotwi.BoolValue(tweet.PossiblySensitive) {
										log.Info(log.V{"Tweet potentially sensitive, so skipping: ": gotwi.StringValue(tweet.Text)})
										continue
									}

									var tweetUserName, tweetRealName string

									for _, tweetUserFields := range listTweetLookupOutput.Includes.Users {
										if authorID == gotwi.StringValue(tweetUserFields.ID) {
											log.Info(log.V{"Tweet User Field Author ID: ": gotwi.StringValue(tweetUserFields.ID)})
											log.Info(log.V{"Tweet User Field Author UserName: ": gotwi.StringValue(tweetUserFields.Username)})
											tweetUserName = gotwi.StringValue(tweetUserFields.Username)
											log.Info(log.V{"Tweet User Field Author Name: ": gotwi.StringValue(tweetUserFields.Name)})
											tweetRealName = gotwi.StringValue(tweetUserFields.Name)
										}
									}

									if len(user.Keywords) > 0 {
										categories := classifyTweet(gotwi.StringValue(tweet.Text), user.Keywords)

										if len(categories) > 0 {
											log.Info(log.V{"Tweet matches categories: ": categories})
											tweetText := "<br/><br/>" + gotwi.StringValue(tweet.Text) + "<br/><br/>" + "From " + tweetRealName + "(" + "<a style='color: #1363DF;' href='https://twitter.com/" + tweetUserName + "/" + "status/" + gotwi.StringValue(tweet.ID) + "'>" + "@" + tweetUserName + "</a>" + ")" + "<br/><br/>" + "Matches topics: " + fmt.Sprintf("%v", categories) + "<br/><br/>" + "<hr>"
											log.Info(log.V{"Formatted Tweet: ": tweetText})
											rdb.SAdd(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_tweets_suffix"), tweetText)

											if user.AutoLike {
												err := likeTweets(user.TwitterId, gotwi.StringValue(tweet.ID), c)

												if err == nil {
													log.Info(log.V{"Liked Tweet: ": tweetText})
												} else {
													log.Error(log.V{"Error liking tweet: ": err})
													getDetailedError(err)
												}
											}
										}
									}

									rdb.SAdd(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_tweet_ids_suffix"), gotwi.StringValue(tweet.ID))
								} else {
									log.Info(log.V{"Tweet already processed, So do nothing: ": gotwi.StringValue(tweet.Text)})
								}
							}
						}
					} else {
						log.Error(log.V{"Unable to find the list for tweet lookup: ": err})
						log.Info(log.V{"User Auto Like": user.AutoLike, "User Notification": user.Notification, "List exists:": listExists})
						// TODO send user email ABOUT both auto-like and notification is disabled
						continue
					}
				} else {
					log.Error(log.V{"Twitter user lookup failed": err})
					getDetailedError(err)
					askUserToConnectTwitter(rdb, ctx, user)
				}

			} else {
				log.Error(log.V{"Error in Gotwi client": err})
				getDetailedError(err)
			}

		}
	}
}

func askUserToConnectTwitter(rdb *redis.Client, ctx context.Context, user *userModel.User) {
	twitterConnectEmailTime, err := rdb.Get(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_twitter_connection_suffix")).Result()
	var sendTwitterConnectEmail bool

	if err == nil {
		if twitterConnectEmailTime != "" {
			parsedTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", twitterConnectEmailTime)
			if err == nil {
				currentTime := time.Now().UTC()
				elapsedTime := currentTime.Sub(parsedTime).Hours()

				if elapsedTime > 24 {
					sendTwitterConnectEmail = true
				} else {
					sendTwitterConnectEmail = false
				}
			}
		}
	} else if err.Error() == "redis: nil" {
		sendTwitterConnectEmail = true
	} else {
		log.Error(log.V{"Error retrieving twitter connect email from redis": err})
	}

	if sendTwitterConnectEmail {
		// Mandrill implementation
		client := m.ClientWithKey(config.Get("mandrill_key"))

		message := &m.Message{}
		message.AddRecipient(user.Email, user.Name, "to")
		message.FromEmail = config.Get("email_digest_email")
		message.FromName = config.Get("email_from_name")
		message.Subject = config.Get("email_twitter_connect_subject")

		// Global vars
		message.GlobalMergeVars = m.MapToVars(map[string]interface{}{"FNAME": user.Name})
		templateContent := map[string]string{}

		response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_twitter_connect_template"), templateContent)
		if err != nil {
			log.Error(log.V{"msg": "Twitter connect email, error sending password reset email", "error": err})
		} else {
			log.Info(log.V{"msg": "Twitter connect email, response from the server", "response": response})

			// store the time to avoid sending the email again immediately
			rdb.Set(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_twitter_connection_suffix"), time.Now().UTC().String(), 0)

			// Change the Twitter Connected status to false
			userParams := make(map[string]string)
			userParams["twitter_connected"] = "False"

			err = user.Update(userParams)
			if err != nil {
				log.Error(log.V{"Error updating twitter connected status in user": err})
			}
		}
	}
}

func classifyTweet(tweetText string, keywords []string) []string {
	type bertResults struct {
		Label string  `json:"label"`
		Score float64 `json:"score"'`
	}

	var bertResult []bertResults

	for _, hashtag := range keywords {
		type bertRequest struct {
			Text   string   `json:"text"`
			Labels []string `json:"labels"`
		}

		var labels []string
		labels = append(labels, hashtag)

		req := &bertRequest{
			Text:   tweetText,
			Labels: labels,
		}

		postBody, err := json.Marshal(req)
		if err != nil {
			log.Error(log.V{"Update stories. Error creating POST body for bert: ": err})
		}

		responseBody := bytes.NewBuffer(postBody)
		//Leverage Go's HTTP Post function to make request
		resp, err := http.Post("http://192.168.3.159:8000/classification", "application/json", responseBody)
		//Handle Error
		if err != nil {
			log.Info(log.V{"Bert, An Error Occurred": err})
		}
		defer resp.Body.Close()
		//Read the response body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error(log.V{"Bert An Error Occurred": err})
		}

		type bertResponse struct {
			Labels []string  `json:"labels"`
			Scores []float64 `json:"scores"'`
		}

		res := bertResponse{}
		json.Unmarshal(body, &res)

		log.Info(log.V{"Bert, Tweet Text: %s": tweetText})

		for i, score := range res.Scores {
			log.Info(log.V{"Bert, Label: ": res.Labels[i], "Score: ": res.Scores[i]})
			if score > 0.90 {
				log.Info(log.V{"Bert score matches threshold, Label: ": res.Labels[i], "Score: ": res.Scores[i]})
				b := bertResults{
					Label: res.Labels[i],
					Score: res.Scores[i],
				}
				bertResult = append(bertResult, b)
			}
		}
	}

	sort.Slice(bertResult, func(i, j int) bool {
		return bertResult[i].Score > bertResult[j].Score
	})

	if len(bertResult) > 3 {
		bertResult = bertResult[:3]
	}

	var categories []string
	for i := range bertResult {
		categories = append(categories, bertResult[i].Label)
	}

	return categories
}

func likeTweets(id string, tweetID string, c *gotwi.Client) error {
	tweetLikeInput := tweetLikeType.CreateInput{
		ID:      id,
		TweetID: tweetID,
	}

	tweetLike, err := like.Create(context.Background(), c, &tweetLikeInput)

	if err == nil {
		log.Info(log.V{"Tweet: ": id, "liked successfully: ": tweetLike.Data.Liked})
	}

	return err
}

func getTwitterFollowers(user *userModel.User, c *gotwi.Client, paginationToken string) (*followType.ListFollowersOutput, error) {
	followInput := followType.ListFollowersInput{
		ID:              user.TwitterId,
		MaxResults:      1000,
		PaginationToken: paginationToken,
		Expansions:      nil,
		TweetFields:     nil,
		UserFields: fields.UserFieldList{
			fields.UserFieldID,
		},
	}

	followers, err := follow.ListFollowers(context.Background(), c, &followInput)

	if err == nil {
		return followers, nil
	}

	return followers, err
}

func getTwitterAccessToken(refreshToken string) (Token, error) {
	token := Token{}
	encodedClientCreds := base64.StdEncoding.EncodeToString([]byte(config.Get("client_Id") + ":" + config.Get("client_secret")))

	apiUrl := "https://api.twitter.com/2/oauth2/token"
	data := url.Values{}
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

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

			json.Unmarshal(body, &token)

			if token.Error != "" {
				log.Error(log.V{"Twitter connect Error: ": token.Error, "Error description: ": token.ErrorDescription})
				return token, err
			}

			t := time.Now().UTC()

			if token.ExpiresIn == 7200 {
				token.ExpiryTime = t.Add(time.Second * 7200)
			} else {
				// TODO: Implement routine to know if the expiry has been changed
				log.Error(log.V{"Twitter Expiry token": "Token expiry time changed"})
			}

		}
	}
	return token, err
}

func Shuffle(vals []string) {
	// We start at the end of the slice, inserting our random
	// values one at a time.
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for len(vals) > 0 {
		n := len(vals)
		randIndex := r.Intn(n)
		// We swap the value at index n-1 and the random index
		// to move our randomly chosen value to the end of the
		// slice, and to move the value that was at n-1 into our
		// unshuffled portion of the slice.

		vals[n-1], vals[randIndex] = vals[randIndex], vals[n-1]
		vals = vals[:n-1]
	}
}
