package lantern

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"

	"github.com/getlantern/eventual"
	"github.com/getlantern/flashlight/util"
)

const (
	// the Feed endpoint where recent content is published to
	// mostly just a compendium of RSS feeds
	feedEndpoint = `https://feeds.getiantem.org/%s/feed.json`
)

var (
	feed Feed

	httpClient *http.Client

	// locales we have separate feeds available for
	supportedLocales = map[string]bool{
		"en_US": true,
		"fa_IR": true,
		"fa":    true,
		"zh_CN": true,
	}
)

// Feed contains the data we get back
// from the public feed
type Feed struct {
	Feeds   map[string]Source    `json:"feeds"`
	Entries FeedItems            `json:"entries"`
	Items   map[string]FeedItems `json:"-"`
}

// Source represents a feed authority,
// a place where content is fetched from
// e.g. BBC, NYT, Reddit, etc.
type Source struct {
	FeedUrl string `json:"feedUrl"`
	Title   string `json:"title"`
	Url     string `json:"link"`
	Entries []int  `json:"entries"`
}

type FeedItem struct {
	Title       string                 `json:"title"`
	Link        string                 `json:"link"`
	Image       string                 `json:"image"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
	Description string                 `json:"-"`
}

type FeedItems []FeedItem

type FeedProvider interface {
	AddSource(string)
}

type FeedRetriever interface {
	AddFeed(string, string, string, string)
	Finish()
}

func FeedByName(name string, retriever FeedRetriever) {
	if items, exists := feed.Items[name]; exists {
		for _, i := range items {
			retriever.AddFeed(i.Title, i.Description,
				i.Image, i.Link)
		}
	}
	retriever.Finish()
}

// GetFeed creates an http.Client and fetches the latest
// Lantern public feed for displaying on the home screen.
// If a proxyAddr is specified, the http.Client will proxy
// through it
func GetFeed(locale string, proxyAddr string, provider FeedProvider) {
	var err error
	var req *http.Request
	var res *http.Response

	if !supportedLocales[locale] {
		// always default to English if we don't
		// have a feed available in a specific locale
		locale = "en_US"
	}

	feedUrl := fmt.Sprintf(feedEndpoint, locale)

	if req, err = http.NewRequest("GET", feedUrl, nil); err != nil {
		log.Errorf("Error fetching feed: %v", err)
		return
	}

	if proxyAddr == "" {
		httpClient = &http.Client{}
	} else {
		httpClient, err = util.HTTPClient("", eventual.DefaultGetter(proxyAddr))
		if err != nil {
			log.Errorf("Error creating client: %v", err)
			return
		}
	}

	if res, err = httpClient.Do(req); err != nil {
		log.Errorf("Error fetching feed: %v", err)
		return
	}

	defer res.Body.Close()
	contents, err := ioutil.ReadAll(res.Body)

	if err != nil {
		log.Errorf("Error reading body: %v", err)
		return
	}

	json.Unmarshal(contents, &feed)
	processFeed(provider)
}

// processFeed is used after a feed has been downloaded
// to extract feed sources and items for processing.
func processFeed(provider FeedProvider) {

	log.Debugf("Num of Feed Entries: %v", len(feed.Entries))

	feed.Items = make(map[string]FeedItems)

	// the 'all' tab contains every article
	feed.Items["all"] = feed.Entries

	// Get a list of feed sources & send those back to the UI
	for _, s := range feed.Feeds {
		if s.Title != "" {
			log.Debugf("Adding feed source: %s", s.Title)
			provider.AddSource(s.Title)
		}
	}

	// Add a (shortened) description to every article
	for i, entry := range feed.Entries {
		desc := ""
		if aDesc := entry.Meta["description"]; aDesc != nil {
			desc = strings.TrimSpace(aDesc.(string))
		}
		min := int(math.Min(float64(len(desc)), 150))
		feed.Entries[i].Description = desc[:min]
	}

	for _, s := range feed.Feeds {
		for _, i := range s.Entries {
			entry := feed.Entries[i]
			// every feed item gets appended to a feed source array
			// for quick reference
			feed.Items[s.Title] = append(feed.Items[s.Title], entry)
		}
	}
}