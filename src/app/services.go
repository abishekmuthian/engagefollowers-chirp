package app

import (
	"time"

	"github.com/abishekmuthian/engagefollowers/src/lib/server/config"
	"github.com/abishekmuthian/engagefollowers/src/lib/server/log"
	useractions "github.com/abishekmuthian/engagefollowers/src/users/actions"
)

// SetupServices sets up external services from our config file
func SetupServices() {

	// Don't send if not on production server
	if !config.Production() {
		return
	}

	now := time.Now().UTC()

	// Update Tweets
	updateInterval := 60 * time.Minute // Schedule every 60 minutes

	// Starting immediately on launch for testing
	// updateTime := now.Add(time.Second * 2)

	// Starting 45 minutes after launch
	updateTime := now.Add(time.Minute * 45)

	ScheduleAt(useractions.GetTweetsOfFollowers, updateTime, updateInterval)

	// Setup profile update

	// Update Profile Banner
	profileUpdateInterval := 60 * time.Minute // Schedule every 60 minutes

	// Start immediately for testing
	// profileUpdateTime := now.Add(time.Minute * 2)

	// Starting 15 minutes after launch
	profileUpdateTime := now.Add(time.Minute * 15) // Update profile banner every hour

	ScheduleAt(useractions.GenerateProfileBanner, profileUpdateTime, profileUpdateInterval)

	// Set up mail
	if config.Get("mandrill_key") != "" {

		// Email digest
		emailInterval := 24 * time.Hour // Send email every day

		// Send email after 60 seconds for testing
		// emailTime := now.Add(time.Second * 60)

		// Send the email every day at 2AM UTC
		emailTime := time.Date(now.Year(), now.Month(), now.Day(), 2, 00, 00, 00, time.UTC)

		ScheduleAt(useractions.EmailDailyDigest, emailTime, emailInterval)

		// To use when trial plan is announced
		/*		//End Trial
				// Email delivery
				emailInterval = 24 * time.Hour // Send emails every 24 hours
				// Send one immediately on launch
				emailTime = now.Add(time.Second * 2)
				ScheduleAt(useractions.EmailEndOfTrialNotification, emailTime, emailInterval)*/
	} else {
		log.Error(log.V{"Services": "Mandrill key missing"})
	}

}

// ScheduleAt schedules execution for a particular time and at intervals thereafter.
// If interval is 0, the function will be called only once.
// Callers should call close(task) before exiting the app or to stop repeating the action.
func ScheduleAt(f func(), t time.Time, i time.Duration) chan struct{} {
	task := make(chan struct{})
	now := time.Now().UTC()

	// Check that t is not in the past, if it is increment it by interval until it is not
	for now.Sub(t) > 0 {
		t = t.Add(i)
	}

	// We ignore the timer returned by AfterFunc - so no cancelling, perhaps rethink this
	tillTime := t.Sub(now)
	time.AfterFunc(tillTime, func() {
		// Call f at least once at the time specified
		go f()

		// If we have an interval, call it again repeatedly after interval
		// stopping if the caller calls stop(task) on returned channel
		if i > 0 {
			ticker := time.NewTicker(i)
			go func() {
				for {
					select {
					case <-ticker.C:
						go f()
					case <-task:
						ticker.Stop()
						return
					}
				}
			}()
		}
	})

	return task // call close(task) to stop executing the task for repeated tasks
}
