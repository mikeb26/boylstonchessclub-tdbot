package uschess

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mikeb26/boylstonchessclub-tdbot/internal"
)

type Event struct {
	EndDate time.Time
	Name    string
	ID      int
}

// GetAffiliateEvents fetches and parses the Affiliate Tournament History page
// for the given affiliate code and returns a slice of Event.
func GetAffiliateEvents(affiliateCode string) ([]Event, error) {

	url := fmt.Sprintf("https://www.uschess.org/msa/AffDtlTnmtHst.php?%s",
		affiliateCode)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", internal.UserAgent)

	client := internal.NewCachedHttpClient(context.Background(), 24*time.Hour)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var events []Event

	// Find the main events table
	table := doc.Find("table[width='750'][border='1']").First()

	// Iterate each row in the table
	table.Find("tr").Each(func(_ int, row *goquery.Selection) {
		dateTd := row.Find("td[width='120']")
		if dateTd.Length() == 0 {
			return // not an event row
		}

		// Extract end date (text node before <small>)
		endDateStr := dateTd.Contents().FilterFunction(func(i int, s *goquery.Selection) bool {
			return goquery.NodeName(s) == "#text"
		}).Text()
		endDateStr = strings.TrimSpace(endDateStr)

		// Extract event ID from <small>
		id := strings.TrimSpace(dateTd.Find("small").Text())
		idInt, err := strconv.Atoi(id)
		if err != nil {
			// skip events with invalid ID
			return
		}

		// Extract event name from the link in the second cell
		name := strings.TrimSpace(row.Find("td").Eq(1).Find("a").Text())

		endDate, err := internal.ParseDateOrZero(endDateStr)
		if err != nil {
			log.Printf("*warning: unable to parse date %v for event %v\n",
				endDateStr, id)
			endDate = time.Time{}
		}
		events = append(events, Event{
			EndDate: endDate,
			Name:    name,
			ID:      idInt,
		})
	})

	return events, nil
}
