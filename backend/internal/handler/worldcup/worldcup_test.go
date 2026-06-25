package worldcup

import (
	"context"
	"encoding/json"
	"finance-backend/internal/config"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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
	if strings.TrimSpace(cfg.Football.Token) == "" {
		t.Skip("FOOTBALL_DATA_TOKEN is not configured")
	}

	handler := &Handler{
		cfg: Config{
			Token:             cfg.Football.Token,
			BaseURL:           cfg.Football.BaseURL,
			CompetitionCode:   cfg.Football.CompetitionCode,
			Season:            cfg.Football.Season,
			RankingBaseURL:    cfg.Football.RankingBaseURL,
			RankingScheduleID: cfg.Football.RankingScheduleID,
			RankingLocale:     cfg.Football.RankingLocale,
			DisableEnvProxy:   !cfg.Football.UseEnvProxy,
		},
	}
	handler.client = newFootballDataHTTPClient(handler.cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	tournament, err := handler.fetchTournament(ctx)
	if err != nil {
		errorText := strings.ToLower(err.Error())
		if strings.Contains(errorText, "restricted") || strings.Contains(errorText, "paid subscription") || strings.Contains(errorText, "token") {
			t.Skipf("football-data.org token cannot access this resource: %v", err)
		}
		t.Fatalf("fetch tournament: %v", err)
	}
	if tournament.Summary.GroupCount == 0 {
		t.Fatalf("expected groups, got %d", tournament.Summary.GroupCount)
	}
	if tournament.Summary.TeamCount == 0 {
		t.Fatalf("expected teams, got %d", tournament.Summary.TeamCount)
	}
	expectedSource := footballDataName + " + " + fifaRankingName
	if tournament.Source.Name != expectedSource {
		t.Fatalf("expected source %q, got %q", expectedSource, tournament.Source.Name)
	}
}

func TestFootballDataGetRetriesEOF(t *testing.T) {
	attempts := 0
	handler := &Handler{
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				if req.Header.Get("X-Auth-Token") != "test-token" {
					t.Fatalf("expected X-Auth-Token header")
				}
				if attempts == 1 {
					return nil, io.EOF
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"standings":[]}`)),
				}, nil
			}),
		},
		cfg: Config{
			Token:   "test-token",
			BaseURL: "https://example.test",
		},
	}

	var payload footballDataStandingsResponse
	if err := handler.footballDataGet(context.Background(), "/competitions/WC/standings", url.Values{}, &payload); err != nil {
		t.Fatalf("footballDataGet: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestNewFootballDataHTTPClientTransport(t *testing.T) {
	client := newFootballDataHTTPClient(Config{DisableEnvProxy: true})
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

func TestFootballDataSeasonFilterAcceptsStringOrNumber(t *testing.T) {
	for name, body := range map[string]string{
		"number": `{"filters":{"season":2026},"matches":[]}`,
		"string": `{"filters":{"season":"2026"},"matches":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var payload footballDataMatchesResponse
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if payload.Filters.Season.String() != "2026" {
				t.Fatalf("expected 2026, got %q", payload.Filters.Season.String())
			}
		})
	}
}

func TestFootballDataTournamentDerivesGroupsFromMatches(t *testing.T) {
	handler := &Handler{
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				body := `{}`
				switch req.URL.Path {
				case "/competitions/WC/standings":
					body = `{
						"filters":{"season":2026},
						"competition":{"name":"FIFA World Cup","code":"WC"},
						"standings":[{
							"type":"TOTAL",
							"group":null,
							"table":[
								{"position":1,"team":{"id":1,"name":"Alpha","tla":"ALP","crest":"alpha.svg"},"playedGames":0,"won":0,"draw":0,"lost":0,"points":0,"goalsFor":0,"goalsAgainst":0,"goalDifference":0},
								{"position":2,"team":{"id":2,"name":"Beta","tla":"BET","crest":"beta.svg"},"playedGames":0,"won":0,"draw":0,"lost":0,"points":0,"goalsFor":0,"goalsAgainst":0,"goalDifference":0},
								{"position":3,"team":{"id":3,"name":"Gamma","tla":"GAM","crest":"gamma.svg"},"playedGames":0,"won":0,"draw":0,"lost":0,"points":0,"goalsFor":0,"goalsAgainst":0,"goalDifference":0},
								{"position":4,"team":{"id":4,"name":"Delta","tla":"DEL","crest":"delta.svg"},"playedGames":0,"won":0,"draw":0,"lost":0,"points":0,"goalsFor":0,"goalsAgainst":0,"goalDifference":0}
							]
						}]
					}`
				case "/competitions/WC/matches":
					body = `{
						"filters":{"season":2026},
						"competition":{"name":"FIFA World Cup","code":"WC"},
						"matches":[
							{"id":101,"utcDate":"2026-06-11T19:00:00Z","status":"TIMED","stage":"GROUP_STAGE","group":"GROUP_A","venue":"Alpha Stadium","homeTeam":{"id":1,"name":"Alpha","tla":"ALP","crest":"alpha.svg"},"awayTeam":{"id":2,"name":"Beta","tla":"BET","crest":"beta.svg"},"score":{"fullTime":{"home":null,"away":null}}},
							{"id":102,"utcDate":"2026-06-12T19:00:00Z","status":"TIMED","stage":"GROUP_STAGE","group":"GROUP_B","venue":{"name":"Gamma Field","city":"Delta City"},"homeTeam":{"id":3,"name":"Gamma","tla":"GAM","crest":"gamma.svg"},"awayTeam":{"id":4,"name":"Delta","tla":"DEL","crest":"delta.svg"},"score":{"fullTime":{"home":null,"away":null}}},
							{"id":201,"utcDate":"2026-07-02T19:00:00Z","status":"TIMED","stage":"LAST_16","group":"","venue":"Knockout Stadium","homeTeam":{"id":1,"name":"Alpha","tla":"ALP","crest":"alpha.svg"},"awayTeam":{"id":2,"name":"Beta","tla":"BET","crest":"beta.svg"},"score":{"fullTime":{"home":null,"away":null}}}
						]
					}`
				case "/fifarankings/rankings/live":
					if req.Header.Get("X-Auth-Token") != "" {
						t.Fatalf("did not expect X-Auth-Token for FIFA rankings")
					}
					if req.URL.Query().Get("gender") != "1" || req.URL.Query().Get("sportType") != "0" {
						t.Fatalf("unexpected FIFA rankings query: %s", req.URL.RawQuery)
					}
					body = `{
						"Results":[
							{"IdTeam":"9001","TeamName":[{"Locale":"en-GB","Description":"Alpha"}],"ConfederationName":"UEFA","IdCountry":"ALP","Rank":11,"TotalPoints":1000.1},
							{"IdTeam":"9002","TeamName":[{"Locale":"en-GB","Description":"Beta"}],"ConfederationName":"CAF","IdCountry":"BET","Rank":22,"TotalPoints":900.2},
							{"IdTeam":"9003","TeamName":[{"Locale":"en-GB","Description":"Gamma"}],"ConfederationName":"AFC","IdCountry":"GAM","Rank":33,"TotalPoints":800.3},
							{"IdTeam":"9004","TeamName":[{"Locale":"en-GB","Description":"Delta"}],"ConfederationName":"OFC","IdCountry":"DEL","Rank":44,"TotalPoints":700.4}
						]
					}`
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
		cfg: Config{
			Token:           "test-token",
			BaseURL:         "https://example.test",
			CompetitionCode: "WC",
			Season:          2026,
			RankingBaseURL:  "https://ranking.example.test",
			RankingLocale:   "en-GB",
		},
	}

	tournament, err := handler.fetchTournament(context.Background())
	if err != nil {
		t.Fatalf("fetchTournament: %v", err)
	}
	if tournament.Summary.GroupCount != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", tournament.Summary.GroupCount, tournament.Groups)
	}
	for _, group := range tournament.Groups {
		if group.Key == "Unknown" {
			t.Fatalf("did not expect Unknown group: %+v", tournament.Groups)
		}
		if len(group.Teams) != 2 {
			t.Fatalf("expected 2 teams in group %s, got %d", group.Key, len(group.Teams))
		}
		if len(group.Matches) != 1 {
			t.Fatalf("expected 1 match in group %s, got %d", group.Key, len(group.Matches))
		}
	}
	if tournament.Groups[0].Standings[0].WorldRank != 11 {
		t.Fatalf("expected Alpha FIFA rank 11, got %+v", tournament.Groups[0].Standings[0])
	}
	if tournament.Groups[0].Standings[0].Confederation != "UEFA" {
		t.Fatalf("expected Alpha confederation from FIFA ranking, got %+v", tournament.Groups[0].Standings[0])
	}
	if tournament.Groups[0].Matches[0].UTCDate != "2026-06-11T19:00:00Z" {
		t.Fatalf("expected UTC date to be preserved, got %+v", tournament.Groups[0].Matches[0])
	}
	if tournament.Groups[0].Matches[0].Venue != "Alpha Stadium" {
		t.Fatalf("expected football-data venue, got %+v", tournament.Groups[0].Matches[0])
	}
	if tournament.Groups[1].Matches[0].Venue != "Gamma Field, Delta City" {
		t.Fatalf("expected object venue to be normalized, got %+v", tournament.Groups[1].Matches[0])
	}
	if tournament.Summary.MatchCount != 3 || tournament.Summary.KnockoutMatches != 1 {
		t.Fatalf("expected knockout match to be counted, got summary %+v", tournament.Summary)
	}
	if len(tournament.Knockout) != 1 || tournament.Knockout[0].Key != "LAST_16" || tournament.Knockout[0].Label != "16强" {
		t.Fatalf("expected LAST_16 knockout round, got %+v", tournament.Knockout)
	}
	if len(tournament.Knockout[0].Matches) != 1 || tournament.Knockout[0].Matches[0].Venue != "Knockout Stadium" {
		t.Fatalf("expected knockout match with venue, got %+v", tournament.Knockout[0].Matches)
	}
	if tournament.Source.Name != footballDataName+" + "+fifaRankingName {
		t.Fatalf("expected combined source name, got %q", tournament.Source.Name)
	}
}

func TestBuildKnockoutRoundsSortsKnownStages(t *testing.T) {
	rounds := buildKnockoutRounds([]Match{
		{ID: "3", Stage: "FINAL", Date: "2026-07-19", Time: "19:00"},
		{ID: "1", Stage: "LAST_32", Date: "2026-06-28", Time: "19:00"},
		{ID: "2", Stage: "QUARTER_FINALS", Date: "2026-07-09", Time: "19:00"},
	})
	if len(rounds) != 3 {
		t.Fatalf("expected 3 knockout rounds, got %+v", rounds)
	}
	if rounds[0].Key != "LAST_32" || rounds[1].Key != "QUARTER_FINALS" || rounds[2].Key != "FINAL" {
		t.Fatalf("unexpected knockout round order: %+v", rounds)
	}
}

func TestMatchVenueIndexMatchesAliasesAndReverseFixtureOrder(t *testing.T) {
	index := newMatchVenueIndex([]Group{{
		Key: "A",
		Matches: []Match{{
			Group:    "A",
			HomeTeam: "United States",
			AwayTeam: "Iran",
			Venue:    "Test Venue",
		}},
	}})

	match := Match{Group: "A", HomeTeam: "IR Iran", AwayTeam: "USA"}
	if venue := index.find(match); venue != "Test Venue" {
		t.Fatalf("expected alias/reverse venue match, got %q", venue)
	}
}

func TestLoadTournamentRefreshCooldownUsesCache(t *testing.T) {
	var mu sync.Mutex
	standingsRequests := 0
	handler := newFootballDataTestHandler(func(req *http.Request) {
		if req.URL.Path == "/competitions/WC/standings" {
			mu.Lock()
			standingsRequests++
			mu.Unlock()
		}
	})

	if _, err := handler.loadTournament(context.Background(), false); err != nil {
		t.Fatalf("initial loadTournament: %v", err)
	}
	if _, err := handler.loadTournament(context.Background(), true); err != nil {
		t.Fatalf("refresh loadTournament: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if standingsRequests != 1 {
		t.Fatalf("expected refresh cooldown to reuse cache, got %d standings requests", standingsRequests)
	}
}

func TestLoadTournamentDeduplicatesConcurrentFetches(t *testing.T) {
	var mu sync.Mutex
	standingsRequests := 0
	handler := newFootballDataTestHandler(func(req *http.Request) {
		if req.URL.Path == "/competitions/WC/standings" {
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			standingsRequests++
			mu.Unlock()
		}
	})

	const workers = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := handler.loadTournament(context.Background(), false)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("loadTournament: %v", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if standingsRequests != 1 {
		t.Fatalf("expected one upstream standings request, got %d", standingsRequests)
	}
}

func newFootballDataTestHandler(onRequest func(*http.Request)) *Handler {
	return &Handler{
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if onRequest != nil {
					onRequest(req)
				}

				body := `{}`
				switch req.URL.Path {
				case "/competitions/WC/standings":
					body = `{
						"filters":{"season":2026},
						"competition":{"name":"FIFA World Cup","code":"WC"},
						"standings":[{
							"type":"TOTAL",
							"group":"GROUP_A",
							"table":[
								{"position":1,"team":{"id":1,"name":"Alpha","tla":"ALP","crest":"alpha.svg"},"playedGames":0,"won":0,"draw":0,"lost":0,"points":0,"goalsFor":0,"goalsAgainst":0,"goalDifference":0},
								{"position":2,"team":{"id":2,"name":"Beta","tla":"BET","crest":"beta.svg"},"playedGames":0,"won":0,"draw":0,"lost":0,"points":0,"goalsFor":0,"goalsAgainst":0,"goalDifference":0}
							]
						}]
					}`
				case "/competitions/WC/matches":
					body = `{
						"filters":{"season":2026},
						"competition":{"name":"FIFA World Cup","code":"WC"},
						"matches":[{
							"id":101,
							"utcDate":"2026-06-11T19:00:00Z",
							"status":"TIMED",
							"stage":"GROUP_STAGE",
							"group":"GROUP_A",
							"venue":"Alpha Stadium",
							"homeTeam":{"id":1,"name":"Alpha","tla":"ALP","crest":"alpha.svg"},
							"awayTeam":{"id":2,"name":"Beta","tla":"BET","crest":"beta.svg"},
							"score":{"fullTime":{"home":null,"away":null}}
						}]
					}`
				case "/fifarankings/rankings/live":
					body = `{
						"Results":[
							{"IdTeam":"9001","TeamName":[{"Locale":"en-GB","Description":"Alpha"}],"ConfederationName":"UEFA","IdCountry":"ALP","Rank":11,"TotalPoints":1000.1},
							{"IdTeam":"9002","TeamName":[{"Locale":"en-GB","Description":"Beta"}],"ConfederationName":"CAF","IdCountry":"BET","Rank":22,"TotalPoints":900.2}
						]
					}`
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
		cfg: Config{
			Token:           "test-token",
			BaseURL:         "https://example.test",
			CompetitionCode: "WC",
			Season:          2026,
			RankingBaseURL:  "https://ranking.example.test",
			RankingLocale:   "en-GB",
		},
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
