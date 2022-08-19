// package subscriptions refers to subscriptions resource
package subscriptions

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/resource"
	"github.com/abishekmuthian/engagefollowers/src/lib/status"
	"time"
)

// Subscription handles saving and retrieving users from the database
type Subscription struct {
	// resource.Base defines behaviour and fields shared between all resources
	resource.Base

	// status.ResourceStatus defines a status field and associated behaviour
	status.ResourceStatus

	Created        time.Time
	AmountTotal    float64
	AmountSubTotal float64
	Currency       string
	CustomerId     string
	CustomerEmail  string
	SubscriptionId string
	UserId         int64
	Plan           string
	Invoice        string
}
