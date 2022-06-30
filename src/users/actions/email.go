package useractions

import (
	"context"
	m "github.com/abishekmuthian/engagefollowers/src/lib/mandrill"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	userModel "github.com/abishekmuthian/engagefollowers/src/users"
	"github.com/go-redis/redis/v8"
	"strconv"
	"strings"
)

func EmailDailyDigest() {
	// Initialize redis
	var ctx = context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Get("redis_server"),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	q := userModel.Query()

	// User should have email notification enabled
	q.Where("notification is TRUE")

	// Fetch the userModel
	users, err := userModel.FindAll(q)
	if err != nil {
		log.Error(log.V{"message": "email: error getting users for email", "error": err})
		return
	}

	if len(users) > 0 {
		for _, user := range users {
			var bodyContent strings.Builder

			tweets, err := rdb.SMembers(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_tweets_suffix")).Result()

			if err == nil {
				if len(tweets) > 0 {
					for _, tweet := range tweets {
						bodyContent.WriteString(tweet)
					}
					// Mandrill implementation
					client := m.ClientWithKey(config.Get("mandrill_key"))

					message := &m.Message{}
					message.AddRecipient(user.Email, user.Name, "to")
					message.FromEmail = config.Get("email_digest_email")
					message.FromName = config.Get("email_from_name")
					message.Subject = config.Get("email_digest_subject")
					//message.HTML = "<h1> Click this url " + url + " to reset the password in your account.</h1>"
					//message.Text = "Click this url " + url + " to reset the password in your account."

					// Global vars
					message.GlobalMergeVars = m.MapToVars(map[string]interface{}{"FNAME": user.Name, "EMAILDIGEST": bodyContent.String()})
					templateContent := map[string]string{}

					response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_email_digest_template"), templateContent)
					if err != nil {
						log.Error(log.V{"msg": "Email digest email, error sending password reset email", "error": err})
					} else {
						log.Info(log.V{"msg": "Email digest email, response from the server", "response": response})

						//Remove the tweets from redis
						rdb.Del(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_tweets_suffix"))
					}
				} else {
					log.Info(log.V{"No tweets stored for the user": "@" + user.TwitterUsername})
				}
			} else {
				log.Error(log.V{"Error retrieving tweets for sending email digest": err})
			}
		}
	}
}
