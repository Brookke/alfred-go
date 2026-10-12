// Command search searches the web from Alfred: type "s alfred workflows".
//
//	go run ./examples/search install
package main

import (
	"net/url"
	"strings"

	alfred "github.com/brookke/alfred-go"
)

func main() {
	alfred.Main(alfred.Workflow("Search",
		alfred.ScriptFilter("s", search).
			Title("Search the web").
			Subtitle("Type something to search for").
			Then(alfred.OpenURL()).
			On(alfred.Cmd, "Copy URL", alfred.Copy()),
	).
		BundleID("com.example.search").
		Version("1.0.0").
		Description("Search Google, DuckDuckGo or Wikipedia"))
}

var engines = []struct{ name, url string }{
	{"Google", "https://www.google.com/search?q="},
	{"DuckDuckGo", "https://duckduckgo.com/?q="},
	{"Wikipedia", "https://en.wikipedia.org/w/index.php?search="},
}

func search(query string) alfred.Output {
	query = strings.TrimSpace(query)
	if query == "" {
		return alfred.Items{alfred.Item("Type something to search for").Invalid()}
	}
	var items alfred.Items
	for _, engine := range engines {
		items = append(items, alfred.Item("Search "+engine.name+" for “"+query+"”").
			Subtitle(engine.name).
			Arg(engine.url+url.QueryEscape(query)))
	}
	return items
}
