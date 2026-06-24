<template>
  <div class="section-header worldcup-header">
    <div>
      <h1>世界杯</h1>
      <div class="light-text">参赛队伍、小组积分、世界排名与小组赛程</div>
    </div>
    <div class="toolbar worldcup-toolbar">
      <el-select v-model="selectedGroup" placeholder="小组" class="group-select">
        <el-option v-for="option in groupOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
      <el-input v-model="keyword" :prefix-icon="Search" clearable placeholder="搜索球队 / 场馆 / 赛程" class="search-input" />
      <el-radio-group v-model="matchStatusFilter" class="status-filter">
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
      <div class="summary-label">已结束</div>
      <div class="summary-value">{{ formatCount(worldCup?.summary.finished_matches) }}</div>
    </div>
    <div class="worldcup-summary-item source-item">
      <div class="summary-label">数据更新时间</div>
      <div class="summary-value small">{{ formatFetchedAt(worldCup?.fetched_at) }}</div>
    </div>
  </div>

  <el-skeleton v-if="loading && !worldCup" :rows="8" animated class="worldcup-skeleton" />

  <el-empty v-else-if="!worldCup" description="暂无世界杯数据" />

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
                  <div class="team-schedule-title">{{ row.name }} 赛程</div>
                  <div class="team-schedule-list">
                    <div v-for="match in row.schedule" :key="match.id" class="schedule-line">
                      <div class="schedule-time">
                        <span>{{ match.date || '-' }}</span>
                        <span>{{ match.time || '-' }}</span>
                      </div>
                      <div class="schedule-scoreline">
                        <span :class="teamNameClass(match, match.home_team)">{{ match.home_team }}</span>
                        <strong>{{ formatMatchScore(match) }}</strong>
                        <span :class="teamNameClass(match, match.away_team)">{{ match.away_team }}</span>
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
                    <div class="team-name">{{ row.name }}</div>
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
                <span>{{ match.date || '-' }}</span>
                <span>{{ match.time || '-' }}</span>
              </div>
              <div class="fixture-teams">
                <span>{{ match.home_team }}</span>
                <strong>{{ formatMatchScore(match) }}</strong>
                <span>{{ match.away_team }}</span>
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

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Link, RefreshRight, Search } from '@element-plus/icons-vue';
import { fetchWorldCup } from '@/api/worldCup';
import type { WorldCupGroup, WorldCupMatch, WorldCupResponse, WorldCupTeam } from '@/types/worldCup';

type MatchStatusFilter = 'all' | 'finished' | 'scheduled';
type GroupView = WorldCupGroup & {
  visibleTeams: WorldCupTeam[];
  visibleMatches: WorldCupMatch[];
};

const worldCup = ref<WorldCupResponse | null>(null);
const loading = ref(false);
const loadError = ref('');
const selectedGroup = ref('all');
const keyword = ref('');
const matchStatusFilter = ref<MatchStatusFilter>('all');

const groupOptions = computed(() => [
  { label: '全部小组', value: 'all' },
  ...(worldCup.value?.groups.map((group) => ({ label: group.label, value: group.key })) ?? [])
]);

const normalizedKeyword = computed(() => keyword.value.trim().toLowerCase());

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
    team.name,
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
  return [match.home_team, match.away_team, match.date, match.time, match.venue]
    .filter(Boolean)
    .some((field) => field.toLowerCase().includes(value));
}

function matchMatchesStatus(match: WorldCupMatch): boolean {
  if (matchStatusFilter.value === 'all') return true;
  return match.status === matchStatusFilter.value;
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
  if (/free plans do not have access|plan:/i.test(value)) {
    return '当前 API-Football 套餐无法访问 2026 世界杯赛季。需要升级 API 套餐，或把 FOOTBALL_WORLD_CUP_SEASON 临时改为 2022/2024 查看可用历史数据。';
  }
  if (/FOOTBALL_API_KEY/i.test(value)) {
    return '世界杯数据加载失败：请检查后端环境变量 FOOTBALL_API_KEY。';
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

.worldcup-summary-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
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

.worldcup-group {
  padding: 18px;
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

  .source-item {
    grid-column: span 2;
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

  .source-item {
    grid-column: auto;
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
