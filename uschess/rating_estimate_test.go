/* Copyright © 2026 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */

package uschess

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestKFactor_Standard(t *testing.T) {
	k := KFactor(2000, 20, 4, false)
	want := 800.0 / (20.0 + 4.0)
	if math.Abs(k-want) > 1e-9 {
		t.Fatalf("KFactor standard: got %v want %v", k, want)
	}
}

func TestKFactor_DualRatedOTBR_2200To2500(t *testing.T) {
	R := 2400.0
	N0 := 20.0
	m := 4
	k := KFactor(R, N0, m, true)
	want := 800.0 * (6.5 - 0.0025*R) / (N0 + float64(m))
	if math.Abs(k-want) > 1e-9 {
		t.Fatalf("KFactor dual-rated 2200<R<2500: got %v want %v", k, want)
	}
}

func TestKFactor_DualRatedOTBR_AtLeast2500(t *testing.T) {
	R := 2550.0
	N0 := 20.0
	m := 4
	k := KFactor(R, N0, m, true)
	want := 200.0 / (N0 + float64(m))
	if math.Abs(k-want) > 1e-9 {
		t.Fatalf("KFactor dual-rated R>=2500: got %v want %v", k, want)
	}
}

func TestGetRatingEstimate_ProvisionalUsesSpecial(t *testing.T) {
	// Prior games <= 8 => special formula. With score==expected, the rating should
	// remain (very close to) the prior rating.
	old := 1500.0
	priorGames := 7
	opps := []float64{1500, 1500, 1500, 1500}
	score := 2.0 // expected score against equal opponents is 2.0

	newR, err := getRatingEstimate(old, priorGames, score, opps, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(newR-old) > 1e-6 {
		t.Fatalf("provisional special estimate drift: got %v want %v", newR, old)
	}
}

func TestGetRatingEstimate_EstablishedUsesStandardAndDualRatedK(t *testing.T) {
	old := 2300.0
	priorGames := 100
	opps := []float64{2300, 2300, 2300, 2300}
	score := 4.0

	newStandard, err := getRatingEstimate(old, priorGames, score, opps, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	newDual, err := getRatingEstimate(old, priorGames, score, opps, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With R>2200, the dual-rated K should be smaller than the standard K, so
	// the rating gain should be smaller as well.
	if !(newDual < newStandard) {
		t.Fatalf("expected dual-rated estimate < standard estimate; got dual=%v standard=%v", newDual, newStandard)
	}
}

type rewriteHostRoundTripper struct {
	base *url.URL
	up   http.RoundTripper
}

func (rt rewriteHostRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request and rewrite the destination to the test server.
	req2 := req.Clone(req.Context())
	u := *req.URL
	u.Scheme = rt.base.Scheme
	u.Host = rt.base.Host
	req2.URL = &u
	return rt.up.RoundTrip(req2)
}

func TestGetRatingEstimateWrap_UnratedOpponentErrors(t *testing.T) {
	ctx := context.Background()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Minimal routing for /api/v1/members/{id} and /api/v1/members/{id}/events
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/v1/members/") {
			trim := strings.TrimPrefix(path, "/api/v1/members/")
			if strings.HasSuffix(trim, "/events") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[]}`))
				return
			}
			idStr := trim
			if strings.Contains(idStr, "/") {
				idStr = strings.Split(idStr, "/")[0]
			}
			id, _ := strconv.Atoi(idStr)

			w.Header().Set("Content-Type", "application/json")
			switch id {
			case 1: // rated player
				_, _ = w.Write([]byte(`{
					"firstName":"A",
					"lastName":"Player",
					"ratings":[{"rating":1500,"ratingSystem":"R","isProvisional":false,"gamesPlayed":20,"floor":0}]
				}`))
			case 2: // unrated opponent
				_, _ = w.Write([]byte(`{
					"firstName":"B",
					"lastName":"Opp",
					"ratings":[{"rating":0,"ratingSystem":"R","isProvisional":false,"gamesPlayed":0,"floor":0}]
				}`))
			default:
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"not found"}`))
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	base, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parsing test server url: %v", err)
	}

	hc := &http.Client{Transport: rewriteHostRoundTripper{base: base, up: http.DefaultTransport}}

	c := &Client{httpClient1day: hc, httpClient30day: hc}

	_, err = c.GetRatingEstimate(ctx, 1, []MemID{2}, 1.0)
	if err == nil {
		t.Fatalf("expected error")
	}
}
