package useractions

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/query"
	"github.com/lib/pq"
)

// UpdateKeywords updates keywords of the user
func UpdateKeywords(keywords []string, userId int64) error {
	_, err := query.Exec("UPDATE users SET keywords=$1 WHERE id=$2", pq.Array(keywords), userId)
	return err
}

// UpdateFollowers updates followers of the user
func UpdateFollowers(followers []string, userId int64) error {
	_, err := query.Exec("UPDATE users SET twitter_followers=$1 WHERE id=$2", pq.Array(followers), userId)
	return err
}
