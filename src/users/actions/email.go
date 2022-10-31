package useractions

import (
	"context"
	"strconv"
	"strings"
	"time"

	m "github.com/abishekmuthian/engagefollowers/src/lib/mandrill"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	userModel "github.com/abishekmuthian/engagefollowers/src/users"
	"github.com/go-redis/redis/v8"
)

// EmailDailyDigest sends the daily digest of categorized tweets to the user
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

// sendTwitterConnectEmail sends the email asking the user to connect their Twitter account
func sendTwitterConnectEmail(user *userModel.User, rdb *redis.Client, ctx context.Context) {
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
		log.Error(log.V{"msg": "Twitter connect email, error sending Twitter connect email", "error": err})
	} else {
		log.Info(log.V{"msg": "Twitter connect email, response from the server", "response": response})

		// store the time to avoid sending the email again immediately
		rdb.Set(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_twitter_connection_suffix"), time.Now().UTC().String(), 0)

		// Increment the number of times this email has been sent
		rdb.IncrBy(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_ask_twitter_connection_count_suffix"), 1)

		// Change the Twitter Connected status to false
		userParams := make(map[string]string)
		userParams["twitter_connected"] = "False"

		err = user.Update(userParams)
		if err != nil {
			log.Error(log.V{"Error updating twitter connected status in user": err})
		}
	}
}

// sendProfileBannerEmail sends the email informing the user about profile banner change
func sendProfileBannerEmail(user *userModel.User, rdb *redis.Client, ctx context.Context) {
	// Mandrill implementation
	client := m.ClientWithKey(config.Get("mandrill_key"))

	message := &m.Message{}
	message.AddRecipient(user.Email, user.Name, "to")
	message.FromEmail = config.Get("email_digest_email")
	message.FromName = config.Get("email_from_name")
	message.Subject = config.Get("email_profile_banner_set_subject")

	// Global vars
	message.GlobalMergeVars = m.MapToVars(map[string]interface{}{"FNAME": user.Name, "TWITTER_PROFILE": "https://twitter.com/" + user.TwitterUsername})
	templateContent := map[string]string{}

	response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_profile_banner_template"), templateContent)
	if err != nil {
		log.Error(log.V{"msg": "Profile banner email, error sending Twitter connect email", "error": err})
	} else {
		log.Info(log.V{"msg": "Profile banner email, response from the server", "response": response})
	}
}

// sendSetTopicsEmail sends the email to the user asking them to set the topics
func sendSetTopicsEmail(user *userModel.User, rdb *redis.Client, ctx context.Context) {
	// Mandrill implementation
	client := m.ClientWithKey(config.Get("mandrill_key"))

	message := &m.Message{}
	message.AddRecipient(user.Email, user.Name, "to")
	message.FromEmail = config.Get("email_digest_email")
	message.FromName = config.Get("email_from_name")
	message.Subject = config.Get("email_set_topics_subject")

	// Global vars
	message.GlobalMergeVars = m.MapToVars(map[string]interface{}{"FNAME": user.Name})
	templateContent := map[string]string{}

	response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_set_topics_template"), templateContent)
	if err != nil {
		log.Error(log.V{"msg": "Set topics email, error sending topics email", "error": err})
	} else {
		log.Info(log.V{"msg": "Set topics email, response from the server", "response": response})

		// store the time to avoid sending the email again immediately
		rdb.Set(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_set_topics_suffix"), time.Now().UTC().String(), 0)

		// Increment the number of times this email has been sent
		rdb.IncrBy(ctx, config.Get("redis_key_prefix")+strconv.FormatInt(user.ID, 10)+config.Get("redis_key_ask_set_topics_count_suffix"), 1)
	}
}

// sendAdminEmail sends the email to the Admin about the errors
func sendAdminEmail(user *userModel.User, subject string, errorMessage string) {
	// Mandrill implementation
	client := m.ClientWithKey(config.Get("mandrill_key"))

	message := &m.Message{}
	message.AddRecipient(config.Get("admin_email"), config.Get("admin_name"), "to")
	message.FromEmail = config.Get("email_digest_email")
	message.FromName = config.Get("email_from_name")
	message.Subject = subject

	// Global vars
	message.GlobalMergeVars = m.MapToVars(map[string]interface{}{"FNAME": config.Get("admin_name"), "TNAME": user.Name, "TUNAME": user.TwitterUsername, "ERROR": errorMessage})
	templateContent := map[string]string{}

	response, err := client.MessagesSendTemplate(message, config.Get("mailchimp_admin_email_template"), templateContent)
	if err != nil {
		log.Error(log.V{"msg": "Twitter connect email to admin, error sending Twitter connect admin email", "error": err})
	} else {
		log.Info(log.V{"msg": "Twitter connect email to admin, response from the server", "response": response})
	}
}
