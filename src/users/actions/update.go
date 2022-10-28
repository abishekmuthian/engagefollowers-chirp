package useractions

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/abishekmuthian/engagefollowers/src/lib/query"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	"github.com/abishekmuthian/engagefollowers/src/users"
	"github.com/dghubble/oauth1"
	"github.com/lib/pq"
)

// UpdateKeywords updates keywords (topics) of the user in db
func UpdateKeywords(keywords []string, userId int64) error {
	_, err := query.Exec("UPDATE users SET keywords=$1 WHERE id=$2", pq.Array(keywords), userId)
	return err
}

// UpdateFollowers updates followers of the user in db
func UpdateFollowers(followers []string, userId int64) error {
	_, err := query.Exec("UPDATE users SET twitter_followers=$1 WHERE id=$2", pq.Array(followers), userId)
	return err
}

// UpdateProfileBanner updates the Twitter Profile Banner
func UpdateProfileBanner(currentUser *users.User, base64Image string) error {
	/* 	method := http.MethodPost
	   	url := "https://api.twitter.com/1.1/account/update_profile_banner.json"

	   	auth := oauth1.OAuth1{
	   		ConsumerKey:    config.Current.Get("twitter_api_key"),
	   		ConsumerSecret: config.Current.Get("twitter_api_key_secret"),
	   		AccessToken:    currentUser.TwitterOauthToken,
	   		AccessSecret:   currentUser.TwitterOauthTokenSecret,
	   	}

	   	authHeader := auth.BuildOAuth1Header(method, url, map[string]string{
	   		"banner": base64Image,
	   	})

	   	req, err := http.NewRequest(method, url, nil)

	   	if err != nil {
	   		log.Error(log.V{"Update, Profile banner update, Error in request": err})
	   		return err
	   	}

	   	req.Header.Set("Authorization", authHeader)
	   	// req.URL.RawQuery = req.URL.Query().Encode()

	   	if res, err := http.DefaultClient.Do(req); err == nil {
	   		fmt.Println(res.StatusCode)
	   	} else {
	   		log.Error(log.V{"Update, Profile banner update, Error in response": err})
	   		return err
	   	} */

	config := oauth1.NewConfig(config.Current.Get("twitter_api_key"), config.Current.Get("twitter_api_key_secret"))
	token := oauth1.NewToken(currentUser.TwitterOauthToken, currentUser.TwitterOauthTokenSecret)
	client := config.Client(oauth1.NoContext, token)

	apiUrl := "https://api.twitter.com/1.1/account/update_profile_banner.json"
	data := url.Values{}
	data.Set("banner", base64Image)

	u, err := url.ParseRequestURI(apiUrl)

	if err != nil {
		log.Error(log.V{"Twitter update banner, Error in parse request URI": err})
		server.InternalError(err)
	}

	urlStr := u.String()

	r, err := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(data.Encode())) // URL-encoded payload

	if err != nil {
		log.Error(log.V{"update, Error creating request": err})
		return err
	}
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

	return err
}
