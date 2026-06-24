package worldcup

import (
	"context"
	"finance-backend/internal/config"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestParseGroupHTML(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(sampleGroupHTML))
	if err != nil {
		t.Fatalf("parse sample html: %v", err)
	}

	teams := parseTeams(doc)
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if teams[0].Name != "Mexico" || teams[0].WorldRank != 14 || teams[0].DrawRank != 15 {
		t.Fatalf("unexpected team parse: %+v", teams[0])
	}

	standings := parseStandings(doc)
	if len(standings) != 1 {
		t.Fatalf("expected 1 standing row, got %d", len(standings))
	}
	if standings[0].GroupRank != 1 || standings[0].GoalDifference != 4 || standings[0].Points != 6 {
		t.Fatalf("unexpected standing parse: %+v", standings[0])
	}

	matches := parseMatches(doc, "A")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Status != "finished" || matches[0].HomeScore == nil || *matches[0].HomeScore != 2 {
		t.Fatalf("unexpected match parse: %+v", matches[0])
	}
}

func TestLiveFetchGroupA(t *testing.T) {
	if os.Getenv("WORLD_CUP_LIVE_TEST") != "1" {
		t.Skip("set WORLD_CUP_LIVE_TEST=1 to hit the public MediaWiki API")
	}

	handler := &Handler{client: &http.Client{Timeout: requestTimeout}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	group, err := handler.fetchGroup(ctx, "A")
	if err != nil {
		t.Fatalf("fetch group A: %v", err)
	}
	if len(group.Teams) != 4 {
		t.Fatalf("expected 4 teams, got %d", len(group.Teams))
	}
	if len(group.Matches) != 6 {
		t.Fatalf("expected 6 matches, got %d", len(group.Matches))
	}
}

func TestLiveFetchTournament(t *testing.T) {
	if os.Getenv("WORLD_CUP_LIVE_TEST") != "1" {
		t.Skip("set WORLD_CUP_LIVE_TEST=1 to hit the public football APIs")
	}

	cfg := config.Load()
	if strings.TrimSpace(cfg.Football.APIKey) == "" {
		t.Skip("FOOTBALL_API_KEY is not configured")
	}

	handler := &Handler{
		cfg: Config{
			APIKey:          cfg.Football.APIKey,
			BaseURL:         cfg.Football.BaseURL,
			LeagueID:        cfg.Football.LeagueID,
			Season:          cfg.Football.Season,
			DisableEnvProxy: !cfg.Football.UseEnvProxy,
		},
	}
	handler.client = newAPIFootballHTTPClient(handler.cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	tournament, err := handler.fetchTournament(ctx)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "plan") {
			t.Skipf("API-FOOTBALL plan does not allow this season: %v", err)
		}
		t.Fatalf("fetch tournament: %v", err)
	}
	if tournament.Summary.GroupCount != 12 {
		t.Fatalf("expected 12 groups, got %d", tournament.Summary.GroupCount)
	}
	if tournament.Summary.TeamCount != 48 {
		t.Fatalf("expected 48 teams, got %d", tournament.Summary.TeamCount)
	}
	if tournament.Summary.MatchCount != 72 {
		counts := make(map[string]int, len(tournament.Groups))
		for _, group := range tournament.Groups {
			counts[group.Key] = len(group.Matches)
		}
		t.Fatalf("expected 72 matches, got %d: %+v", tournament.Summary.MatchCount, counts)
	}
}

func TestAPIFootballGetRetriesEOF(t *testing.T) {
	attempts := 0
	handler := &Handler{
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					return nil, io.EOF
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"errors":[],"results":0,"response":[]}`)),
				}, nil
			}),
		},
		cfg: Config{
			APIKey:  "test-key",
			BaseURL: "https://example.test",
		},
	}

	var payload apiFootballStandingsResponse
	if err := handler.apiFootballGet(context.Background(), "/standings", url.Values{}, &payload); err != nil {
		t.Fatalf("apiFootballGet: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestNewAPIFootballHTTPClientTransport(t *testing.T) {
	client := newAPIFootballHTTPClient(Config{DisableEnvProxy: true})
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatalf("expected proxy lookup to be disabled")
	}
	if !transport.DisableKeepAlives {
		t.Fatalf("expected keep-alives to be disabled")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

const sampleGroupHTML = `
<div>
  <table class="wikitable sortable">
    <tbody>
      <tr>
        <th>Draw position</th><th>Team</th><th>Pot</th><th>Confederation</th>
        <th>Method of<br>qualification</th><th>Date of<br>qualification</th>
        <th>Finals<br>appearance</th><th>Last<br>appearance</th><th>Previous best<br>performance</th>
        <th colspan="2">FIFA Rankings</th>
      </tr>
      <tr><th>November 2025</th><th>June 2026</th></tr>
      <tr>
        <td>A1</td>
        <td><span><img src="//upload.wikimedia.org/flag.png"></span><a href="/wiki/Mexico_national_football_team" title="Mexico national football team">Mexico</a></td>
        <td>1</td><td>CONCACAF</td><td>Co-host</td><td>February 14, 2023</td>
        <td>18th</td><td>2022</td><td>Quarter-finals</td><td>15</td><td>14</td>
      </tr>
    </tbody>
  </table>
  <table class="wikitable">
    <tbody>
      <tr>
        <th><abbr title="Position">Pos</abbr></th><th>Team</th><th><abbr title="Played">Pld</abbr></th>
        <th>W</th><th>D</th><th>L</th><th>GF</th><th>GA</th><th>GD</th><th>Pts</th>
        <th>Position will qualify for:</th>
      </tr>
      <tr>
        <td>1</td><th><a href="/wiki/Mexico_national_football_team" title="Mexico national football team">Mexico</a></th>
        <td>2</td><td>2</td><td>0</td><td>0</td><td>4</td><td>0</td><td>+4</td><td>6</td><td>Round of 32</td>
      </tr>
    </tbody>
  </table>
  <div itemscope itemtype="http://schema.org/SportsEvent" class="footballbox">
    <div class="fleft"><time><div class="fdate">June 11, 2026<span style="display: none;"><span class="bday dtstart">2026-06-11</span></span></div><div class="ftime">1:00 p.m. UTC-6</div></time></div>
    <table class="fevent"><tbody><tr>
      <th class="fhome" itemprop="homeTeam"><span itemprop="name"><a href="/wiki/Mexico_national_football_team" title="Mexico national football team">Mexico</a></span></th>
      <th class="fscore">2-0</th>
      <th class="faway" itemprop="awayTeam"><span itemprop="name"><a href="/wiki/South_Africa_national_soccer_team" title="South Africa national soccer team">South Africa</a></span></th>
    </tr></tbody></table>
    <div class="fright"><div itemprop="location"><span itemprop="name address">Estadio Azteca, Mexico City</span></div></div>
  </div>
</div>`
