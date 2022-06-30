package app

import (
	"github.com/abishekmuthian/engagefollowers/src/lib/stats"
	"github.com/abishekmuthian/engagefollowers/src/lib/view"
	"net/http"
)

// HandlePrivacy displays the home page
// responds to GET /privacy
func HandleLegal(w http.ResponseWriter, r *http.Request) error {
	stats.RegisterHit(r)

	// Render the template
	view := view.NewRenderer(w, r)

	view.Template("app/views/legal.html.got")

	return view.Render()
}
