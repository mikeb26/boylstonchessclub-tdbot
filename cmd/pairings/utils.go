/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func parseArgs() string {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	url := flag.Arg(0)

	return url
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(),
		"Usage:\n\n%v <url>\n\nFetch tournament registration <url> and predict first round pairings.\n",
		os.Args[0])
}

func fetch(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// displayRating returns a string for the rating, showing "unrated" if 0
// a *suffix indicates that the rating was user reported and not from USCF
func displayRating(p Player) string {
	var ret string
	if p.Rating == RatingUnrated {
		ret = "unrated"
	} else {
		ret = strconv.Itoa(p.Rating)
	}
	if p.RType == RatingTypeReported {
		ret += "*"
	}

	return ret
}

func byeValFromReason(br ByeReason) float32 {
	if br == ByeReasonOdd {
		return 1.0
	}

	return 0.5
}

func round1ByeRequested(req string) bool {
	s := strings.TrimSpace(req)
	if s == "" {
		return false
	}
	// If input is just a number, e.g., "1"
	numOnly := regexp.MustCompile(`^\d+$`)
	if numOnly.MatchString(s) {
		if n, err := strconv.Atoi(s); err == nil && n == 1 {
			return true
		}
	}

	// Look for patterns like "round 1,5" or "rnds 1&4"
	sl := strings.ToLower(s)
	listRe := regexp.MustCompile(`(?i)\b(?:round|rnd|rounds|rnds)\b[\s:]*((?:\d+(?:\s*[,&;/]\s*\d+)*))`)
	if matches := listRe.FindStringSubmatch(sl); matches != nil {
		nums := regexp.MustCompile(`\d+`).FindAllString(matches[1], -1)
		for _, m := range nums {
			if n, err := strconv.Atoi(m); err == nil && n == 1 {
				return true
			}
		}
	}

	return false
}

func extractName(s string) string {
	re := regexp.MustCompile(`\b\d{6,8}:\s*([^<]+)</b>`)
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(htmlUnescape(m[1]))
	}
	return "Unknown"
}

func extractRating(s string) int {
	re := regexp.MustCompile(`Regular Rating[\s\S]*?<b[^>]*>([\s\S]*?)</b>`)
	if m := re.FindStringSubmatch(s); m != nil {
		inner := m[1]
		reTag := regexp.MustCompile(`<[^>]+>`)
		text := reTag.ReplaceAllString(inner, " ")
		text = htmlUnescape(text)
		reDigits := regexp.MustCompile(`\d+`)
		if d := reDigits.FindString(text); d != "" {
			r, err := strconv.Atoi(d)
			if err == nil {
				return r
			}
		}
	}
	return 0
}

func removeIndex(s []Player, i int) []Player {
	return append(s[:i], s[i+1:]...)
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&#39;", "'",
		"&quot;", `"`,
	)
	return r.Replace(s)
}

func normalizeName(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	first := parts[0]
	last := first
	if len(parts) > 1 {
		last = parts[len(parts)-1]
	}
	firstLower := strings.ToLower(first)
	lastLower := strings.ToLower(last)
	fn := []rune(firstLower)
	ln := []rune(lastLower)
	firstTitle := string(unicode.ToUpper(fn[0])) + string(fn[1:])
	lastTitle := string(unicode.ToUpper(ln[0])) + string(ln[1:])
	if firstTitle == lastTitle {
		return firstTitle
	}
	return firstTitle + " " + lastTitle
}
