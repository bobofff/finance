<template>
  <div class="section-header worldcup-header">
    <div>
      <h1>世界杯</h1>
      <div class="light-text">参赛队伍、小组积分、世界排名与小组赛程</div>
    </div>
    <div class="toolbar worldcup-toolbar">
      <el-select v-model="selectedGroup" placeholder="小组" class="group-select" :disabled="activeStage !== 'group-stage'">
        <el-option v-for="option in groupOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
      <el-input
        v-model="keyword"
        :prefix-icon="Search"
        clearable
        placeholder="搜索球队 / 场馆 / 赛程"
        class="search-input"
        :disabled="activeStage === 'score-frequency'"
      />
      <el-radio-group v-model="matchStatusFilter" class="status-filter" :disabled="activeStage === 'score-frequency'">
        <el-radio-button label="all">全部赛程</el-radio-button>
        <el-radio-button label="finished">已结束</el-radio-button>
        <el-radio-button label="scheduled">未开始</el-radio-button>
      </el-radio-group>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadWorldCup(true)">刷新数据</el-button>
    </div>
  </div>

  <el-alert
    v-if="worldCup?.stale"
    type="warning"
    show-icon
    :closable="false"
    class="stale-alert"
    :title="`当前展示缓存数据，实时刷新失败：${worldCup.warning || '未知错误'}`"
  />

  <el-alert
    v-if="loadError"
    type="error"
    show-icon
    :closable="false"
    class="stale-alert"
    :title="loadError"
  />

  <div class="worldcup-summary-grid">
    <div class="worldcup-summary-item">
      <div class="summary-label">小组</div>
      <div class="summary-value">{{ formatCount(worldCup?.summary.group_count) }}</div>
    </div>
    <div class="worldcup-summary-item">
      <div class="summary-label">球队</div>
      <div class="summary-value">{{ formatCount(worldCup?.summary.team_count) }}</div>
    </div>
    <div class="worldcup-summary-item">
      <div class="summary-label">比赛</div>
      <div class="summary-value">{{ formatCount(worldCup?.summary.match_count) }}</div>
    </div>
    <div class="worldcup-summary-item">
      <div class="summary-label">淘汰赛</div>
      <div class="summary-value">{{ formatCount(worldCup?.summary.knockout_matches) }}</div>
    </div>
    <div class="worldcup-summary-item">
      <div class="summary-label">已结束</div>
      <div class="summary-value">{{ formatCount(worldCup?.summary.finished_matches) }}</div>
    </div>
    <div class="worldcup-summary-item source-item">
      <div class="summary-label">数据更新时间</div>
      <div class="summary-value small">{{ formatFetchedAt(worldCup?.fetched_at) }}</div>
    </div>
  </div>

  <div class="worldcup-stage-nav">
    <el-breadcrumb separator="/" class="worldcup-breadcrumb">
      <el-breadcrumb-item>世界杯</el-breadcrumb-item>
      <el-breadcrumb-item v-for="item in stageNavItems" :key="item.key">
        <button
          type="button"
          class="breadcrumb-stage"
          :class="activeStage === item.key ? 'active' : undefined"
          :aria-current="activeStage === item.key ? 'page' : undefined"
          @click="activeStage = item.key"
        >
          <span>{{ item.label }}</span>
          <small>{{ item.meta }}</small>
        </button>
      </el-breadcrumb-item>
    </el-breadcrumb>
    <div class="stage-current">{{ activeStageLabel }}</div>
  </div>

  <el-skeleton v-if="loading && !worldCup" :rows="8" animated class="worldcup-skeleton" />

  <el-empty v-else-if="!worldCup" description="暂无世界杯数据" />

  <template v-else>
    <div v-if="showTodayMatches" class="card today-matches-card">
      <div class="match-day-grid">
        <div v-for="panel in matchDayPanels" :key="panel.key" class="match-day-panel">
          <div class="match-day-header">
            <div>
              <div class="group-title">{{ panel.title }}</div>
              <div class="light-text">{{ panel.dateLabel }}</div>
            </div>
            <el-tag type="info" effect="light">{{ panel.visibleMatches.length }}/{{ panel.matches.length }} 场</el-tag>
          </div>

          <div v-if="panel.visibleMatches.length > 0" class="today-fixtures-list">
            <div v-for="match in panel.visibleMatches" :key="matchIdentity(match)" class="fixture-line today-fixture-line">
              <div class="fixture-date">
                <span class="today-match-stage">{{ matchStageLabel(match) }}</span>
                <span>{{ formatMatchTime(match) }}</span>
              </div>
              <div class="fixture-teams">
                <span>{{ formatMatchTeamName(match.home_team) }}</span>
                <strong>{{ formatMatchScore(match) }}</strong>
                <span>{{ formatMatchTeamName(match.away_team) }}</span>
              </div>
              <div class="fixture-meta">
                <el-tag :type="matchTagType(match)" effect="light" size="small">
                  {{ match.status === 'finished' ? '已结束' : '未开始' }}
                </el-tag>
                <span>{{ match.venue || '-' }}</span>
              </div>
            </div>
          </div>
          <el-empty v-else :description="panel.emptyDescription" :image-size="80" />
        </div>
      </div>
    </div>

    <div v-else-if="showKnockoutBracket" class="card knockout-bracket">
      <div class="knockout-header">
        <div>
          <div class="group-title">淘汰赛对阵图</div>
          <div class="light-text">{{ knockoutBracketSubtitle }}</div>
        </div>
      </div>

      <div v-if="visibleKnockoutRounds.length > 0" class="bracket-scroll">
        <div class="bracket-symmetric" :style="{ '--bracket-match-slots': String(knockoutBracketLayout.maxMatchesPerColumn) }">
          <div class="bracket-side bracket-side-left">
            <div
              v-for="round in knockoutBracketLayout.leftRounds"
              :key="round.key"
              class="bracket-round bracket-round-left"
              :style="{ '--round-match-count': String(round.visibleMatches.length) }"
            >
              <div class="bracket-round-title">{{ round.label }}</div>
              <div class="bracket-match-list">
                <div v-for="match in round.visibleMatches" :key="match.id" class="bracket-match">
                  <div class="bracket-match-time">
                    <span>{{ formatMatchDate(match) }}</span>
                    <span>{{ formatMatchTime(match) }}</span>
                  </div>
                  <div class="bracket-team-row">
                    <span :class="teamNameClass(match, match.home_team)">{{ formatMatchTeamName(match.home_team) }}</span>
                    <strong>{{ formatMatchScore(match) }}</strong>
                    <span :class="teamNameClass(match, match.away_team)">{{ formatMatchTeamName(match.away_team) }}</span>
                  </div>
                  <div class="bracket-match-meta">
                    <el-tag :type="matchTagType(match)" effect="light" size="small">
                      {{ match.status === 'finished' ? '已结束' : '未开始' }}
                    </el-tag>
                    <span>{{ match.venue || '-' }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="bracket-center">
            <div v-if="knockoutBracketLayout.centerRounds.length > 0" class="bracket-center-rounds">
              <div v-for="round in knockoutBracketLayout.centerRounds" :key="round.key" class="bracket-center-round">
                <div class="bracket-round-title">{{ round.label }}</div>
                <div class="bracket-match-list">
                  <div v-for="match in round.visibleMatches" :key="match.id" class="bracket-match bracket-match-center">
                    <div class="bracket-match-time">
                      <span>{{ formatMatchDate(match) }}</span>
                      <span>{{ formatMatchTime(match) }}</span>
                    </div>
                    <div class="bracket-team-row">
                      <span :class="teamNameClass(match, match.home_team)">{{ formatMatchTeamName(match.home_team) }}</span>
                      <strong>{{ formatMatchScore(match) }}</strong>
                      <span :class="teamNameClass(match, match.away_team)">{{ formatMatchTeamName(match.away_team) }}</span>
                    </div>
                    <div class="bracket-match-meta">
                      <el-tag :type="matchTagType(match)" effect="light" size="small">
                        {{ match.status === 'finished' ? '已结束' : '未开始' }}
                      </el-tag>
                      <span>{{ match.venue || '-' }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="bracket-center-empty">决赛区</div>
          </div>

          <div class="bracket-side bracket-side-right">
            <div
              v-for="round in knockoutBracketLayout.rightRounds"
              :key="round.key"
              class="bracket-round bracket-round-right"
              :style="{ '--round-match-count': String(round.visibleMatches.length) }"
            >
              <div class="bracket-round-title">{{ round.label }}</div>
              <div class="bracket-match-list">
                <div v-for="match in round.visibleMatches" :key="match.id" class="bracket-match">
                  <div class="bracket-match-time">
                    <span>{{ formatMatchDate(match) }}</span>
                    <span>{{ formatMatchTime(match) }}</span>
                  </div>
                  <div class="bracket-team-row">
                    <span :class="teamNameClass(match, match.home_team)">{{ formatMatchTeamName(match.home_team) }}</span>
                    <strong>{{ formatMatchScore(match) }}</strong>
                    <span :class="teamNameClass(match, match.away_team)">{{ formatMatchTeamName(match.away_team) }}</span>
                  </div>
                  <div class="bracket-match-meta">
                    <el-tag :type="matchTagType(match)" effect="light" size="small">
                      {{ match.status === 'finished' ? '已结束' : '未开始' }}
                    </el-tag>
                    <span>{{ match.venue || '-' }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <el-empty v-else description="暂无淘汰赛赛程" :image-size="80" />
    </div>

    <div v-else-if="showScoreFrequency" class="card score-frequency-card">
      <div class="score-frequency-header">
        <div>
          <div class="group-title">比分频率</div>
          <div class="light-text">{{ scoreFrequencySummary }}</div>
        </div>
        <div class="score-frequency-kpis">
          <div class="score-kpi">
            <span>已统计</span>
            <strong>{{ scoredMatchCount }}</strong>
          </div>
          <div class="score-kpi">
            <span>比分种类</span>
            <strong>{{ scoreFrequencies.length }}</strong>
          </div>
          <div class="score-kpi">
            <span>最高频</span>
            <strong>{{ topScoreFrequency?.score || '-' }}</strong>
          </div>
        </div>
      </div>

      <div v-if="scoreTreemapTiles.length > 0" class="score-treemap" role="list" :aria-label="scoreFrequencySummary">
        <div
          v-for="(item, index) in scoreTreemapTiles"
          :key="item.score"
          class="score-treemap-tile"
          :class="item.compact ? 'compact' : undefined"
          :style="scoreTreemapTileStyle(item, index)"
          :title="scoreTreemapTileTitle(item)"
          role="listitem"
        >
          <div class="score-tile-score">{{ item.score }}</div>
          <div class="score-tile-count">{{ item.count }} 场</div>
          <div v-if="!item.compact" class="score-tile-percent">{{ scoreFrequencyPercent(item.count) }}</div>
        </div>
      </div>
      <el-empty v-else description="暂无已结束比赛比分" :image-size="80" />
    </div>

    <template v-else>
      <el-empty v-if="visibleGroups.length === 0" description="没有匹配的小组或球队" />

      <div v-else class="worldcup-groups">
        <div v-for="group in visibleGroups" :key="group.key" class="card worldcup-group">
          <div class="worldcup-group-header">
            <div>
              <div class="group-title">{{ group.label }}</div>
              <div class="light-text">
                {{ group.teams.length }} 支球队 · {{ group.matches.length }} 场小组赛
              </div>
            </div>
            <div class="group-actions">
              <el-tag type="info" effect="light">{{ finishedCount(group.matches) }}/{{ group.matches.length }} 已结束</el-tag>
              <el-button :icon="Link" size="small" @click="openSource(group.source_url)">来源</el-button>
            </div>
          </div>

          <div class="worldcup-table-scroll">
            <el-table
              :data="group.visibleTeams"
              stripe
              border
              row-key="code"
              style="width: 100%; min-width: 1180px"
            >
              <el-table-column type="expand" width="48">
                <template #default="{ row }">
                  <div class="team-schedule">
                    <div class="team-schedule-title">{{ formatTeamNameZh(row.name, row.code) }} 赛程</div>
                    <div class="team-schedule-list">
                      <div v-for="match in row.schedule" :key="match.id" class="schedule-line">
                        <div class="schedule-time">
                          <span>{{ formatMatchDate(match) }}</span>
                          <span>{{ formatMatchTime(match) }}</span>
                        </div>
                        <div class="schedule-scoreline">
                          <span :class="teamNameClass(match, match.home_team)">{{ formatTeamNameZh(match.home_team) }}</span>
                          <strong>{{ formatMatchScore(match) }}</strong>
                          <span :class="teamNameClass(match, match.away_team)">{{ formatTeamNameZh(match.away_team) }}</span>
                        </div>
                        <div class="schedule-meta">
                          <el-tag :type="matchTagType(match)" effect="light" size="small">
                            {{ match.status === 'finished' ? '已结束' : '未开始' }}
                          </el-tag>
                          <span>{{ match.venue || '-' }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="小组排名" width="100">
                <template #default="{ row }">
                  <span class="rank-pill">{{ formatRank(row.group_rank) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="球队" min-width="190">
                <template #default="{ row }">
                  <div class="team-cell">
                    <img v-if="row.flag_url" :src="row.flag_url" alt="" class="team-flag" />
                    <div>
                      <div class="team-name">{{ formatTeamNameZh(row.name, row.code) }}</div>
                      <div class="team-sub">{{ teamSubline(row) }}</div>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="世界排名" width="100">
                <template #default="{ row }">{{ formatWorldRank(row.world_rank) }}</template>
              </el-table-column>
              <el-table-column label="档位" width="76">
                <template #default="{ row }">{{ formatOptionalNumber(row.pot) }}</template>
              </el-table-column>
              <el-table-column prop="played" label="场" width="62" />
              <el-table-column prop="won" label="胜" width="62" />
              <el-table-column prop="drawn" label="平" width="62" />
              <el-table-column prop="lost" label="负" width="62" />
              <el-table-column prop="goals_for" label="进球" width="76" />
              <el-table-column prop="goals_against" label="失球" width="76" />
              <el-table-column label="净胜球" width="88">
                <template #default="{ row }">
                  <span :class="goalDiffClass(row.goal_difference)">{{ formatGoalDiff(row.goal_difference) }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="points" label="积分" width="76" />
              <el-table-column label="晋级状态" min-width="150">
                <template #default="{ row }">
                  <span>{{ compactAdvanceNote(row.advance_note) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="资格来源" min-width="190">
                <template #default="{ row }">
                  <span>{{ row.qualification || '-' }}</span>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="group-fixtures">
            <div class="fixtures-title">小组赛程</div>
            <div class="fixtures-list">
              <div v-for="match in group.visibleMatches" :key="match.id" class="fixture-line">
                <div class="fixture-date">
                  <span>{{ formatMatchDate(match) }}</span>
                  <span>{{ formatMatchTime(match) }}</span>
                </div>
                <div class="fixture-teams">
                  <span>{{ formatTeamNameZh(match.home_team) }}</span>
                  <strong>{{ formatMatchScore(match) }}</strong>
                  <span>{{ formatTeamNameZh(match.away_team) }}</span>
                </div>
                <div class="fixture-meta">
                  <el-tag :type="matchTagType(match)" effect="light" size="small">
                    {{ match.status === 'finished' ? '已结束' : '未开始' }}
                  </el-tag>
                  <span>{{ match.venue || '-' }}</span>
                </div>
              </div>
              <el-empty v-if="group.visibleMatches.length === 0" description="当前筛选下没有赛程" :image-size="80" />
            </div>
          </div>
        </div>
      </div>
    </template>
  </template>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Link, RefreshRight, Search } from '@element-plus/icons-vue';
import { fetchWorldCup } from '@/api/worldCup';
import type { WorldCupGroup, WorldCupKnockoutRound, WorldCupMatch, WorldCupResponse, WorldCupTeam } from '@/types/worldCup';
import { formatTeamNameZh, teamNameSearchValues } from '@/utils/teamNames';

type MatchStatusFilter = 'all' | 'finished' | 'scheduled';
type WorldCupStage = 'today-matches' | 'group-stage' | 'knockout-stage' | 'score-frequency';
type WorldCupStageNavItem = {
  key: WorldCupStage;
  label: string;
  meta: string;
};
type GroupView = WorldCupGroup & {
  visibleTeams: WorldCupTeam[];
  visibleMatches: WorldCupMatch[];
};
type KnockoutRoundView = WorldCupKnockoutRound & {
  visibleMatches: WorldCupMatch[];
};
type KnockoutBracketLayout = {
  leftRounds: KnockoutRoundView[];
  centerRounds: KnockoutRoundView[];
  rightRounds: KnockoutRoundView[];
  maxMatchesPerColumn: number;
};
type ScoreFrequency = {
  score: string;
  count: number;
  matches: WorldCupMatch[];
  totalGoals: number;
};
type ScoreTreemapTile = ScoreFrequency & {
  x: number;
  y: number;
  width: number;
  height: number;
  compact: boolean;
};
type MatchDayPanel = {
  key: 'today' | 'tomorrow';
  title: string;
  dateLabel: string;
  matches: WorldCupMatch[];
  visibleMatches: WorldCupMatch[];
  emptyDescription: string;
};

const scoreTreemapColors = ['#2563eb', '#16a34a', '#f59e0b', '#dc2626', '#7c3aed', '#0891b2', '#db2777', '#475569', '#65a30d', '#ea580c'];

const worldCup = ref<WorldCupResponse | null>(null);
const loading = ref(false);
const loadError = ref('');
const selectedGroup = ref('all');
const keyword = ref('');
const matchStatusFilter = ref<MatchStatusFilter>('all');
const activeStage = ref<WorldCupStage>('today-matches');
const today = ref(new Date());

const groupOptions = computed(() => [
  { label: '全部小组', value: 'all' },
  ...(worldCup.value?.groups.map((group) => ({ label: group.label, value: group.key })) ?? [])
]);

const normalizedKeyword = computed(() => keyword.value.trim().toLowerCase());
const showTodayMatches = computed(() => activeStage.value === 'today-matches');
const showKnockoutBracket = computed(() => activeStage.value === 'knockout-stage');
const showScoreFrequency = computed(() => activeStage.value === 'score-frequency');

const visibleKnockoutRounds = computed<KnockoutRoundView[]>(() => {
  if (!showKnockoutBracket.value) return [];
  return (worldCup.value?.knockout_rounds ?? [])
    .map((round) => ({
      ...round,
      visibleMatches: round.matches.filter(
        (match) => matchMatchesKeyword(match, normalizedKeyword.value) && matchMatchesStatus(match)
      )
    }))
    .filter((round) => round.visibleMatches.length > 0);
});

const knockoutBracketLayout = computed<KnockoutBracketLayout>(() => {
  const sideRounds: KnockoutRoundView[] = [];
  const centerRounds: KnockoutRoundView[] = [];

  for (const round of visibleKnockoutRounds.value) {
    if (isCenterKnockoutRound(round.key)) {
      centerRounds.push(round);
    } else {
      sideRounds.push(round);
    }
  }

  const leftRounds = sideRounds
    .map((round) => ({
      ...round,
      visibleMatches: splitKnockoutRoundMatches(round.visibleMatches, 'left')
    }))
    .filter((round) => round.visibleMatches.length > 0);
  const rightRounds = sideRounds
    .map((round) => ({
      ...round,
      visibleMatches: splitKnockoutRoundMatches(round.visibleMatches, 'right')
    }))
    .filter((round) => round.visibleMatches.length > 0)
    .reverse();
  const orderedCenterRounds = [...centerRounds].sort((a, b) => centerKnockoutRoundOrder(a.key) - centerKnockoutRoundOrder(b.key));
  const maxMatchesPerColumn = Math.max(
    2,
    ...leftRounds.map((round) => round.visibleMatches.length),
    ...rightRounds.map((round) => round.visibleMatches.length),
    ...orderedCenterRounds.map((round) => round.visibleMatches.length)
  );

  return {
    leftRounds,
    centerRounds: orderedCenterRounds,
    rightRounds,
    maxMatchesPerColumn
  };
});

const knockoutBracketSubtitle = computed(() => {
  const count = worldCup.value?.summary.knockout_matches ?? 0;
  return count > 0 ? `${count} 场淘汰赛 · 从 32 强到决赛` : '等待数据源公布淘汰赛赛程';
});

const allTournamentMatches = computed<WorldCupMatch[]>(() => {
  const matches = new Map<string, WorldCupMatch>();
  for (const group of worldCup.value?.groups ?? []) {
    for (const match of group.matches) {
      matches.set(matchIdentity(match), match);
    }
  }
  for (const round of worldCup.value?.knockout_rounds ?? []) {
    for (const match of round.matches) {
      matches.set(matchIdentity(match), match);
    }
  }
  return Array.from(matches.values());
});

const matchStageLabels = computed(() => {
  const labels = new Map<string, string>();
  for (const group of worldCup.value?.groups ?? []) {
    for (const match of group.matches) {
      labels.set(matchIdentity(match), group.label);
    }
  }
  for (const round of worldCup.value?.knockout_rounds ?? []) {
    for (const match of round.matches) {
      labels.set(matchIdentity(match), round.label);
    }
  }
  return labels;
});

const todayDateKey = computed(() => dateKey(today.value));
const tomorrow = computed(() => addDays(today.value, 1));
const tomorrowDateKey = computed(() => dateKey(tomorrow.value));
const todayDateLabel = computed(() => formatDayLabel(today.value));
const tomorrowDateLabel = computed(() => formatDayLabel(tomorrow.value));

const todayMatches = computed<WorldCupMatch[]>(() =>
  allTournamentMatches.value
    .filter((match) => matchLocalDateKey(match) === todayDateKey.value)
    .sort(compareMatchesByKickoff)
);

const tomorrowMatches = computed<WorldCupMatch[]>(() =>
  allTournamentMatches.value
    .filter((match) => matchLocalDateKey(match) === tomorrowDateKey.value)
    .sort(compareMatchesByKickoff)
);

const visibleTodayMatches = computed(() =>
  todayMatches.value.filter((match) => matchMatchesKeyword(match, normalizedKeyword.value) && matchMatchesStatus(match))
);

const visibleTomorrowMatches = computed(() =>
  tomorrowMatches.value.filter((match) => matchMatchesKeyword(match, normalizedKeyword.value) && matchMatchesStatus(match))
);

const todayEmptyDescription = computed(() => (todayMatches.value.length === 0 ? '今天暂无比赛' : '当前筛选下没有今日比赛'));
const tomorrowEmptyDescription = computed(() => (tomorrowMatches.value.length === 0 ? '明天暂无比赛' : '当前筛选下没有明日比赛'));

const matchDayPanels = computed<MatchDayPanel[]>(() => [
  {
    key: 'today',
    title: '今日比赛',
    dateLabel: todayDateLabel.value,
    matches: todayMatches.value,
    visibleMatches: visibleTodayMatches.value,
    emptyDescription: todayEmptyDescription.value
  },
  {
    key: 'tomorrow',
    title: '明日比赛',
    dateLabel: tomorrowDateLabel.value,
    matches: tomorrowMatches.value,
    visibleMatches: visibleTomorrowMatches.value,
    emptyDescription: tomorrowEmptyDescription.value
  }
]);

const scoreFrequencies = computed<ScoreFrequency[]>(() => {
  const frequencies = new Map<string, ScoreFrequency>();
  for (const match of allTournamentMatches.value) {
    if (!isFinishedMatch(match)) continue;
    const normalized = normalizeMatchScore(match);
    if (!normalized) continue;
    const existing = frequencies.get(normalized.score);
    if (existing) {
      existing.count += 1;
      existing.matches.push(match);
      continue;
    }
    frequencies.set(normalized.score, {
      score: normalized.score,
      count: 1,
      matches: [match],
      totalGoals: normalized.totalGoals
    });
  }
  return Array.from(frequencies.values()).sort(
    (a, b) => b.count - a.count || a.totalGoals - b.totalGoals || a.score.localeCompare(b.score, 'zh-CN', { numeric: true })
  );
});

const scoredMatchCount = computed(() => scoreFrequencies.value.reduce((sum, item) => sum + item.count, 0));
const topScoreFrequency = computed(() => scoreFrequencies.value[0]);
const scoreTreemapTiles = computed<ScoreTreemapTile[]>(() => buildScoreTreemap(scoreFrequencies.value, 0, 0, 100, 100));

const scoreFrequencySummary = computed(() => {
  if (scoredMatchCount.value === 0) return '等待已结束比赛产生比分';
  const topScore = topScoreFrequency.value ? ` · ${topScoreFrequency.value.score} 出现最多` : '';
  return `${scoredMatchCount.value} 场已结束比赛 · ${scoreFrequencies.value.length} 种比分${topScore}`;
});

const stageNavItems = computed<WorldCupStageNavItem[]>(() => [
  { key: 'today-matches', label: '今日比赛', meta: `今 ${todayMatches.value.length} · 明 ${tomorrowMatches.value.length}` },
  { key: 'group-stage', label: '小组赛', meta: `${formatCount(worldCup.value?.summary.group_count)} 组` },
  { key: 'knockout-stage', label: '淘汰赛', meta: `${formatCount(worldCup.value?.summary.knockout_matches)} 场` },
  { key: 'score-frequency', label: '比分统计', meta: `${scoreFrequencies.value.length} 种` }
]);

const activeStageLabel = computed(() => stageNavItems.value.find((item) => item.key === activeStage.value)?.label ?? '今日比赛');

const visibleGroups = computed<GroupView[]>(() => {
  const groups = worldCup.value?.groups ?? [];
  return groups
    .filter((group) => selectedGroup.value === 'all' || group.key === selectedGroup.value)
    .map((group) => {
      const visibleTeams = group.standings.filter((team) => teamMatchesKeyword(team, normalizedKeyword.value));
      const visibleMatches = group.matches.filter(
        (match) => matchMatchesKeyword(match, normalizedKeyword.value) && matchMatchesStatus(match)
      );
      return { ...group, visibleTeams, visibleMatches };
    })
    .filter((group) => group.visibleTeams.length > 0 || group.visibleMatches.length > 0);
});

const loadWorldCup = async (refresh = false) => {
  today.value = new Date();
  loading.value = true;
  loadError.value = '';
  try {
    worldCup.value = await fetchWorldCup({ refresh });
  } catch (error) {
    const message = formatWorldCupError((error as Error).message);
    loadError.value = message;
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
};

function teamMatchesKeyword(team: WorldCupTeam, value: string): boolean {
  if (!value) return true;
  const fields = [
    ...teamNameSearchValues(team.name, team.code),
    team.draw_position,
    team.confederation,
    team.qualification,
    team.best_performance,
    team.advance_note
  ];
  return fields.some((field) => field?.toLowerCase().includes(value)) || team.schedule.some((match) => matchMatchesKeyword(match, value));
}

function matchMatchesKeyword(match: WorldCupMatch, value: string): boolean {
  if (!value) return true;
  return [
    ...teamNameSearchValues(match.home_team),
    ...teamNameSearchValues(match.away_team),
    match.utc_date,
    match.date,
    match.time,
    formatMatchDate(match),
    formatMatchTime(match),
    match.venue
  ]
    .some((field) => field?.toLowerCase().includes(value));
}

function matchMatchesStatus(match: WorldCupMatch): boolean {
  if (matchStatusFilter.value === 'all') return true;
  return match.status === matchStatusFilter.value;
}

function isCenterKnockoutRound(key: string): boolean {
  return key === 'FINAL' || key === 'THIRD_PLACE';
}

function centerKnockoutRoundOrder(key: string): number {
  if (key === 'FINAL') return 1;
  if (key === 'THIRD_PLACE') return 2;
  return 3;
}

function splitKnockoutRoundMatches(matches: WorldCupMatch[], side: 'left' | 'right'): WorldCupMatch[] {
  if (matches.length <= 1) return side === 'left' ? matches : [];
  const pivot = Math.ceil(matches.length / 2);
  return side === 'left' ? matches.slice(0, pivot) : matches.slice(pivot);
}

function matchIdentity(match: WorldCupMatch): string {
  return match.id || [match.stage, match.group, match.utc_date, match.home_team, match.away_team].filter(Boolean).join('|');
}

function dateKey(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

function formatDayLabel(date: Date): string {
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    weekday: 'short'
  });
}

function matchLocalDateKey(match: WorldCupMatch): string {
  const date = matchDateValue(match) ?? parseMatchDate(match.date);
  return date ? dateKey(date) : match.date;
}

function parseMatchDate(value?: string): Date | null {
  const trimmed = value?.trim();
  if (!trimmed) return null;
  const plainDate = trimmed.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (plainDate) {
    return new Date(Number(plainDate[1]), Number(plainDate[2]) - 1, Number(plainDate[3]));
  }
  const parsed = new Date(trimmed);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function compareMatchesByKickoff(a: WorldCupMatch, b: WorldCupMatch): number {
  const aDate = matchDateValue(a) ?? parseMatchDate(a.date);
  const bDate = matchDateValue(b) ?? parseMatchDate(b.date);
  if (aDate && bDate && aDate.getTime() !== bDate.getTime()) {
    return aDate.getTime() - bDate.getTime();
  }
  return (a.time || '').localeCompare(b.time || '', 'zh-CN', { numeric: true });
}

function matchStageLabel(match: WorldCupMatch): string {
  return matchStageLabels.value.get(matchIdentity(match)) ?? fallbackMatchStageLabel(match);
}

function fallbackMatchStageLabel(match: WorldCupMatch): string {
  if (match.group) return `小组 ${match.group}`;
  const stage = match.stage?.trim();
  if (!stage) return '世界杯';
  const normalized = stage.toUpperCase().replace(/\s+/g, '_');
  const stageLabels: Record<string, string> = {
    GROUP_STAGE: '小组赛',
    LAST_32: '32强',
    ROUND_OF_32: '32强',
    LAST_16: '16强',
    ROUND_OF_16: '16强',
    QUARTER_FINALS: '1/4决赛',
    SEMI_FINALS: '半决赛',
    THIRD_PLACE: '季军赛',
    FINAL: '决赛'
  };
  return stageLabels[normalized] ?? stage;
}

function isFinishedMatch(match: WorldCupMatch): boolean {
  return match.status.toLowerCase() === 'finished';
}

function normalizeMatchScore(match: WorldCupMatch): { score: string; totalGoals: number } | null {
  if (match.home_score !== undefined && match.away_score !== undefined) {
    return {
      score: `${match.home_score}-${match.away_score}`,
      totalGoals: match.home_score + match.away_score
    };
  }
  const scoreMatch = match.score.trim().match(/^(\d+)\s*[–-]\s*(\d+)/);
  if (!scoreMatch) return null;
  const homeScore = Number(scoreMatch[1]);
  const awayScore = Number(scoreMatch[2]);
  return {
    score: `${homeScore}-${awayScore}`,
    totalGoals: homeScore + awayScore
  };
}

function buildScoreTreemap(items: ScoreFrequency[], x: number, y: number, width: number, height: number): ScoreTreemapTile[] {
  if (items.length === 0) return [];
  if (items.length === 1) {
    return [
      {
        ...items[0],
        x,
        y,
        width,
        height,
        compact: width < 16 || height < 18 || width * height < 260
      }
    ];
  }

  const total = sumScoreFrequency(items);
  if (total <= 0) return [];
  const splitIndex = balancedTreemapSplitIndex(items, total);
  const firstItems = items.slice(0, splitIndex);
  const secondItems = items.slice(splitIndex);
  const firstTotal = sumScoreFrequency(firstItems);

  if (width >= height) {
    const firstWidth = (width * firstTotal) / total;
    return [
      ...buildScoreTreemap(firstItems, x, y, firstWidth, height),
      ...buildScoreTreemap(secondItems, x + firstWidth, y, width - firstWidth, height)
    ];
  }

  const firstHeight = (height * firstTotal) / total;
  return [
    ...buildScoreTreemap(firstItems, x, y, width, firstHeight),
    ...buildScoreTreemap(secondItems, x, y + firstHeight, width, height - firstHeight)
  ];
}

function balancedTreemapSplitIndex(items: ScoreFrequency[], total: number): number {
  let runningTotal = 0;
  let bestIndex = 1;
  let bestDiff = Number.POSITIVE_INFINITY;

  for (let index = 0; index < items.length - 1; index += 1) {
    runningTotal += items[index].count;
    const diff = Math.abs(total / 2 - runningTotal);
    if (diff < bestDiff) {
      bestDiff = diff;
      bestIndex = index + 1;
    }
  }

  return bestIndex;
}

function sumScoreFrequency(items: ScoreFrequency[]): number {
  return items.reduce((sum, item) => sum + item.count, 0);
}

function scoreTreemapTileStyle(item: ScoreTreemapTile, index: number): Record<string, string> {
  return {
    left: `${item.x}%`,
    top: `${item.y}%`,
    width: `${item.width}%`,
    height: `${item.height}%`,
    background: scoreTreemapColors[index % scoreTreemapColors.length]
  };
}

function scoreTreemapTileTitle(item: ScoreTreemapTile): string {
  return `${item.score}：${item.count} 场，${scoreFrequencyPercent(item.count)}`;
}

function scoreFrequencyPercent(count: number): string {
  if (scoredMatchCount.value <= 0) return '0%';
  return `${((count / scoredMatchCount.value) * 100).toFixed(1)}%`;
}

function formatCount(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return String(value);
}

function formatRank(value?: number): string {
  if (!value || value <= 0) return '-';
  return String(value);
}

function formatWorldRank(value?: number): string {
  if (!value || value <= 0) return '-';
  return `#${value}`;
}

function formatOptionalNumber(value?: number): string {
  if (!value || value <= 0) return '-';
  return String(value);
}

function teamSubline(team: WorldCupTeam): string {
  const parts = [team.draw_position, team.confederation].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : '-';
}

function formatWorldCupError(message: string): string {
  const value = message || '世界杯数据加载失败';
  if (/restricted|paid subscription|forbidden|403/i.test(value)) {
    return '当前 football-data.org token 无法访问这个世界杯赛季或资源。请确认 token 权限，或临时调整 FOOTBALL_WORLD_CUP_SEASON 查看可用历史数据。';
  }
  if (/FOOTBALL_DATA_TOKEN/i.test(value)) {
    return '世界杯数据加载失败：请检查后端环境变量 FOOTBALL_DATA_TOKEN。';
  }
  return value;
}

function formatGoalDiff(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return value > 0 ? `+${value}` : String(value);
}

function formatFetchedAt(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  });
}

function matchDateValue(match: WorldCupMatch): Date | null {
  if (!match.utc_date) return null;
  const date = new Date(match.utc_date);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatMatchDate(match: WorldCupMatch): string {
  const date = matchDateValue(match);
  if (!date) return match.date || '-';
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  });
}

function formatMatchTime(match: WorldCupMatch): string {
  const date = matchDateValue(match);
  if (!date) return match.time || '-';
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit'
  });
}

function formatMatchTeamName(name: string): string {
  return name ? formatTeamNameZh(name) : '待定';
}

function formatMatchScore(match: WorldCupMatch): string {
  return match.status === 'finished' && match.score ? match.score : '未赛';
}

function goalDiffClass(value: number): string {
  if (value > 0) return 'goal-positive';
  if (value < 0) return 'goal-negative';
  return 'goal-neutral';
}

function matchTagType(match: WorldCupMatch): 'success' | 'info' {
  return match.status === 'finished' ? 'success' : 'info';
}

function teamNameClass(match: WorldCupMatch, teamName: string): string {
  if (match.home_score === undefined || match.away_score === undefined || match.home_score === match.away_score) {
    return '';
  }
  const isHomeWinner = match.home_score > match.away_score;
  const isHomeTeam = teamName === match.home_team;
  return isHomeWinner === isHomeTeam ? 'winner-name' : '';
}

function compactAdvanceNote(value: string): string {
  if (!value) return '-';
  return value.replace('Round of 32', '32强').replace('Possible Round of 32', '可能晋级32强');
}

function finishedCount(matches: WorldCupMatch[]): number {
  return matches.filter((match) => match.status === 'finished').length;
}

function openSource(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer');
}

onMounted(() => {
  loadWorldCup();
});
</script>

<style scoped>
.worldcup-header {
  align-items: flex-start;
}

.worldcup-toolbar {
  justify-content: flex-end;
  max-width: 920px;
}

.group-select {
  width: 132px;
}

.search-input {
  width: 240px;
}

.status-filter {
  flex-shrink: 0;
}

.stale-alert {
  margin-bottom: 16px;
}

.worldcup-stage-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  margin-bottom: 16px;
  border: 1px solid var(--app-border-soft);
  border-radius: 12px;
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.worldcup-breadcrumb {
  display: flex;
  flex-wrap: wrap;
  row-gap: 6px;
  min-width: 0;
}

.worldcup-breadcrumb :deep(.el-breadcrumb__inner) {
  display: inline-flex;
  align-items: center;
}

.breadcrumb-stage {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  padding: 5px 7px;
  border: none;
  border-radius: 8px;
  color: var(--app-text-secondary);
  background: transparent;
  font: inherit;
  line-height: 1.2;
  cursor: pointer;
}

.breadcrumb-stage:hover,
.breadcrumb-stage:focus-visible {
  color: var(--app-info);
  background: rgba(37, 99, 235, 0.1);
  outline: none;
}

.breadcrumb-stage.active {
  color: var(--app-info);
  background: rgba(37, 99, 235, 0.14);
  font-weight: 720;
}

.breadcrumb-stage small {
  color: var(--app-text-muted);
  font-size: 11px;
}

.breadcrumb-stage.active small {
  color: inherit;
  opacity: 0.78;
}

.stage-current {
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 720;
  color: var(--app-text-primary);
}

.worldcup-summary-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.worldcup-summary-item {
  min-height: 86px;
  padding: 16px;
  border: 1px solid var(--app-border-soft);
  border-radius: 12px;
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.summary-label {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.summary-value {
  margin-top: 8px;
  font-size: 28px;
  line-height: 1.1;
  font-weight: 750;
  color: var(--app-text-primary);
}

.summary-value.small {
  font-size: 18px;
  line-height: 1.3;
}

.worldcup-skeleton {
  padding: 20px;
  border-radius: 16px;
  background: var(--app-surface);
}

.worldcup-groups {
  display: grid;
  gap: 16px;
}

.today-matches-card,
.knockout-bracket {
  padding: 18px;
  margin-bottom: 16px;
}

.match-day-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.match-day-panel {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--app-border-soft);
  border-radius: 10px;
  background: var(--app-surface-solid);
}

.match-day-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.today-fixtures-list {
  display: grid;
  gap: 8px;
}

.today-match-stage {
  font-weight: 720;
  color: var(--app-text-primary);
}

.knockout-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.bracket-scroll {
  overflow-x: auto;
  padding-bottom: 4px;
}

.bracket-symmetric {
  display: grid;
  grid-template-columns: minmax(420px, 1fr) minmax(220px, 0.46fr) minmax(420px, 1fr);
  gap: 16px;
  min-width: 1180px;
  min-height: max(460px, calc(var(--bracket-match-slots, 4) * 112px));
}

.bracket-side {
  display: grid;
  gap: 12px;
  align-items: stretch;
}

.bracket-side-left {
  grid-auto-flow: column;
  grid-auto-columns: minmax(150px, 1fr);
}

.bracket-side-right {
  grid-auto-flow: column;
  grid-auto-columns: minmax(150px, 1fr);
}

.bracket-round {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 12px;
}

.bracket-round-title {
  font-weight: 720;
  color: var(--app-text-primary);
}

.bracket-match-list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
  justify-content: space-around;
  min-height: calc(var(--round-match-count, 1) * 96px);
}

.bracket-center {
  display: flex;
  align-items: stretch;
  justify-content: center;
  min-width: 0;
}

.bracket-center-rounds {
  display: flex;
  width: 100%;
  flex-direction: column;
  justify-content: center;
  gap: 18px;
}

.bracket-center-round {
  display: grid;
  gap: 12px;
}

.bracket-center-empty {
  display: grid;
  width: 100%;
  min-height: 180px;
  place-items: center;
  align-self: center;
  border: 1px dashed var(--app-border-soft);
  border-radius: 8px;
  color: var(--app-text-muted);
  background: var(--app-surface-muted);
}

.bracket-match {
  position: relative;
  display: grid;
  gap: 8px;
  min-height: 96px;
  padding: 10px;
  border: 1px solid var(--app-border-soft);
  border-radius: 8px;
  background: var(--app-surface-solid);
}

.bracket-round-left .bracket-match::after,
.bracket-round-right .bracket-match::before {
  position: absolute;
  top: 50%;
  width: 12px;
  height: 1px;
  content: '';
  background: var(--app-border);
}

.bracket-round-left .bracket-match::after {
  right: -13px;
}

.bracket-round-right .bracket-match::before {
  left: -13px;
}

.bracket-match-center {
  min-height: 112px;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.12);
}

.bracket-match-time,
.bracket-match-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  color: var(--app-text-secondary);
  font-size: 12px;
}

.bracket-team-row {
  display: grid;
  grid-template-columns: minmax(58px, 1fr) 54px minmax(58px, 1fr);
  align-items: center;
  gap: 8px;
  color: var(--app-text-primary);
}

.bracket-team-row span:first-child {
  text-align: right;
}

.bracket-team-row span:last-child {
  text-align: left;
}

.bracket-team-row strong {
  text-align: center;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}

.worldcup-group {
  padding: 18px;
}

.score-frequency-card {
  padding: 18px;
}

.score-frequency-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.score-frequency-kpis {
  display: grid;
  grid-template-columns: repeat(3, minmax(78px, 1fr));
  gap: 8px;
  min-width: 300px;
}

.score-kpi {
  padding: 10px 12px;
  border: 1px solid var(--app-border-soft);
  border-radius: 8px;
  background: var(--app-surface-muted);
}

.score-kpi span {
  display: block;
  font-size: 12px;
  color: var(--app-text-secondary);
}

.score-kpi strong {
  display: block;
  margin-top: 4px;
  font-size: 20px;
  line-height: 1.2;
  color: var(--app-text-primary);
}

.score-treemap {
  position: relative;
  min-height: 430px;
  overflow: hidden;
  border: 1px solid var(--app-border-soft);
  border-radius: 8px;
  background: var(--app-surface-muted);
}

.score-treemap-tile {
  position: absolute;
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  justify-content: space-between;
  gap: 4px;
  padding: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.72);
  color: #ffffff;
  box-shadow: inset 0 -30px 54px rgba(15, 23, 42, 0.18);
}

.score-treemap-tile.compact {
  justify-content: center;
  padding: 7px;
}

.score-tile-score {
  overflow: hidden;
  font-size: 24px;
  line-height: 1;
  font-weight: 820;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score-tile-count,
.score-tile-percent {
  overflow: hidden;
  font-size: 13px;
  line-height: 1.2;
  opacity: 0.88;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score-treemap-tile.compact .score-tile-score {
  font-size: 14px;
}

.score-treemap-tile.compact .score-tile-count {
  display: none;
}

.worldcup-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.group-title {
  font-size: 20px;
  font-weight: 760;
  color: var(--app-text-primary);
}

.group-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.worldcup-table-scroll {
  overflow-x: auto;
}

.team-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  text-align: left;
}

.team-flag {
  width: 28px;
  height: 20px;
  object-fit: cover;
  border: 1px solid var(--app-border);
  border-radius: 3px;
  background: var(--app-surface-muted);
}

.team-name {
  font-weight: 680;
  color: var(--app-text-primary);
}

.team-sub {
  margin-top: 2px;
  font-size: 12px;
  color: var(--app-text-secondary);
}

.rank-pill {
  display: inline-grid;
  min-width: 30px;
  height: 26px;
  place-items: center;
  border-radius: 8px;
  background: rgba(34, 197, 94, 0.12);
  color: #15803d;
  font-weight: 760;
}

.goal-positive {
  color: var(--app-positive);
  font-weight: 700;
}

.goal-negative {
  color: var(--app-negative);
  font-weight: 700;
}

.goal-neutral {
  color: var(--app-text-secondary);
}

.team-schedule {
  padding: 12px 12px 8px;
  background: var(--app-surface-muted);
  border-radius: 10px;
}

.team-schedule-title,
.fixtures-title {
  font-weight: 720;
  color: var(--app-text-primary);
  margin-bottom: 10px;
}

.team-schedule-list,
.fixtures-list {
  display: grid;
  gap: 8px;
}

.schedule-line,
.fixture-line {
  display: grid;
  grid-template-columns: minmax(160px, 0.8fr) minmax(240px, 1.2fr) minmax(220px, 1fr);
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--app-border-soft);
  border-radius: 10px;
  background: var(--app-surface-solid);
}

.schedule-time,
.fixture-date,
.fixture-meta,
.schedule-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  color: var(--app-text-secondary);
  font-size: 12px;
}

.schedule-scoreline,
.fixture-teams {
  display: grid;
  grid-template-columns: minmax(80px, 1fr) 64px minmax(80px, 1fr);
  align-items: center;
  gap: 8px;
  color: var(--app-text-primary);
}

.schedule-scoreline span:first-child,
.fixture-teams span:first-child {
  text-align: right;
}

.schedule-scoreline span:last-child,
.fixture-teams span:last-child {
  text-align: left;
}

.schedule-scoreline strong,
.fixture-teams strong {
  text-align: center;
  font-size: 14px;
}

.winner-name {
  font-weight: 760;
  color: var(--app-positive);
}

.group-fixtures {
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid var(--app-border-soft);
}

@media (max-width: 1100px) {
  .section-header.worldcup-header {
    flex-direction: column;
  }

  .worldcup-toolbar {
    justify-content: flex-start;
    width: 100%;
  }

  .worldcup-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .worldcup-stage-nav,
  .score-frequency-header {
    align-items: stretch;
    flex-direction: column;
  }

  .score-frequency-kpis {
    width: 100%;
    min-width: 0;
  }

  .source-item {
    grid-column: span 2;
  }

  .bracket-symmetric {
    min-width: 980px;
  }

  .schedule-line,
  .fixture-line {
    grid-template-columns: 1fr;
  }

  .schedule-scoreline,
  .fixture-teams {
    grid-template-columns: minmax(80px, 1fr) 58px minmax(80px, 1fr);
  }
}

@media (max-width: 720px) {
  .worldcup-toolbar,
  .status-filter {
    width: 100%;
  }

  .group-select,
  .search-input {
    width: 100%;
  }

  .worldcup-summary-grid {
    grid-template-columns: 1fr;
  }

  .worldcup-stage-nav {
    padding: 12px;
  }

  .stage-current {
    display: none;
  }

  .breadcrumb-stage {
    gap: 4px;
    padding: 4px 5px;
  }

  .score-frequency-kpis {
    grid-template-columns: 1fr;
  }

  .score-treemap {
    min-height: 360px;
  }

  .score-treemap-tile {
    padding: 9px;
  }

  .score-tile-score {
    font-size: 18px;
  }

  .source-item {
    grid-column: auto;
  }

  .match-day-grid {
    grid-template-columns: 1fr;
  }

  .match-day-header,
  .knockout-header {
    flex-direction: column;
  }

  .bracket-symmetric {
    grid-template-columns: minmax(360px, 1fr) minmax(190px, 0.5fr) minmax(360px, 1fr);
    min-width: 940px;
  }

  .bracket-side-left,
  .bracket-side-right {
    grid-auto-columns: minmax(128px, 1fr);
  }

  .worldcup-group-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .group-actions {
    justify-content: flex-start;
  }
}
</style>
