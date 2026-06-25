export interface WorldCupMatch {
  id: string;
  group: string;
  stage?: string;
  utc_date?: string;
  date: string;
  time: string;
  home_team: string;
  home_wiki_title: string;
  away_team: string;
  away_wiki_title: string;
  score: string;
  home_score?: number;
  away_score?: number;
  status: 'finished' | 'scheduled' | string;
  venue: string;
}

export interface WorldCupTeam {
  code: string;
  draw_position: string;
  name: string;
  wiki_title: string;
  wiki_url: string;
  flag_url: string;
  pot: number;
  confederation: string;
  qualification: string;
  qualified_on: string;
  finals_appearance: string;
  last_appearance: string;
  best_performance: string;
  draw_rank: number;
  world_rank: number;
  group_rank: number;
  played: number;
  won: number;
  drawn: number;
  lost: number;
  goals_for: number;
  goals_against: number;
  goal_difference: number;
  points: number;
  advance_note: string;
  schedule: WorldCupMatch[];
}

export interface WorldCupGroup {
  key: string;
  label: string;
  source_url: string;
  teams: WorldCupTeam[];
  standings: WorldCupTeam[];
  matches: WorldCupMatch[];
}

export interface WorldCupKnockoutRound {
  key: string;
  label: string;
  matches: WorldCupMatch[];
}

export interface WorldCupResponse {
  competition: string;
  season: number;
  fetched_at: string;
  cache_seconds: number;
  stale: boolean;
  warning?: string;
  source: {
    name: string;
    url: string;
  };
  summary: {
    group_count: number;
    team_count: number;
    match_count: number;
    knockout_matches: number;
    finished_matches: number;
    scheduled_matches: number;
  };
  groups: WorldCupGroup[];
  knockout_rounds: WorldCupKnockoutRound[];
}
