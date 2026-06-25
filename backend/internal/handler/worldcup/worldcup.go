package worldcup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

const (
	competitionName        = "2026 FIFA World Cup"
	footballDataName       = "football-data.org"
	footballDataDocs       = "https://www.football-data.org/documentation/quickstart"
	defaultBaseURL         = "https://api.football-data.org/v4"
	defaultCompetitionCode = "WC"
	fifaRankingName        = "FIFA World Rankings"
	fifaRankingDocs        = "https://inside.fifa.com/fifa-world-ranking/men"
	defaultRankingBaseURL  = "https://api.fifa.com/api/v3"
	defaultRankingLocale   = "en-GB"
	apiFootballName        = "API-FOOTBALL"
	apiFootballDocs        = "https://www.api-football.com/documentation-v3"
	mediaWikiAPI           = "https://en.wikipedia.org/w/api.php"
	cacheTTL               = 15 * time.Minute
	refreshCooldown        = 1 * time.Minute
	requestTimeout         = 15 * time.Second
	tournamentTimeout      = 60 * time.Second
	groupConcurrency       = 1
	apiRetryAttempts       = 3
	apiRetryBackoff        = 250 * time.Millisecond
)

var groupKeys = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}

type Handler struct {
	client *http.Client
	cfg    Config

	mu          sync.Mutex
	cache       *TournamentResponse
	cacheExpiry time.Time
	lastFetchAt time.Time
	inFlight    *tournamentFetchCall
}

type tournamentFetchCall struct {
	done       chan struct{}
	tournament *TournamentResponse
	err        error
}

type Config struct {
	Token             string
	BaseURL           string
	CompetitionCode   string
	Season            int
	RankingBaseURL    string
	RankingScheduleID string
	RankingLocale     string
	DisableEnvProxy   bool

	// Legacy API-Football fields kept only so the old parser can compile while
	// football-data.org is the active source.
	APIKey   string
	LeagueID int
}

func RegisterRoutes(rg *gin.RouterGroup, cfg Config) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if strings.TrimSpace(cfg.CompetitionCode) == "" {
		cfg.CompetitionCode = defaultCompetitionCode
	}
	if strings.TrimSpace(cfg.RankingBaseURL) == "" {
		cfg.RankingBaseURL = defaultRankingBaseURL
	}
	if strings.TrimSpace(cfg.RankingLocale) == "" {
		cfg.RankingLocale = defaultRankingLocale
	}
	if cfg.Season <= 0 {
		cfg.Season = 2026
	}

	h := &Handler{
		client: newFootballDataHTTPClient(cfg),
		cfg:    cfg,
	}

	rg.GET("", h.getTournament)
	rg.GET("/", h.getTournament)
}

type TournamentResponse struct {
	Competition  string            `json:"competition"`
	Season       int               `json:"season"`
	FetchedAt    string            `json:"fetched_at"`
	CacheSeconds int               `json:"cache_seconds"`
	Stale        bool              `json:"stale"`
	Warning      string            `json:"warning,omitempty"`
	Source       TournamentSource  `json:"source"`
	Summary      TournamentSummary `json:"summary"`
	Groups       []Group           `json:"groups"`
	Knockout     []KnockoutRound   `json:"knockout_rounds"`
}

type TournamentSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type TournamentSummary struct {
	GroupCount       int `json:"group_count"`
	TeamCount        int `json:"team_count"`
	MatchCount       int `json:"match_count"`
	KnockoutMatches  int `json:"knockout_matches"`
	FinishedMatches  int `json:"finished_matches"`
	ScheduledMatches int `json:"scheduled_matches"`
}

type Group struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	SourceURL string  `json:"source_url"`
	Teams     []Team  `json:"teams"`
	Standings []Team  `json:"standings"`
	Matches   []Match `json:"matches"`
}

type KnockoutRound struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Matches []Match `json:"matches"`
}

type Team struct {
	Code             string  `json:"code"`
	DrawPosition     string  `json:"draw_position"`
	Name             string  `json:"name"`
	WikiTitle        string  `json:"wiki_title"`
	WikiURL          string  `json:"wiki_url"`
	FlagURL          string  `json:"flag_url"`
	Pot              int     `json:"pot"`
	Confederation    string  `json:"confederation"`
	Qualification    string  `json:"qualification"`
	QualifiedOn      string  `json:"qualified_on"`
	FinalsAppearance string  `json:"finals_appearance"`
	LastAppearance   string  `json:"last_appearance"`
	BestPerformance  string  `json:"best_performance"`
	DrawRank         int     `json:"draw_rank"`
	WorldRank        int     `json:"world_rank"`
	GroupRank        int     `json:"group_rank"`
	Played           int     `json:"played"`
	Won              int     `json:"won"`
	Drawn            int     `json:"drawn"`
	Lost             int     `json:"lost"`
	GoalsFor         int     `json:"goals_for"`
	GoalsAgainst     int     `json:"goals_against"`
	GoalDifference   int     `json:"goal_difference"`
	Points           int     `json:"points"`
	AdvanceNote      string  `json:"advance_note"`
	Schedule         []Match `json:"schedule"`
}

type Match struct {
	ID            string `json:"id"`
	Group         string `json:"group"`
	Stage         string `json:"stage,omitempty"`
	UTCDate       string `json:"utc_date,omitempty"`
	Date          string `json:"date"`
	Time          string `json:"time"`
	HomeTeam      string `json:"home_team"`
	HomeWikiTitle string `json:"home_wiki_title"`
	AwayTeam      string `json:"away_team"`
	AwayWikiTitle string `json:"away_wiki_title"`
	Score         string `json:"score"`
	HomeScore     *int   `json:"home_score,omitempty"`
	AwayScore     *int   `json:"away_score,omitempty"`
	Status        string `json:"status"`
	Venue         string `json:"venue"`
}

type mediaWikiParseResponse struct {
	Parse struct {
		Title string            `json:"title"`
		Text  map[string]string `json:"text"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error,omitempty"`
}

type mediaWikiQueryResponse struct {
	Query struct {
		Pages []struct {
			Title     string `json:"title"`
			Revisions []struct {
				Slots struct {
					Main struct {
						Content string `json:"content"`
					} `json:"main"`
				} `json:"slots"`
			} `json:"revisions"`
		} `json:"pages"`
	} `json:"query"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error,omitempty"`
}

type apiFootballStandingsResponse struct {
	Errors   json.RawMessage `json:"errors"`
	Results  int             `json:"results"`
	Response []struct {
		League struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Season    int    `json:"season"`
			Standings [][]struct {
				Rank int `json:"rank"`
				Team struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
					Logo string `json:"logo"`
				} `json:"team"`
				Points      int    `json:"points"`
				GoalsDiff   int    `json:"goalsDiff"`
				Group       string `json:"group"`
				Form        string `json:"form"`
				Status      string `json:"status"`
				Description string `json:"description"`
				All         struct {
					Played int `json:"played"`
					Win    int `json:"win"`
					Draw   int `json:"draw"`
					Lose   int `json:"lose"`
					Goals  struct {
						For     int `json:"for"`
						Against int `json:"against"`
					} `json:"goals"`
				} `json:"all"`
			} `json:"standings"`
		} `json:"league"`
	} `json:"response"`
}

type apiFootballFixturesResponse struct {
	Errors   json.RawMessage `json:"errors"`
	Results  int             `json:"results"`
	Response []struct {
		Fixture struct {
			ID       int    `json:"id"`
			Date     string `json:"date"`
			Timezone string `json:"timezone"`
			Status   struct {
				Long  string `json:"long"`
				Short string `json:"short"`
			} `json:"status"`
			Venue struct {
				Name string `json:"name"`
				City string `json:"city"`
			} `json:"venue"`
		} `json:"fixture"`
		League struct {
			Round string `json:"round"`
		} `json:"league"`
		Teams struct {
			Home struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				Logo   string `json:"logo"`
				Winner *bool  `json:"winner"`
			} `json:"home"`
			Away struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				Logo   string `json:"logo"`
				Winner *bool  `json:"winner"`
			} `json:"away"`
		} `json:"teams"`
		Goals struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"goals"`
	} `json:"response"`
}

type footballDataStandingsResponse struct {
	Filters struct {
		Season flexibleJSONValue `json:"season"`
	} `json:"filters"`
	Competition struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"competition"`
	Season struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	} `json:"season"`
	Standings []struct {
		Stage string `json:"stage"`
		Type  string `json:"type"`
		Group string `json:"group"`
		Table []struct {
			Position       int                 `json:"position"`
			Team           footballDataTeamDTO `json:"team"`
			PlayedGames    int                 `json:"playedGames"`
			Form           string              `json:"form"`
			Won            int                 `json:"won"`
			Draw           int                 `json:"draw"`
			Lost           int                 `json:"lost"`
			Points         int                 `json:"points"`
			GoalsFor       int                 `json:"goalsFor"`
			GoalsAgainst   int                 `json:"goalsAgainst"`
			GoalDifference int                 `json:"goalDifference"`
		} `json:"table"`
	} `json:"standings"`
}

type footballDataMatchesResponse struct {
	Filters struct {
		Season flexibleJSONValue `json:"season"`
	} `json:"filters"`
	Competition struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"competition"`
	Matches []footballDataMatchDTO `json:"matches"`
}

type footballDataTeamDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	TLA       string `json:"tla"`
	Crest     string `json:"crest"`
}

type footballDataMatchDTO struct {
	ID       int                 `json:"id"`
	UTCDate  string              `json:"utcDate"`
	Status   string              `json:"status"`
	Stage    string              `json:"stage"`
	Group    string              `json:"group"`
	Venue    footballDataVenue   `json:"venue"`
	HomeTeam footballDataTeamDTO `json:"homeTeam"`
	AwayTeam footballDataTeamDTO `json:"awayTeam"`
	Score    struct {
		Winner   string `json:"winner"`
		Duration string `json:"duration"`
		FullTime struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"fullTime"`
	} `json:"score"`
}

type fifaRankingResponse struct {
	Results []fifaRankingEntry `json:"Results"`
}

type fifaRankingEntry struct {
	IDTeam            string                 `json:"IdTeam"`
	TeamName          []localizedDescription `json:"TeamName"`
	ConfederationName string                 `json:"ConfederationName"`
	IDCountry         string                 `json:"IdCountry"`
	Rank              int                    `json:"Rank"`
	TotalPoints       float64                `json:"TotalPoints"`
}

type localizedDescription struct {
	Locale      string `json:"Locale"`
	Description string `json:"Description"`
}

type fifaRankingIndex struct {
	byCode map[string]fifaRankingEntry
	byName map[string]fifaRankingEntry
}

type flexibleJSONValue string

func (value *flexibleJSONValue) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*value = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = flexibleJSONValue(text)
		return nil
	}

	*value = flexibleJSONValue(trimmed)
	return nil
}

func (value flexibleJSONValue) String() string {
	return string(value)
}

type footballDataVenue string

func (value *footballDataVenue) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*value = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = footballDataVenue(strings.TrimSpace(text))
		return nil
	}

	var object map[string]interface{}
	if err := json.Unmarshal(data, &object); err == nil {
		parts := nonEmptyStrings(
			stringFromMap(object, "name"),
			stringFromMap(object, "stadium"),
			stringFromMap(object, "city"),
		)
		*value = footballDataVenue(strings.Join(parts, ", "))
		return nil
	}

	*value = footballDataVenue(trimmed)
	return nil
}

func (value footballDataVenue) String() string {
	return string(value)
}

type wikiTeam struct {
	Team
	code string
}

type wikiStanding struct {
	Team
	code string
}

type wikiTeamLink struct {
	name  string
	title string
	url   string
}

type tableCell struct {
	Node *html.Node
	Text string
}

type teamRef struct {
	groupKey  string
	teamIndex int
}

type matchVenueIndex map[string]string

func (h *Handler) getTournament(c *gin.Context) {
	refresh := strings.EqualFold(strings.TrimSpace(c.Query("refresh")), "1")
	tournament, err := h.loadTournament(c.Request.Context(), refresh)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tournament)
}

func (h *Handler) loadTournament(ctx context.Context, refresh bool) (*TournamentResponse, error) {
	now := time.Now()

	h.mu.Lock()
	if !refresh && h.cache != nil && now.Before(h.cacheExpiry) {
		cached := cloneTournament(h.cache)
		h.mu.Unlock()
		return cached, nil
	}
	if refresh && h.cache != nil && !h.lastFetchAt.IsZero() && now.Sub(h.lastFetchAt) < refreshCooldown {
		cached := cloneTournament(h.cache)
		if now.After(h.cacheExpiry) {
			cached.Stale = true
			cached.Warning = "refresh skipped because upstream data was requested recently"
		}
		h.mu.Unlock()
		return cached, nil
	}
	stale := cloneTournament(h.cache)
	if h.inFlight != nil {
		call := h.inFlight
		h.mu.Unlock()
		return h.waitForTournamentFetch(ctx, call, stale)
	}
	call := &tournamentFetchCall{done: make(chan struct{})}
	h.inFlight = call
	h.lastFetchAt = now
	h.mu.Unlock()

	tournament, err := h.fetchTournament(ctx)
	h.mu.Lock()
	call.tournament = cloneTournament(tournament)
	call.err = err
	if err == nil {
		h.cache = cloneTournament(tournament)
		h.cacheExpiry = time.Now().Add(cacheTTL)
	}
	if h.inFlight == call {
		h.inFlight = nil
	}
	close(call.done)
	h.mu.Unlock()

	if err != nil {
		if stale != nil {
			stale.Stale = true
			stale.Warning = err.Error()
			return stale, nil
		}
		return nil, err
	}

	return tournament, nil
}

func (h *Handler) waitForTournamentFetch(ctx context.Context, call *tournamentFetchCall, stale *TournamentResponse) (*TournamentResponse, error) {
	select {
	case <-call.done:
		h.mu.Lock()
		tournament := cloneTournament(call.tournament)
		err := call.err
		h.mu.Unlock()
		if err != nil {
			if stale != nil {
				stale.Stale = true
				stale.Warning = err.Error()
				return stale, nil
			}
			return nil, err
		}
		return tournament, nil
	case <-ctx.Done():
		if stale != nil {
			stale.Stale = true
			stale.Warning = ctx.Err().Error()
			return stale, nil
		}
		return nil, ctx.Err()
	}
}

func (h *Handler) fetchTournament(ctx context.Context) (*TournamentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, tournamentTimeout)
	defer cancel()

	return h.fetchTournamentFromFootballData(ctx)
}

func (h *Handler) fetchTournamentFromFootballData(ctx context.Context) (*TournamentResponse, error) {
	var standingsPayload footballDataStandingsResponse
	if err := h.footballDataGet(ctx, fmt.Sprintf("/competitions/%s/standings", url.PathEscape(h.cfg.CompetitionCode)), url.Values{
		"season": []string{strconv.Itoa(h.cfg.Season)},
	}, &standingsPayload); err != nil {
		return nil, err
	}
	if len(standingsPayload.Standings) == 0 {
		return nil, errors.New("football-data.org returned no World Cup standings")
	}

	var matchesPayload footballDataMatchesResponse
	if err := h.footballDataGet(ctx, fmt.Sprintf("/competitions/%s/matches", url.PathEscape(h.cfg.CompetitionCode)), url.Values{
		"season": []string{strconv.Itoa(h.cfg.Season)},
	}, &matchesPayload); err != nil {
		return nil, err
	}

	var warnings []string
	rankings, err := h.fetchFIFARankings(ctx)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s unavailable: %v", fifaRankingName, err))
	}

	var venueIndex matchVenueIndex
	if footballDataNeedsVenueFallback(matchesPayload.Matches) {
		if index, err := h.fetchWikiMatchVenueIndex(ctx); err == nil {
			venueIndex = index
		} else {
			warnings = append(warnings, fmt.Sprintf("match venues unavailable: %v", err))
		}
	}

	groupMap := make(map[string]*Group)
	teamRefs := make(map[int]teamRef)
	standingsByTeamID := make(map[int]Team)
	for _, standing := range standingsPayload.Standings {
		if !strings.EqualFold(standing.Type, "TOTAL") {
			continue
		}
		groupKey := groupKeyFromText(standing.Group)
		hasStandingGroup := groupKey != ""
		for _, row := range standing.Table {
			team := Team{
				Code:           footballDataTeamCode(row.Team),
				Name:           footballDataTeamName(row.Team),
				FlagURL:        row.Team.Crest,
				Played:         row.PlayedGames,
				Won:            row.Won,
				Drawn:          row.Draw,
				Lost:           row.Lost,
				GoalsFor:       row.GoalsFor,
				GoalsAgainst:   row.GoalsAgainst,
				GoalDifference: row.GoalDifference,
				Points:         row.Points,
			}
			if hasStandingGroup {
				team.GroupRank = row.Position
				group := ensureFootballDataGroup(groupMap, groupKey, standing.Group)
				group.Teams = append(group.Teams, team)
				teamRefs[row.Team.ID] = teamRef{groupKey: groupKey, teamIndex: len(group.Teams) - 1}
			}
			standingsByTeamID[row.Team.ID] = team
		}
	}

	knockoutMatches := make([]Match, 0)
	for _, fixture := range matchesPayload.Matches {
		if !isFootballDataGroupStage(fixture.Stage) {
			if isFootballDataKnockoutStage(fixture.Stage) {
				match := footballDataMatchToMatch(knockoutStageKey(fixture.Stage), fixture)
				knockoutMatches = append(knockoutMatches, match)
			}
			continue
		}
		groupKey := groupKeyFromText(fixture.Group)
		if groupKey == "" {
			groupKey = groupKeyForFixture(fixture.Group, fixture.HomeTeam.ID, fixture.AwayTeam.ID, teamRefs)
		}
		if groupKey == "" {
			continue
		}
		group := ensureFootballDataGroup(groupMap, groupKey, fixture.Group)
		homeRef := ensureFootballDataTeamInGroup(group, teamRefs, standingsByTeamID, fixture.HomeTeam, groupKey)
		awayRef := ensureFootballDataTeamInGroup(group, teamRefs, standingsByTeamID, fixture.AwayTeam, groupKey)
		match := footballDataMatchToMatch(groupKey, fixture)
		if match.Venue == "" {
			match.Venue = venueIndex.find(match)
		}
		group.Matches = append(group.Matches, match)
		group.Teams[homeRef.teamIndex].Schedule = append(group.Teams[homeRef.teamIndex].Schedule, match)
		group.Teams[awayRef.teamIndex].Schedule = append(group.Teams[awayRef.teamIndex].Schedule, match)
	}

	groups := make([]Group, 0, len(groupMap))
	for _, group := range groupMap {
		sortFootballDataTeams(group.Teams)
		if rankings != nil {
			applyFIFARankings(group.Teams, rankings)
		}
		group.Standings = append([]Team(nil), group.Teams...)
		sort.SliceStable(group.Matches, func(i, j int) bool {
			if group.Matches[i].Date == group.Matches[j].Date {
				return group.Matches[i].Time < group.Matches[j].Time
			}
			return group.Matches[i].Date < group.Matches[j].Date
		})
		groups = append(groups, *group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Key < groups[j].Key
	})
	knockoutRounds := buildKnockoutRounds(knockoutMatches)

	summary := TournamentSummary{
		GroupCount: len(groups),
	}
	for _, group := range groups {
		summary.TeamCount += len(group.Teams)
		summary.MatchCount += len(group.Matches)
		for _, match := range group.Matches {
			if match.Status == "finished" {
				summary.FinishedMatches++
			} else {
				summary.ScheduledMatches++
			}
		}
	}
	for _, round := range knockoutRounds {
		summary.KnockoutMatches += len(round.Matches)
		summary.MatchCount += len(round.Matches)
		for _, match := range round.Matches {
			if match.Status == "finished" {
				summary.FinishedMatches++
			} else {
				summary.ScheduledMatches++
			}
		}
	}

	season := h.cfg.Season
	if standingsPayload.Filters.Season.String() != "" {
		if parsedSeason, err := strconv.Atoi(standingsPayload.Filters.Season.String()); err == nil {
			season = parsedSeason
		}
	}

	return &TournamentResponse{
		Competition:  firstNonEmpty(standingsPayload.Competition.Name, competitionName),
		Season:       season,
		FetchedAt:    time.Now().Format(time.RFC3339),
		CacheSeconds: int(cacheTTL.Seconds()),
		Warning:      strings.Join(warnings, "; "),
		Source: TournamentSource{
			Name: footballDataName + " + " + fifaRankingName,
			URL:  fifaRankingDocs,
		},
		Summary:  summary,
		Groups:   groups,
		Knockout: knockoutRounds,
	}, nil
}

func (h *Handler) fetchTournamentFromAPIFootball(ctx context.Context) (*TournamentResponse, error) {
	var standingsPayload apiFootballStandingsResponse
	if err := h.apiFootballGet(ctx, "/standings", url.Values{
		"league": []string{strconv.Itoa(h.cfg.LeagueID)},
		"season": []string{strconv.Itoa(h.cfg.Season)},
	}, &standingsPayload); err != nil {
		return nil, err
	}
	if message := apiFootballErrorMessage(standingsPayload.Errors); message != "" {
		return nil, errors.New(message)
	}
	if len(standingsPayload.Response) == 0 || len(standingsPayload.Response[0].League.Standings) == 0 {
		return nil, errors.New("api-football returned no World Cup standings")
	}

	var fixturesPayload apiFootballFixturesResponse
	if err := h.apiFootballGet(ctx, "/fixtures", url.Values{
		"league": []string{strconv.Itoa(h.cfg.LeagueID)},
		"season": []string{strconv.Itoa(h.cfg.Season)},
	}, &fixturesPayload); err != nil {
		return nil, err
	}
	if message := apiFootballErrorMessage(fixturesPayload.Errors); message != "" {
		return nil, errors.New(message)
	}

	groupMap := make(map[string]*Group)
	teamRefs := make(map[int]teamRef)
	for _, table := range standingsPayload.Response[0].League.Standings {
		for _, row := range table {
			groupKey := groupKeyFromText(row.Group)
			if groupKey == "" {
				groupKey = row.Group
			}
			group := ensureAPIGroup(groupMap, groupKey, row.Group)
			team := Team{
				Code:           strconv.Itoa(row.Team.ID),
				Name:           row.Team.Name,
				FlagURL:        row.Team.Logo,
				GroupRank:      row.Rank,
				Played:         row.All.Played,
				Won:            row.All.Win,
				Drawn:          row.All.Draw,
				Lost:           row.All.Lose,
				GoalsFor:       row.All.Goals.For,
				GoalsAgainst:   row.All.Goals.Against,
				GoalDifference: row.GoalsDiff,
				Points:         row.Points,
				AdvanceNote:    row.Description,
			}
			group.Teams = append(group.Teams, team)
			teamRefs[row.Team.ID] = teamRef{groupKey: groupKey, teamIndex: len(group.Teams) - 1}
		}
	}

	for _, fixture := range fixturesPayload.Response {
		groupKey := groupKeyForFixture(fixture.League.Round, fixture.Teams.Home.ID, fixture.Teams.Away.ID, teamRefs)
		if groupKey == "" {
			continue
		}
		group := ensureAPIGroup(groupMap, groupKey, "Group "+groupKey)
		match := apiFixtureToMatch(groupKey, fixture)
		group.Matches = append(group.Matches, match)
		if ref, ok := teamRefs[fixture.Teams.Home.ID]; ok && ref.groupKey == groupKey {
			group.Teams[ref.teamIndex].Schedule = append(group.Teams[ref.teamIndex].Schedule, match)
		}
		if ref, ok := teamRefs[fixture.Teams.Away.ID]; ok && ref.groupKey == groupKey {
			group.Teams[ref.teamIndex].Schedule = append(group.Teams[ref.teamIndex].Schedule, match)
		}
	}

	groups := make([]Group, 0, len(groupMap))
	for _, group := range groupMap {
		sort.SliceStable(group.Teams, func(i, j int) bool {
			if group.Teams[i].GroupRank == group.Teams[j].GroupRank {
				return group.Teams[i].Name < group.Teams[j].Name
			}
			return group.Teams[i].GroupRank < group.Teams[j].GroupRank
		})
		group.Standings = append([]Team(nil), group.Teams...)
		sort.SliceStable(group.Matches, func(i, j int) bool {
			if group.Matches[i].Date == group.Matches[j].Date {
				return group.Matches[i].Time < group.Matches[j].Time
			}
			return group.Matches[i].Date < group.Matches[j].Date
		})
		groups = append(groups, *group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Key < groups[j].Key
	})

	summary := TournamentSummary{
		GroupCount: len(groups),
	}
	for _, group := range groups {
		summary.TeamCount += len(group.Teams)
		summary.MatchCount += len(group.Matches)
		for _, match := range group.Matches {
			if match.Status == "finished" {
				summary.FinishedMatches++
			} else {
				summary.ScheduledMatches++
			}
		}
	}

	season := h.cfg.Season
	if standingsPayload.Response[0].League.Season > 0 {
		season = standingsPayload.Response[0].League.Season
	}

	return &TournamentResponse{
		Competition:  competitionName,
		Season:       season,
		FetchedAt:    time.Now().Format(time.RFC3339),
		CacheSeconds: int(cacheTTL.Seconds()),
		Source: TournamentSource{
			Name: apiFootballName,
			URL:  apiFootballDocs,
		},
		Summary: summary,
		Groups:  groups,
	}, nil
}

func (h *Handler) fetchTournamentFromMediaWiki(ctx context.Context) (*TournamentResponse, error) {
	type groupResult struct {
		index int
		group Group
		err   error
	}

	results := make(chan groupResult, len(groupKeys))
	sem := make(chan struct{}, groupConcurrency)
	for index, key := range groupKeys {
		go func(index int, key string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			group, err := h.fetchGroup(ctx, key)
			results <- groupResult{index: index, group: group, err: err}
		}(index, key)
	}

	groups := make([]Group, len(groupKeys))
	var errs []string
	for range groupKeys {
		result := <-results
		if result.err != nil {
			errs = append(errs, fmt.Sprintf("Group %s: %v", groupKeys[result.index], result.err))
			continue
		}
		groups[result.index] = result.group
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	summary := TournamentSummary{
		GroupCount: len(groups),
	}
	for _, group := range groups {
		summary.TeamCount += len(group.Teams)
		summary.MatchCount += len(group.Matches)
		for _, match := range group.Matches {
			if match.Status == "finished" {
				summary.FinishedMatches++
			} else {
				summary.ScheduledMatches++
			}
		}
	}

	return &TournamentResponse{
		Competition:  competitionName,
		Season:       2026,
		FetchedAt:    time.Now().Format(time.RFC3339),
		CacheSeconds: int(cacheTTL.Seconds()),
		Source: TournamentSource{
			Name: "Wikipedia MediaWiki API",
			URL:  mediaWikiAPI,
		},
		Summary: summary,
		Groups:  groups,
	}, nil
}

func (h *Handler) footballDataGet(ctx context.Context, path string, params url.Values, target interface{}) error {
	var lastErr error
	for attempt := 1; attempt <= apiRetryAttempts; attempt++ {
		err := h.footballDataGetOnce(ctx, path, params, target)
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == apiRetryAttempts || !isTransientSourceError(err) {
			break
		}

		delay := time.Duration(attempt) * apiRetryBackoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("football-data.org %s request failed: %w", strings.TrimPrefix(path, "/"), lastErr)
}

func (h *Handler) footballDataGetOnce(ctx context.Context, path string, params url.Values, target interface{}) error {
	token := strings.TrimSpace(h.cfg.Token)
	if token == "" {
		return errors.New("FOOTBALL_DATA_TOKEN is not configured")
	}

	endpoint, err := url.Parse(strings.TrimRight(h.cfg.BaseURL, "/") + path)
	if err != nil {
		return err
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finance-app-worldcup/1.0")
	req.Header.Set("X-Auth-Token", token)

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sourceHTTPError{status: resp.StatusCode, detail: readHTTPErrorDetail(resp)}
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func newFootballDataHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.DisableEnvProxy {
		transport.Proxy = nil
	}
	transport.DisableKeepAlives = true

	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}
}

func (h *Handler) fetchFIFARankings(ctx context.Context) (*fifaRankingIndex, error) {
	var payload fifaRankingResponse
	locale := firstNonEmpty(h.cfg.RankingLocale, defaultRankingLocale)
	if scheduleID := strings.TrimSpace(h.cfg.RankingScheduleID); scheduleID != "" {
		if err := h.fifaRankingGet(ctx, "/fifarankings/rankings/rankingsbyschedule", url.Values{
			"rankingScheduleId": []string{scheduleID},
			"language":          []string{locale},
		}, &payload); err != nil {
			return nil, err
		}
	} else if err := h.fifaRankingGet(ctx, "/fifarankings/rankings/live", url.Values{
		"gender":    []string{"1"},
		"sportType": []string{"0"},
		"language":  []string{locale},
	}, &payload); err != nil {
		return nil, err
	}
	if len(payload.Results) == 0 {
		return nil, errors.New("FIFA rankings returned no teams")
	}
	return newFIFARankingIndex(payload.Results), nil
}

func (h *Handler) fifaRankingGet(ctx context.Context, path string, params url.Values, target interface{}) error {
	var lastErr error
	for attempt := 1; attempt <= apiRetryAttempts; attempt++ {
		err := h.fifaRankingGetOnce(ctx, path, params, target)
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == apiRetryAttempts || !isTransientSourceError(err) {
			break
		}

		delay := time.Duration(attempt) * apiRetryBackoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("FIFA rankings %s request failed: %w", strings.TrimPrefix(path, "/"), lastErr)
}

func (h *Handler) fifaRankingGetOnce(ctx context.Context, path string, params url.Values, target interface{}) error {
	endpoint, err := url.Parse(strings.TrimRight(firstNonEmpty(h.cfg.RankingBaseURL, defaultRankingBaseURL), "/") + path)
	if err != nil {
		return err
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finance-app-worldcup/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sourceHTTPError{status: resp.StatusCode, detail: readHTTPErrorDetail(resp)}
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (h *Handler) apiFootballGet(ctx context.Context, path string, params url.Values, target interface{}) error {
	var lastErr error
	for attempt := 1; attempt <= apiRetryAttempts; attempt++ {
		err := h.apiFootballGetOnce(ctx, path, params, target)
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == apiRetryAttempts || !isTransientSourceError(err) {
			break
		}

		delay := time.Duration(attempt) * apiRetryBackoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("api-football %s request failed: %w", strings.TrimPrefix(path, "/"), lastErr)
}

func (h *Handler) apiFootballGetOnce(ctx context.Context, path string, params url.Values, target interface{}) error {
	apiKey := strings.TrimSpace(h.cfg.APIKey)
	if apiKey == "" {
		return errors.New("FOOTBALL_API_KEY is not configured")
	}

	endpoint, err := url.Parse(strings.TrimRight(h.cfg.BaseURL, "/") + path)
	if err != nil {
		return err
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finance-app-worldcup/1.0")
	req.Header.Set("x-apisports-key", apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sourceHTTPError{status: resp.StatusCode, detail: readHTTPErrorDetail(resp)}
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func newAPIFootballHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.DisableEnvProxy {
		transport.Proxy = nil
	}
	// This endpoint is queried infrequently, so avoiding reused connections
	// makes transient EOFs from stale proxy/TLS sockets much less likely.
	transport.DisableKeepAlives = true

	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}
}

func isTransientSourceError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var httpErr sourceHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status == http.StatusTooManyRequests || httpErr.status >= http.StatusInternalServerError
	}
	return false
}

func readHTTPErrorDetail(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}

	var payload struct {
		Errors  json.RawMessage `json:"errors"`
		Message string          `json:"message"`
		Error   string          `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if message := apiFootballErrorMessage(payload.Errors); message != "" {
			return message
		}
		if strings.TrimSpace(payload.Message) != "" {
			return strings.TrimSpace(payload.Message)
		}
		if strings.TrimSpace(payload.Error) != "" {
			return strings.TrimSpace(payload.Error)
		}
	}

	return text
}

func apiFootballErrorMessage(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" || trimmed == "{}" {
		return ""
	}

	var object map[string]string
	if err := json.Unmarshal(raw, &object); err == nil {
		var parts []string
		for key, value := range object {
			if value == "" {
				parts = append(parts, key)
				continue
			}
			parts = append(parts, fmt.Sprintf("%s: %s", key, value))
		}
		sort.Strings(parts)
		return strings.Join(parts, "; ")
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return strings.Join(list, "; ")
	}

	return trimmed
}

func ensureAPIGroup(groups map[string]*Group, key string, label string) *Group {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "Unknown"
	}
	if group, ok := groups[key]; ok {
		return group
	}
	if strings.TrimSpace(label) == "" {
		label = "Group " + key
	}
	group := &Group{
		Key:       key,
		Label:     apiGroupLabel(key, label),
		SourceURL: apiFootballDocs,
	}
	groups[key] = group
	return group
}

func ensureFootballDataGroup(groups map[string]*Group, key string, label string) *Group {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "Unknown"
	}
	if group, ok := groups[key]; ok {
		return group
	}
	if strings.TrimSpace(label) == "" {
		label = "Group " + key
	}
	group := &Group{
		Key:       key,
		Label:     apiGroupLabel(key, label),
		SourceURL: footballDataDocs,
	}
	groups[key] = group
	return group
}

func apiGroupLabel(key string, fallback string) string {
	if len([]rune(key)) == 1 && key >= "A" && key <= "Z" {
		return key + "组"
	}
	return fallback
}

func groupKeyForFixture(round string, homeID int, awayID int, refs map[int]teamRef) string {
	if key := groupKeyFromText(round); key != "" {
		return key
	}
	homeRef, hasHome := refs[homeID]
	awayRef, hasAway := refs[awayID]
	if hasHome && hasAway && homeRef.groupKey == awayRef.groupKey {
		return homeRef.groupKey
	}
	if hasHome {
		return homeRef.groupKey
	}
	if hasAway {
		return awayRef.groupKey
	}
	return ""
}

func groupKeyFromText(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if strings.HasPrefix(normalized, "GROUP_") {
		return strings.TrimPrefix(normalized, "GROUP_")
	}
	if strings.HasPrefix(normalized, "GROUP ") {
		return strings.TrimSpace(strings.TrimPrefix(normalized, "GROUP "))
	}
	match := groupTextPattern.FindStringSubmatch(value)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func ensureFootballDataTeamInGroup(group *Group, refs map[int]teamRef, standings map[int]Team, dto footballDataTeamDTO, groupKey string) teamRef {
	if ref, ok := refs[dto.ID]; ok && ref.groupKey == groupKey {
		return ref
	}

	code := footballDataTeamCode(dto)
	for index, team := range group.Teams {
		if team.Code == code || team.Name == footballDataTeamName(dto) {
			ref := teamRef{groupKey: groupKey, teamIndex: index}
			refs[dto.ID] = ref
			return ref
		}
	}

	team, ok := standings[dto.ID]
	if !ok {
		team = Team{
			Code:    code,
			Name:    footballDataTeamName(dto),
			FlagURL: dto.Crest,
		}
	} else {
		team.Code = firstNonEmpty(team.Code, code)
		team.Name = firstNonEmpty(team.Name, footballDataTeamName(dto))
		team.FlagURL = firstNonEmpty(team.FlagURL, dto.Crest)
	}

	group.Teams = append(group.Teams, team)
	ref := teamRef{groupKey: groupKey, teamIndex: len(group.Teams) - 1}
	refs[dto.ID] = ref
	return ref
}

func sortFootballDataTeams(teams []Team) {
	hasRank := false
	for _, team := range teams {
		if team.GroupRank > 0 {
			hasRank = true
			break
		}
	}
	sort.SliceStable(teams, func(i, j int) bool {
		if hasRank {
			if teams[i].GroupRank == 0 {
				return false
			}
			if teams[j].GroupRank == 0 {
				return true
			}
			if teams[i].GroupRank != teams[j].GroupRank {
				return teams[i].GroupRank < teams[j].GroupRank
			}
		}
		if teams[i].Points != teams[j].Points {
			return teams[i].Points > teams[j].Points
		}
		if teams[i].GoalDifference != teams[j].GoalDifference {
			return teams[i].GoalDifference > teams[j].GoalDifference
		}
		if teams[i].GoalsFor != teams[j].GoalsFor {
			return teams[i].GoalsFor > teams[j].GoalsFor
		}
		return teams[i].Name < teams[j].Name
	})
	if !hasRank {
		for index := range teams {
			teams[index].GroupRank = index + 1
		}
	}
}

func footballDataTeamName(team footballDataTeamDTO) string {
	return firstNonEmpty(team.Name, team.ShortName, team.TLA)
}

func footballDataTeamCode(team footballDataTeamDTO) string {
	return firstNonEmpty(team.TLA, strconv.Itoa(team.ID))
}

func footballDataMatchToMatch(groupKey string, fixture footballDataMatchDTO) Match {
	date, matchTime, utcDate := footballDataDateTime(fixture.UTCDate)
	status := "scheduled"
	score := ""
	if isFootballDataFinished(fixture.Status) && fixture.Score.FullTime.Home != nil && fixture.Score.FullTime.Away != nil {
		status = "finished"
		score = fmt.Sprintf("%d-%d", *fixture.Score.FullTime.Home, *fixture.Score.FullTime.Away)
	}
	return Match{
		ID:            strconv.Itoa(fixture.ID),
		Group:         groupKey,
		Stage:         strings.ToUpper(strings.TrimSpace(fixture.Stage)),
		UTCDate:       utcDate,
		Date:          date,
		Time:          matchTime,
		HomeTeam:      footballDataTeamName(fixture.HomeTeam),
		HomeWikiTitle: strconv.Itoa(fixture.HomeTeam.ID),
		AwayTeam:      footballDataTeamName(fixture.AwayTeam),
		AwayWikiTitle: strconv.Itoa(fixture.AwayTeam.ID),
		Score:         score,
		HomeScore:     fixture.Score.FullTime.Home,
		AwayScore:     fixture.Score.FullTime.Away,
		Status:        status,
		Venue:         fixture.Venue.String(),
	}
}

func footballDataDateTime(value string) (string, string, string) {
	return utcDateTimeParts(value)
}

func utcDateTimeParts(value string) (string, string, string) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		if len(value) >= 16 {
			return value[:10], value[11:16], value
		}
		return value, "", value
	}
	utc := parsed.UTC()
	return utc.Format("2006-01-02"), utc.Format("15:04"), utc.Format(time.RFC3339)
}

func isFootballDataFinished(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "FINISHED")
}

func isFootballDataGroupStage(stage string) bool {
	return strings.EqualFold(strings.TrimSpace(stage), "GROUP_STAGE")
}

func isFootballDataKnockoutStage(stage string) bool {
	key := knockoutStageKey(stage)
	return key != "" && key != "GROUP_STAGE"
}

func buildKnockoutRounds(matches []Match) []KnockoutRound {
	if len(matches) == 0 {
		return nil
	}

	roundMap := make(map[string][]Match)
	for _, match := range matches {
		key := knockoutStageKey(firstNonEmpty(match.Stage, match.Group))
		if key == "" {
			key = "KNOCKOUT"
		}
		match.Group = key
		match.Stage = key
		roundMap[key] = append(roundMap[key], match)
	}

	keys := make([]string, 0, len(roundMap))
	for key := range roundMap {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left := knockoutStageOrder(keys[i])
		right := knockoutStageOrder(keys[j])
		if left == right {
			return keys[i] < keys[j]
		}
		return left < right
	})

	rounds := make([]KnockoutRound, 0, len(keys))
	for _, key := range keys {
		roundMatches := roundMap[key]
		sort.SliceStable(roundMatches, func(i, j int) bool {
			if roundMatches[i].Date == roundMatches[j].Date {
				return roundMatches[i].Time < roundMatches[j].Time
			}
			return roundMatches[i].Date < roundMatches[j].Date
		})
		rounds = append(rounds, KnockoutRound{
			Key:     key,
			Label:   knockoutStageLabel(key),
			Matches: roundMatches,
		})
	}
	return rounds
}

func knockoutStageKey(stage string) string {
	normalized := strings.ToUpper(strings.TrimSpace(stage))
	normalized = strings.NewReplacer("-", "_", " ", "_").Replace(normalized)
	switch normalized {
	case "", "GROUP_STAGE":
		return normalized
	case "LAST_32", "ROUND_OF_32", "ROUND_32", "R32":
		return "LAST_32"
	case "LAST_16", "ROUND_OF_16", "ROUND_16", "R16":
		return "LAST_16"
	case "QUARTER_FINALS", "QUARTER_FINAL", "QUARTERFINALS", "QUARTERFINAL":
		return "QUARTER_FINALS"
	case "SEMI_FINALS", "SEMI_FINAL", "SEMIFINALS", "SEMIFINAL":
		return "SEMI_FINALS"
	case "THIRD_PLACE", "THIRD_PLACE_PLAYOFF", "PLAY_OFF_FOR_THIRD_PLACE", "PLAYOFF_FOR_THIRD_PLACE":
		return "THIRD_PLACE"
	case "FINAL", "FINALS":
		return "FINAL"
	default:
		return normalized
	}
}

func knockoutStageOrder(stage string) int {
	switch knockoutStageKey(stage) {
	case "LAST_32":
		return 10
	case "LAST_16":
		return 20
	case "QUARTER_FINALS":
		return 30
	case "SEMI_FINALS":
		return 40
	case "THIRD_PLACE":
		return 50
	case "FINAL":
		return 60
	default:
		return 100
	}
}

func knockoutStageLabel(stage string) string {
	switch knockoutStageKey(stage) {
	case "LAST_32":
		return "32强"
	case "LAST_16":
		return "16强"
	case "QUARTER_FINALS":
		return "1/4 决赛"
	case "SEMI_FINALS":
		return "半决赛"
	case "THIRD_PLACE":
		return "季军赛"
	case "FINAL":
		return "决赛"
	default:
		value := strings.ReplaceAll(strings.TrimSpace(stage), "_", " ")
		if value == "" {
			return "淘汰赛"
		}
		return strings.Title(strings.ToLower(value))
	}
}

func newFIFARankingIndex(entries []fifaRankingEntry) *fifaRankingIndex {
	index := &fifaRankingIndex{
		byCode: make(map[string]fifaRankingEntry, len(entries)),
		byName: make(map[string]fifaRankingEntry, len(entries)),
	}
	for _, entry := range entries {
		if entry.Rank <= 0 {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(entry.IDCountry))
		if code != "" {
			index.byCode[code] = entry
		}
		for _, name := range fifaRankingEntryNames(entry) {
			if key := normalizeTeamLookupKey(name); key != "" {
				index.byName[key] = entry
			}
		}
	}
	return index
}

func applyFIFARankings(teams []Team, rankings *fifaRankingIndex) {
	if rankings == nil {
		return
	}
	for index := range teams {
		entry, ok := rankings.find(teams[index])
		if !ok {
			continue
		}
		teams[index].WorldRank = entry.Rank
		teams[index].Confederation = firstNonEmpty(teams[index].Confederation, entry.ConfederationName)
		teams[index].Code = firstNonEmpty(teams[index].Code, entry.IDCountry)
	}
}

func (rankings *fifaRankingIndex) find(team Team) (fifaRankingEntry, bool) {
	if rankings == nil {
		return fifaRankingEntry{}, false
	}
	if code := strings.ToUpper(strings.TrimSpace(team.Code)); code != "" {
		if entry, ok := rankings.byCode[code]; ok {
			return entry, true
		}
	}
	for _, name := range teamLookupNames(team.Name) {
		if entry, ok := rankings.byName[normalizeTeamLookupKey(name)]; ok {
			return entry, true
		}
	}
	return fifaRankingEntry{}, false
}

func fifaRankingEntryNames(entry fifaRankingEntry) []string {
	names := make([]string, 0, len(entry.TeamName)+1)
	for _, name := range entry.TeamName {
		if strings.TrimSpace(name.Description) != "" {
			names = append(names, name.Description)
		}
	}
	if entry.IDCountry != "" {
		names = append(names, entry.IDCountry)
	}
	return names
}

func teamLookupNames(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	names := []string{name}
	switch normalizeTeamLookupKey(name) {
	case "united states", "usa", "us":
		names = append(names, "USA")
	case "south korea", "korea republic":
		names = append(names, "Korea Republic")
	case "iran", "ir iran":
		names = append(names, "IR Iran")
	case "ivory coast", "cote divoire", "côte divoire":
		names = append(names, "Côte d'Ivoire")
	case "turkey", "turkiye", "türkiye":
		names = append(names, "Türkiye")
	}
	return names
}

func normalizeTeamLookupKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		"&", " and ",
		"'", "",
		".", " ",
		"-", " ",
		"’", "",
		"côte", "cote",
		"türkiye", "turkiye",
	).Replace(value)

	var builder strings.Builder
	lastSpace := true
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func apiFixtureToMatch(groupKey string, fixture struct {
	Fixture struct {
		ID       int    `json:"id"`
		Date     string `json:"date"`
		Timezone string `json:"timezone"`
		Status   struct {
			Long  string `json:"long"`
			Short string `json:"short"`
		} `json:"status"`
		Venue struct {
			Name string `json:"name"`
			City string `json:"city"`
		} `json:"venue"`
	} `json:"fixture"`
	League struct {
		Round string `json:"round"`
	} `json:"league"`
	Teams struct {
		Home struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Logo   string `json:"logo"`
			Winner *bool  `json:"winner"`
		} `json:"home"`
		Away struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Logo   string `json:"logo"`
			Winner *bool  `json:"winner"`
		} `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home *int `json:"home"`
		Away *int `json:"away"`
	} `json:"goals"`
}) Match {
	date, matchTime, utcDate := apiFixtureDateTime(fixture.Fixture.Date)
	status := "scheduled"
	score := ""
	if isAPIFootballFinished(fixture.Fixture.Status.Short) && fixture.Goals.Home != nil && fixture.Goals.Away != nil {
		status = "finished"
		score = fmt.Sprintf("%d-%d", *fixture.Goals.Home, *fixture.Goals.Away)
	}
	venue := strings.TrimSpace(strings.Join(nonEmptyStrings(fixture.Fixture.Venue.Name, fixture.Fixture.Venue.City), ", "))
	return Match{
		ID:            strconv.Itoa(fixture.Fixture.ID),
		Group:         groupKey,
		Stage:         strings.ToUpper(strings.TrimSpace(fixture.League.Round)),
		UTCDate:       utcDate,
		Date:          date,
		Time:          matchTime,
		HomeTeam:      fixture.Teams.Home.Name,
		HomeWikiTitle: strconv.Itoa(fixture.Teams.Home.ID),
		AwayTeam:      fixture.Teams.Away.Name,
		AwayWikiTitle: strconv.Itoa(fixture.Teams.Away.ID),
		Score:         score,
		HomeScore:     fixture.Goals.Home,
		AwayScore:     fixture.Goals.Away,
		Status:        status,
		Venue:         venue,
	}
}

func apiFixtureDateTime(value string) (string, string, string) {
	return utcDateTimeParts(value)
}

func isAPIFootballFinished(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FT", "AET", "PEN":
		return true
	default:
		return false
	}
}

func nonEmptyStrings(values ...string) []string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return parts
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func stringFromMap(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func footballDataNeedsVenueFallback(matches []footballDataMatchDTO) bool {
	for _, match := range matches {
		if isFootballDataGroupStage(match.Stage) && match.Venue.String() == "" {
			return true
		}
	}
	return false
}

func (h *Handler) fetchWikiMatchVenueIndex(ctx context.Context) (matchVenueIndex, error) {
	tournament, err := h.fetchTournamentFromWikiText(ctx)
	if err != nil {
		return nil, err
	}
	return newMatchVenueIndex(tournament.Groups), nil
}

func newMatchVenueIndex(groups []Group) matchVenueIndex {
	index := make(matchVenueIndex)
	for _, group := range groups {
		for _, match := range group.Matches {
			venue := strings.TrimSpace(match.Venue)
			if venue == "" {
				continue
			}
			for _, key := range matchVenueKeys(group.Key, match.HomeTeam, match.AwayTeam) {
				index[key] = venue
			}
			for _, key := range matchVenueKeys(group.Key, match.AwayTeam, match.HomeTeam) {
				index[key] = venue
			}
		}
	}
	return index
}

func (index matchVenueIndex) find(match Match) string {
	if len(index) == 0 {
		return ""
	}
	for _, key := range matchVenueKeys(match.Group, match.HomeTeam, match.AwayTeam) {
		if venue := strings.TrimSpace(index[key]); venue != "" {
			return venue
		}
	}
	return ""
}

func matchVenueKeys(groupKey string, homeTeam string, awayTeam string) []string {
	homeNames := teamLookupNames(homeTeam)
	if len(homeNames) == 0 {
		homeNames = []string{homeTeam}
	}
	awayNames := teamLookupNames(awayTeam)
	if len(awayNames) == 0 {
		awayNames = []string{awayTeam}
	}

	keys := make([]string, 0, len(homeNames)*len(awayNames))
	group := strings.ToUpper(strings.TrimSpace(groupKey))
	for _, home := range homeNames {
		home = normalizeTeamLookupKey(home)
		if home == "" {
			continue
		}
		for _, away := range awayNames {
			away = normalizeTeamLookupKey(away)
			if away == "" {
				continue
			}
			keys = append(keys, group+"|"+home+"|"+away)
		}
	}
	return keys
}

func (h *Handler) fetchTournamentFromWikiText(ctx context.Context) (*TournamentResponse, error) {
	pages, err := h.fetchWikiTextPages(ctx)
	if err != nil {
		return nil, err
	}
	extraTitles := discoverTranscludedWikiPages(pages)
	if len(extraTitles) > 0 {
		extraPages, err := h.fetchWikiTextTitles(ctx, extraTitles)
		if err != nil {
			return nil, err
		}
		for title, content := range extraPages {
			pages[title] = content
		}
	}

	standingsByGroup := parseWikiStandings(pages["Template:2026 FIFA World Cup group tables"])
	groups := make([]Group, 0, len(groupKeys))
	for _, key := range groupKeys {
		pageTitle := fmt.Sprintf("2026 FIFA World Cup Group %s", key)
		pageText := pages[pageTitle]
		if strings.TrimSpace(pageText) == "" {
			return nil, fmt.Errorf("empty wikitext for %s", pageTitle)
		}

		wikiTeams := parseWikiTeams(pageText)
		if len(wikiTeams) == 0 {
			return nil, fmt.Errorf("no teams parsed for Group %s", key)
		}

		teams := make([]Team, len(wikiTeams))
		codeIndex := make(map[string]int, len(wikiTeams))
		for index, wikiTeam := range wikiTeams {
			teams[index] = wikiTeam.Team
			codeIndex[wikiTeam.code] = index
		}

		for _, title := range extractTranscludedPageTitles(pageText) {
			if extraText := pages[title]; extraText != "" {
				pageText += "\n" + extraText
			}
		}
		matches := parseWikiMatches(pageText, key, wikiTeams)
		for _, standing := range standingsByGroup[key] {
			index, ok := codeIndex[standing.code]
			if !ok {
				continue
			}
			teams[index].GroupRank = standing.GroupRank
			teams[index].Played = standing.Played
			teams[index].Won = standing.Won
			teams[index].Drawn = standing.Drawn
			teams[index].Lost = standing.Lost
			teams[index].GoalsFor = standing.GoalsFor
			teams[index].GoalsAgainst = standing.GoalsAgainst
			teams[index].GoalDifference = standing.GoalDifference
			teams[index].Points = standing.Points
			teams[index].AdvanceNote = standing.AdvanceNote
		}

		for _, match := range matches {
			if index, ok := codeIndex[match.HomeWikiTitle]; ok {
				teams[index].Schedule = append(teams[index].Schedule, match)
			}
			if index, ok := codeIndex[match.AwayWikiTitle]; ok {
				teams[index].Schedule = append(teams[index].Schedule, match)
			}
		}

		standings := append([]Team(nil), teams...)
		sort.SliceStable(standings, func(i, j int) bool {
			if standings[i].GroupRank == 0 {
				return false
			}
			if standings[j].GroupRank == 0 {
				return true
			}
			return standings[i].GroupRank < standings[j].GroupRank
		})

		groups = append(groups, Group{
			Key:       key,
			Label:     key + "组",
			SourceURL: "https://en.wikipedia.org/wiki/" + strings.ReplaceAll(pageTitle, " ", "_"),
			Teams:     teams,
			Standings: standings,
			Matches:   matches,
		})
	}

	summary := TournamentSummary{
		GroupCount: len(groups),
	}
	for _, group := range groups {
		summary.TeamCount += len(group.Teams)
		summary.MatchCount += len(group.Matches)
		for _, match := range group.Matches {
			if match.Status == "finished" {
				summary.FinishedMatches++
			} else {
				summary.ScheduledMatches++
			}
		}
	}

	return &TournamentResponse{
		Competition:  competitionName,
		Season:       2026,
		FetchedAt:    time.Now().Format(time.RFC3339),
		CacheSeconds: int(cacheTTL.Seconds()),
		Source: TournamentSource{
			Name: "Wikipedia MediaWiki API",
			URL:  mediaWikiAPI,
		},
		Summary: summary,
		Groups:  groups,
	}, nil
}

func (h *Handler) fetchWikiTextPages(ctx context.Context) (map[string]string, error) {
	titles := make([]string, 0, len(groupKeys)+1)
	for _, key := range groupKeys {
		titles = append(titles, fmt.Sprintf("2026 FIFA World Cup Group %s", key))
	}
	titles = append(titles, "Template:2026 FIFA World Cup group tables")
	return h.fetchWikiTextTitles(ctx, titles)
}

func (h *Handler) fetchWikiTextTitles(ctx context.Context, titles []string) (map[string]string, error) {
	endpoint, err := url.Parse(mediaWikiAPI)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("action", "query")
	query.Set("format", "json")
	query.Set("formatversion", "2")
	query.Set("prop", "revisions")
	query.Set("rvprop", "content")
	query.Set("rvslots", "main")
	query.Set("titles", strings.Join(titles, "|"))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finance-app-worldcup/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, sourceHTTPError{status: resp.StatusCode}
	}

	var payload mediaWikiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Error != nil {
		return nil, fmt.Errorf("%s: %s", payload.Error.Code, payload.Error.Info)
	}

	pages := make(map[string]string, len(payload.Query.Pages))
	for _, page := range payload.Query.Pages {
		if len(page.Revisions) == 0 {
			continue
		}
		pages[page.Title] = page.Revisions[0].Slots.Main.Content
	}
	if len(pages) < len(titles) {
		return nil, fmt.Errorf("expected %d wiki pages, got %d", len(titles), len(pages))
	}
	return pages, nil
}

func (h *Handler) fetchGroup(ctx context.Context, key string) (Group, error) {
	pageTitle := fmt.Sprintf("2026 FIFA World Cup Group %s", key)
	htmlText, err := h.fetchPageHTMLWithRetry(ctx, pageTitle)
	if err != nil {
		return Group{}, err
	}

	doc, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return Group{}, fmt.Errorf("parse html: %w", err)
	}

	teams := parseTeams(doc)
	standingRows := parseStandings(doc)
	matches := parseMatches(doc, key)

	if len(teams) == 0 {
		return Group{}, errors.New("no teams parsed")
	}
	if len(standingRows) == 0 {
		return Group{}, errors.New("no standings parsed")
	}

	teamIndex := make(map[string]int, len(teams)*2)
	for index, team := range teams {
		for _, key := range teamKeys(team.Name, team.WikiTitle) {
			teamIndex[key] = index
		}
	}

	for _, standing := range standingRows {
		index, ok := findTeamIndex(teamIndex, standing.Name, standing.WikiTitle)
		if !ok {
			continue
		}
		teams[index].GroupRank = standing.GroupRank
		teams[index].Played = standing.Played
		teams[index].Won = standing.Won
		teams[index].Drawn = standing.Drawn
		teams[index].Lost = standing.Lost
		teams[index].GoalsFor = standing.GoalsFor
		teams[index].GoalsAgainst = standing.GoalsAgainst
		teams[index].GoalDifference = standing.GoalDifference
		teams[index].Points = standing.Points
		teams[index].AdvanceNote = standing.AdvanceNote
	}

	for _, match := range matches {
		if index, ok := findTeamIndex(teamIndex, match.HomeTeam, match.HomeWikiTitle); ok {
			teams[index].Schedule = append(teams[index].Schedule, match)
		}
		if index, ok := findTeamIndex(teamIndex, match.AwayTeam, match.AwayWikiTitle); ok {
			teams[index].Schedule = append(teams[index].Schedule, match)
		}
	}

	standings := append([]Team(nil), teams...)
	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].GroupRank == 0 {
			return false
		}
		if standings[j].GroupRank == 0 {
			return true
		}
		return standings[i].GroupRank < standings[j].GroupRank
	})

	return Group{
		Key:       key,
		Label:     key + "组",
		SourceURL: "https://en.wikipedia.org/wiki/" + strings.ReplaceAll(pageTitle, " ", "_"),
		Teams:     teams,
		Standings: standings,
		Matches:   matches,
	}, nil
}

func (h *Handler) fetchPageHTML(ctx context.Context, pageTitle string) (string, error) {
	endpoint, err := url.Parse(mediaWikiAPI)
	if err != nil {
		return "", err
	}

	query := endpoint.Query()
	query.Set("action", "parse")
	query.Set("format", "json")
	query.Set("page", pageTitle)
	query.Set("prop", "text")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "finance-app-worldcup/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", sourceHTTPError{status: resp.StatusCode}
	}

	var payload mediaWikiParseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Error != nil {
		return "", fmt.Errorf("%s: %s", payload.Error.Code, payload.Error.Info)
	}

	htmlText := payload.Parse.Text["*"]
	if strings.TrimSpace(htmlText) == "" {
		return "", errors.New("empty page html")
	}
	return htmlText, nil
}

func (h *Handler) fetchPageHTMLWithRetry(ctx context.Context, pageTitle string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		htmlText, err := h.fetchPageHTML(ctx, pageTitle)
		if err == nil {
			return htmlText, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		delay := time.Duration(attempt+1) * 500 * time.Millisecond
		var httpErr sourceHTTPError
		if errors.As(err, &httpErr) && httpErr.status == http.StatusTooManyRequests {
			delay = time.Duration(attempt+1) * 2 * time.Second
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", lastErr
}

type sourceHTTPError struct {
	status int
	detail string
}

func (err sourceHTTPError) Error() string {
	if strings.TrimSpace(err.detail) != "" {
		return fmt.Sprintf("source returned HTTP %d: %s", err.status, err.detail)
	}
	return fmt.Sprintf("source returned HTTP %d", err.status)
}

func parseTeams(doc *html.Node) []Team {
	for _, table := range findAll(doc, "table") {
		rows := tableRows(table)
		if !isTeamsTable(rows) {
			continue
		}

		var teams []Team
		for _, row := range rows {
			if len(row) < 11 || !drawPositionPattern.MatchString(row[0].Text) {
				continue
			}

			name, wikiTitle, wikiURL := firstLink(row[1].Node)
			if name == "" {
				name = row[1].Text
			}

			teams = append(teams, Team{
				DrawPosition:     row[0].Text,
				Name:             name,
				WikiTitle:        wikiTitle,
				WikiURL:          wikiURL,
				FlagURL:          firstImageURL(row[1].Node),
				Pot:              parseInt(row[2].Text),
				Confederation:    row[3].Text,
				Qualification:    row[4].Text,
				QualifiedOn:      row[5].Text,
				FinalsAppearance: row[6].Text,
				LastAppearance:   row[7].Text,
				BestPerformance:  row[8].Text,
				DrawRank:         parseInt(row[9].Text),
				WorldRank:        parseInt(row[10].Text),
			})
		}
		return teams
	}
	return nil
}

func parseStandings(doc *html.Node) []Team {
	for _, table := range findAll(doc, "table") {
		rows := tableRows(table)
		if !isStandingsTable(rows) {
			continue
		}

		var standings []Team
		for _, row := range rows {
			if len(row) < 10 || parseInt(row[0].Text) <= 0 {
				continue
			}

			name, wikiTitle, wikiURL := firstLink(row[1].Node)
			if name == "" {
				name = stripParenthetical(row[1].Text)
			}

			standing := Team{
				Name:           name,
				WikiTitle:      wikiTitle,
				WikiURL:        wikiURL,
				FlagURL:        firstImageURL(row[1].Node),
				GroupRank:      parseInt(row[0].Text),
				Played:         parseInt(row[2].Text),
				Won:            parseInt(row[3].Text),
				Drawn:          parseInt(row[4].Text),
				Lost:           parseInt(row[5].Text),
				GoalsFor:       parseInt(row[6].Text),
				GoalsAgainst:   parseInt(row[7].Text),
				GoalDifference: parseInt(row[8].Text),
				Points:         parseInt(row[9].Text),
			}
			if len(row) > 10 {
				standing.AdvanceNote = row[10].Text
			}
			standings = append(standings, standing)
		}
		return standings
	}
	return nil
}

func parseMatches(doc *html.Node, groupKey string) []Match {
	boxes := findAllByClass(doc, "footballbox")
	matches := make([]Match, 0, len(boxes))
	for _, box := range boxes {
		homeNode := findFirstByClass(box, "fhome")
		awayNode := findFirstByClass(box, "faway")
		scoreNode := findFirstByClass(box, "fscore")
		if homeNode == nil || awayNode == nil || scoreNode == nil {
			continue
		}

		home, homeWikiTitle, _ := firstLink(homeNode)
		away, awayWikiTitle, _ := firstLink(awayNode)
		score := cleanText(textContent(scoreNode))
		homeScore, awayScore, finished := parseScore(score)
		status := "scheduled"
		if finished {
			status = "finished"
		}

		date := ""
		if dateNode := findFirstByClass(box, "dtstart"); dateNode != nil {
			date = cleanText(textContent(dateNode))
		}
		if date == "" {
			date = cleanText(textContent(findFirstByClass(box, "fdate")))
		}

		matchTime := cleanText(textContent(findFirstByClass(box, "ftime")))
		venue := ""
		if locationNode := findFirstByItemprop(box, "location"); locationNode != nil {
			venue = cleanText(textContent(locationNode))
		}

		match := Match{
			ID:            slug(strings.Join([]string{groupKey, date, home, away}, "-")),
			Group:         groupKey,
			Date:          date,
			Time:          matchTime,
			HomeTeam:      home,
			HomeWikiTitle: homeWikiTitle,
			AwayTeam:      away,
			AwayWikiTitle: awayWikiTitle,
			Score:         score,
			HomeScore:     homeScore,
			AwayScore:     awayScore,
			Status:        status,
			Venue:         venue,
		}
		matches = append(matches, match)
	}
	return matches
}

func parseWikiTeams(pageText string) []wikiTeam {
	teamLinks := parseIntroTeamLinks(pageText)
	teamSection := sectionBetween(pageText, "==Teams==", "==Standings==")
	var teams []wikiTeam
	for _, line := range strings.Split(teamSection, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| ") || !strings.Contains(line, "||") {
			continue
		}
		cells := splitWikiTableRow(line)
		if len(cells) < 11 || !drawPositionPattern.MatchString(cells[0]) {
			continue
		}

		code := extractFlagCode(cells[1])
		link := wikiTeamLink{name: code}
		if len(teams) < len(teamLinks) {
			link = teamLinks[len(teams)]
		}
		if link.name == "" {
			link.name = code
		}

		team := Team{
			Code:             code,
			DrawPosition:     cells[0],
			Name:             link.name,
			WikiTitle:        link.title,
			WikiURL:          link.url,
			Pot:              parseInt(cleanWikiText(cells[2])),
			Confederation:    cleanWikiText(cells[3]),
			Qualification:    cleanWikiText(cells[4]),
			QualifiedOn:      cleanWikiText(cells[5]),
			FinalsAppearance: cleanWikiText(cells[6]),
			LastAppearance:   cleanWikiText(cells[7]),
			BestPerformance:  cleanWikiText(cells[8]),
			DrawRank:         parseInt(cleanWikiText(cells[9])),
			WorldRank:        parseInt(cleanWikiText(cells[10])),
		}
		teams = append(teams, wikiTeam{Team: team, code: code})
	}
	return teams
}

func parseIntroTeamLinks(pageText string) []wikiTeamLink {
	intro := pageText
	if index := strings.Index(pageText, "==Teams=="); index >= 0 {
		intro = pageText[:index]
	}

	matches := teamLinkPattern.FindAllStringSubmatch(intro, -1)
	links := make([]wikiTeamLink, 0, 4)
	seen := map[string]struct{}{}
	for _, match := range matches {
		title := strings.TrimSpace(match[1])
		name := strings.TrimSpace(match[2])
		if name == "" {
			name = strings.TrimSuffix(title, " national football team")
			name = strings.TrimSuffix(name, " national soccer team")
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		links = append(links, wikiTeamLink{
			name:  name,
			title: title,
			url:   "https://en.wikipedia.org/wiki/" + strings.ReplaceAll(title, " ", "_"),
		})
		if len(links) == 4 {
			break
		}
	}
	return links
}

func discoverTranscludedWikiPages(pages map[string]string) []string {
	seen := map[string]struct{}{}
	var titles []string
	for _, content := range pages {
		for _, title := range extractTranscludedPageTitles(content) {
			if _, ok := seen[title]; ok {
				continue
			}
			seen[title] = struct{}{}
			titles = append(titles, title)
		}
	}
	sort.Strings(titles)
	return titles
}

func extractTranscludedPageTitles(pageText string) []string {
	matches := listTransclusionPattern.FindAllStringSubmatch(pageText, -1)
	titles := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		title := strings.TrimSpace(match[1])
		if title == "" {
			continue
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		titles = append(titles, title)
	}
	return titles
}

func parseWikiStandings(templateText string) map[string][]wikiStanding {
	standingsByGroup := make(map[string][]wikiStanding, len(groupKeys))
	for index, key := range groupKeys {
		startMarker := "|Group " + key + "="
		endMarker := ""
		if index+1 < len(groupKeys) {
			endMarker = "\n|Group " + groupKeys[index+1] + "="
		} else {
			endMarker = "\n|3rd place="
		}
		section := sectionBetween(templateText, startMarker, endMarker)
		if section == "" {
			continue
		}

		params := parseWikiParams(section)
		order := splitCodes(params["team_order"])
		rows := make([]wikiStanding, 0, len(order))
		for position, code := range order {
			won := parseInt(params["win_"+code])
			drawn := parseInt(params["draw_"+code])
			lost := parseInt(params["loss_"+code])
			goalsFor := parseInt(params["gf_"+code])
			goalsAgainst := parseInt(params["ga_"+code])
			resultCode := params[fmt.Sprintf("result%d", position+1)]
			rows = append(rows, wikiStanding{
				code: code,
				Team: Team{
					Code:           code,
					GroupRank:      position + 1,
					Played:         won + drawn + lost,
					Won:            won,
					Drawn:          drawn,
					Lost:           lost,
					GoalsFor:       goalsFor,
					GoalsAgainst:   goalsAgainst,
					GoalDifference: goalsFor - goalsAgainst,
					Points:         won*3 + drawn,
					AdvanceNote:    wikiResultText(resultCode),
				},
			})
		}
		standingsByGroup[key] = rows
	}
	return standingsByGroup
}

func parseWikiMatches(pageText string, groupKey string, teams []wikiTeam) []Match {
	codeMap := make(map[string]wikiTeam, len(teams))
	for _, team := range teams {
		codeMap[team.code] = team
	}

	templates := extractFootballBoxTemplates(pageText)
	matches := make([]Match, 0, len(templates))
	for _, templateText := range templates {
		params := parseFootballBoxParams(templateText)
		homeCode := extractFlagCode(params["team1"])
		awayCode := extractFlagCode(params["team2"])
		if homeCode == "" || awayCode == "" {
			continue
		}

		home := codeMap[homeCode]
		away := codeMap[awayCode]
		score := cleanWikiText(params["score"])
		homeScore, awayScore, finished := parseScore(score)
		status := "scheduled"
		if finished {
			status = "finished"
		} else {
			score = ""
		}

		date := parseWikiStartDate(params["date"])
		matchTime := cleanWikiText(params["time"])
		venue := cleanWikiText(params["stadium"])
		matches = append(matches, Match{
			ID:            slug(strings.Join([]string{groupKey, date, homeCode, awayCode}, "-")),
			Group:         groupKey,
			Date:          date,
			Time:          matchTime,
			HomeTeam:      home.Name,
			HomeWikiTitle: homeCode,
			AwayTeam:      away.Name,
			AwayWikiTitle: awayCode,
			Score:         score,
			HomeScore:     homeScore,
			AwayScore:     awayScore,
			Status:        status,
			Venue:         venue,
		})
	}
	return matches
}

func splitWikiTableRow(line string) []string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "|"))
	parts := strings.Split(line, "||")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, stripWikiCellAttrs(strings.TrimSpace(part)))
	}
	return cells
}

func stripWikiCellAttrs(value string) string {
	for {
		before, after, ok := strings.Cut(value, " | ")
		if !ok {
			return strings.TrimSpace(value)
		}
		before = strings.TrimSpace(before)
		if strings.Contains(before, "=") || strings.HasPrefix(before, "style") || strings.HasPrefix(before, "data-") {
			value = strings.TrimSpace(after)
			continue
		}
		return strings.TrimSpace(value)
	}
}

func parseWikiParams(section string) map[string]string {
	params := make(map[string]string)
	for _, line := range strings.Split(section, "\n") {
		for _, match := range wikiParamPattern.FindAllStringSubmatch(line, -1) {
			params[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
		}
	}
	return params
}

func parseFootballBoxParams(templateText string) map[string]string {
	params := make(map[string]string)
	for _, line := range strings.Split(templateText, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "|"), "=")
		if !ok {
			continue
		}
		params[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return params
}

func splitCodes(value string) []string {
	rawCodes := strings.Split(value, ",")
	codes := make([]string, 0, len(rawCodes))
	for _, code := range rawCodes {
		code = strings.TrimSpace(code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func wikiResultText(code string) string {
	switch strings.TrimSpace(code) {
	case "KO":
		return "Knockout stage"
	case "3rd":
		return "Possible knockout stage based on ranking"
	default:
		return ""
	}
}

func parseWikiStartDate(value string) string {
	match := startDatePattern.FindStringSubmatch(value)
	if len(match) != 4 {
		return cleanWikiText(value)
	}
	year, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func extractFootballBoxTemplates(pageText string) []string {
	const marker = "{{#invoke:football box|main"
	var templates []string
	searchFrom := 0
	for {
		start := strings.Index(pageText[searchFrom:], marker)
		if start < 0 {
			break
		}
		start += searchFrom
		depth := 0
		for index := start; index < len(pageText)-1; index++ {
			switch pageText[index : index+2] {
			case "{{":
				depth++
				index++
			case "}}":
				depth--
				index++
				if depth == 0 {
					templates = append(templates, pageText[start:index+1])
					searchFrom = index + 1
					goto nextTemplate
				}
			}
		}
		break
	nextTemplate:
	}
	return templates
}

func extractFlagCode(value string) string {
	matches := flagCodePattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}

func cleanWikiText(value string) string {
	value = strings.ReplaceAll(value, "&nbsp;", " ")
	value = strings.ReplaceAll(value, "<br />", " ")
	value = strings.ReplaceAll(value, "<br/>", " ")
	value = strings.ReplaceAll(value, "{{!}}", "|")
	value = htmlCommentPattern.ReplaceAllString(value, "")
	if index := strings.Index(value, "{{refn"); index >= 0 {
		value = value[:index]
	}
	value = refPattern.ReplaceAllString(value, "")
	value = selfClosingRefPattern.ReplaceAllString(value, "")
	for {
		next := wikiLinkPattern.ReplaceAllString(value, "$2")
		next = wikiSimpleLinkPattern.ReplaceAllString(next, "$1")
		if next == value {
			break
		}
		value = next
	}
	for {
		next := simpleTemplatePattern.ReplaceAllString(value, "")
		if next == value {
			break
		}
		value = next
	}
	value = strings.ReplaceAll(value, "'''", "")
	value = strings.ReplaceAll(value, "''", "")
	value = stdhtml.UnescapeString(value)
	return cleanText(value)
}

func sectionBetween(value string, startMarker string, endMarker string) string {
	start := strings.Index(value, startMarker)
	if start < 0 {
		return ""
	}
	start += len(startMarker)
	rest := value[start:]
	if endMarker == "" {
		return rest
	}
	if end := strings.Index(rest, endMarker); end >= 0 {
		return rest[:end]
	}
	return rest
}

func isTeamsTable(rows [][]tableCell) bool {
	header := tableHeaderText(rows)
	return strings.Contains(header, "Draw position") &&
		strings.Contains(header, "FIFA Rankings") &&
		strings.Contains(header, "Method of qualification")
}

func isStandingsTable(rows [][]tableCell) bool {
	header := tableHeaderText(rows)
	return strings.Contains(header, "Pld") &&
		strings.Contains(header, "GF") &&
		strings.Contains(header, "GA") &&
		strings.Contains(header, "GD") &&
		strings.Contains(header, "Pts")
}

func tableHeaderText(rows [][]tableCell) string {
	var parts []string
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		if parseInt(row[0].Text) > 0 || drawPositionPattern.MatchString(row[0].Text) {
			break
		}
		for _, cell := range row {
			parts = append(parts, cell.Text)
		}
	}
	return strings.Join(parts, " ")
}

func tableRows(table *html.Node) [][]tableCell {
	var rows [][]tableCell
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "tr" {
			var row []tableCell
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.ElementNode && (child.Data == "td" || child.Data == "th") {
					row = append(row, tableCell{
						Node: child,
						Text: cleanText(textContent(child)),
					})
				}
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(table)
	return rows
}

func findAll(root *html.Node, tag string) []*html.Node {
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == tag {
			nodes = append(nodes, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return nodes
}

func findAllByClass(root *html.Node, className string) []*html.Node {
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && hasClass(node, className) {
			nodes = append(nodes, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return nodes
}

func findFirstByClass(root *html.Node, className string) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && hasClass(root, className) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstByClass(child, className); found != nil {
			return found
		}
	}
	return nil
}

func findFirstByItemprop(root *html.Node, itemprop string) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode {
		for _, attr := range root.Attr {
			if attr.Key == "itemprop" && attr.Val == itemprop {
				return root
			}
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstByItemprop(child, itemprop); found != nil {
			return found
		}
	}
	return nil
}

func hasClass(node *html.Node, className string) bool {
	for _, attr := range node.Attr {
		if attr.Key != "class" {
			continue
		}
		for _, token := range strings.Fields(attr.Val) {
			if token == className {
				return true
			}
		}
	}
	return false
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}

	var buf bytes.Buffer
	var walk func(*html.Node, bool)
	walk = func(current *html.Node, hidden bool) {
		if current.Type == html.ElementNode {
			switch current.Data {
			case "style", "script", "sup":
				return
			}
			if style := attrValue(current, "style"); strings.Contains(strings.ToLower(style), "display: none") {
				hidden = true
			}
		}

		if current.Type == html.TextNode && !hidden {
			buf.WriteString(current.Data)
			buf.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child, hidden)
		}
	}
	walk(node, false)
	return buf.String()
}

func firstLink(node *html.Node) (text string, wikiTitle string, href string) {
	if node == nil {
		return "", "", ""
	}
	if node.Type == html.ElementNode && node.Data == "a" {
		linkText := cleanText(textContent(node))
		linkTitle := attrValue(node, "title")
		linkHref := normalizeWikiURL(attrValue(node, "href"))
		if linkText != "" {
			return linkText, linkTitle, linkHref
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if text, title, href := firstLink(child); text != "" {
			return text, title, href
		}
	}
	return "", "", ""
}

func firstImageURL(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.ElementNode && node.Data == "img" {
		return normalizeImageURL(attrValue(node, "src"))
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if src := firstImageURL(child); src != "" {
			return src
		}
	}
	return ""
}

func attrValue(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func normalizeWikiURL(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return "https://en.wikipedia.org" + href
	}
	return href
}

func normalizeImageURL(src string) string {
	if src == "" {
		return ""
	}
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}
	if strings.HasPrefix(src, "/") {
		return "https://en.wikipedia.org" + src
	}
	return src
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	fields := strings.Fields(value)
	return strings.TrimSpace(strings.Join(fields, " "))
}

func stripParenthetical(value string) string {
	if index := strings.Index(value, "("); index > 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}

func parseInt(value string) int {
	replacer := strings.NewReplacer(",", "", "+", "", "−", "-", "–", "-", "—", "")
	cleaned := replacer.Replace(cleanText(value))
	match := intPattern.FindString(cleaned)
	if match == "" {
		return 0
	}
	parsed, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return parsed
}

func parseScore(score string) (*int, *int, bool) {
	match := scorePattern.FindStringSubmatch(score)
	if len(match) != 3 {
		return nil, nil, false
	}
	home, errHome := strconv.Atoi(match[1])
	away, errAway := strconv.Atoi(match[2])
	if errHome != nil || errAway != nil {
		return nil, nil, false
	}
	return &home, &away, true
}

func findTeamIndex(index map[string]int, name string, wikiTitle string) (int, bool) {
	for _, key := range teamKeys(name, wikiTitle) {
		if teamIndex, ok := index[key]; ok {
			return teamIndex, true
		}
	}
	return 0, false
}

func teamKeys(name string, wikiTitle string) []string {
	keys := make([]string, 0, 4)
	for _, value := range []string{name, wikiTitle} {
		key := normalizeTeamKey(value)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func normalizeTeamKey(value string) string {
	value = strings.ToLower(value)
	value = strings.TrimSuffix(value, " national football team")
	value = strings.TrimSuffix(value, " national soccer team")
	value = strings.TrimSuffix(value, " national team")
	value = strings.ReplaceAll(value, "&", "and")

	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func slug(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func cloneTournament(tournament *TournamentResponse) *TournamentResponse {
	if tournament == nil {
		return nil
	}
	clone := *tournament
	clone.Groups = append([]Group(nil), tournament.Groups...)
	clone.Knockout = append([]KnockoutRound(nil), tournament.Knockout...)
	for roundIndex := range clone.Knockout {
		clone.Knockout[roundIndex].Matches = append([]Match(nil), tournament.Knockout[roundIndex].Matches...)
	}
	for groupIndex := range clone.Groups {
		clone.Groups[groupIndex].Teams = append([]Team(nil), tournament.Groups[groupIndex].Teams...)
		clone.Groups[groupIndex].Standings = append([]Team(nil), tournament.Groups[groupIndex].Standings...)
		clone.Groups[groupIndex].Matches = append([]Match(nil), tournament.Groups[groupIndex].Matches...)
		for teamIndex := range clone.Groups[groupIndex].Teams {
			clone.Groups[groupIndex].Teams[teamIndex].Schedule = append(
				[]Match(nil),
				tournament.Groups[groupIndex].Teams[teamIndex].Schedule...,
			)
		}
		for teamIndex := range clone.Groups[groupIndex].Standings {
			clone.Groups[groupIndex].Standings[teamIndex].Schedule = append(
				[]Match(nil),
				tournament.Groups[groupIndex].Standings[teamIndex].Schedule...,
			)
		}
	}
	return &clone
}

var (
	drawPositionPattern     = regexp.MustCompile(`^[A-L][1-4]$`)
	groupTextPattern        = regexp.MustCompile(`(?i)\bGroup\s+([A-L])\b`)
	intPattern              = regexp.MustCompile(`-?\d+`)
	scorePattern            = regexp.MustCompile(`^(\d+)\s*[–-]\s*(\d+)`)
	teamLinkPattern         = regexp.MustCompile(`\[\[([^|\]]*national (?:football|soccer) team)(?:\|([^\]]+))?\]\]`)
	listTransclusionPattern = regexp.MustCompile(`{{#lst:([^|{}]+)\|`)
	wikiParamPattern        = regexp.MustCompile(`\|([A-Za-z0-9_]+)\s*=\s*([^|]*)`)
	startDatePattern        = regexp.MustCompile(`Start date\|(\d{4})\|(\d{1,2})\|(\d{1,2})`)
	flagCodePattern         = regexp.MustCompile(`\|([A-Z0-9]{2,4})\s*}}`)
	htmlCommentPattern      = regexp.MustCompile(`(?s)<!--.*?-->`)
	refPattern              = regexp.MustCompile(`(?s)<ref[^>]*>.*?</ref>`)
	selfClosingRefPattern   = regexp.MustCompile(`<ref[^>]*/>`)
	wikiLinkPattern         = regexp.MustCompile(`\[\[[^|\]]+\|([^\]]+)\]\]`)
	wikiSimpleLinkPattern   = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	simpleTemplatePattern   = regexp.MustCompile(`{{[^{}]*}}`)
)
