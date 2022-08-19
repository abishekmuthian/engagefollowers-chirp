package useractions

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/abishekmuthian/engagefollowers/src/lib/mux"
	"github.com/abishekmuthian/engagefollowers/src/lib/server"
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

	if !currentUser.TrialEnd.IsZero() && !currentUser.Subscription {
		return server.Redirect(w, r, "/?error=not_subscribed")
	}

	// Fetch the params
	params, err := mux.Params(r)
	if err != nil {
		return server.InternalError(err)
	}

	keywordsParam := params.Get("keywords")
	autoLikeParam := params.Get("autolike")
	emailParam := params.Get("email")

	log.Info(log.V{"Auto Like": autoLikeParam})
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

	if len(autoLikeParam) > 0 && autoLikeParam == "True" {
		userParams["auto_like"] = autoLikeParam
	} else {
		userParams["auto_like"] = "False"
	}

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
