<template>
  <div class="section-header">
    <div>
      <h1>年度目标</h1>
      <div class="light-text">设置年度净资产目标，自动计算每月所需净收入与完成度</div>
    </div>
    <div class="toolbar">
      <el-input-number v-model="year" :min="2000" :max="9999" :step="1" controls-position="right" @change="onYearChange" />
      <el-date-picker v-model="asOf" type="date" value-format="YYYY-MM-DD" @change="loadProgress" />
      <el-button :loading="loadingGoal || loadingReport" @click="reloadAll">刷新</el-button>
    </div>
  </div>

  <div class="card" v-loading="loadingGoal">
    <div class="card-title-row">
      <div class="card-title">目标配置</div>
      <el-tag v-if="hasGoal" type="success" effect="light">已配置</el-tag>
      <el-tag v-else type="info" effect="light">未配置</el-tag>
    </div>
    <el-form inline class="goal-form">
      <el-form-item label="目标净资产">
        <el-input-number
          v-model="goalForm.targetNetWorth"
          :min="0"
          :precision="2"
          :step="10000"
          controls-position="right"
          class="goal-input"
        />
      </el-form-item>
      <el-form-item label="基准日">
        <el-date-picker
          v-model="goalForm.baselineDate"
          type="date"
          value-format="YYYY-MM-DD"
          :disabled-date="isBaselineDateDisabled"
        />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="goalForm.note" placeholder="可选：目标说明" class="goal-note-input" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="saveGoal">保存目标</el-button>
      </el-form-item>
    </el-form>
  </div>

  <div class="summary-grid">
    <div class="summary-card">
      <div class="summary-label">目标净资产</div>
      <div class="summary-value">{{ formatAmount(report?.goal.target_net_worth) }}</div>
    </div>
    <div class="summary-card">
      <div class="summary-label">当前净资产</div>
      <div class="summary-value">{{ formatAmount(report?.current_net_worth) }}</div>
    </div>
    <div class="summary-card">
      <div class="summary-label">剩余差额</div>
      <div class="summary-value">{{ formatAmount(report?.remaining_gap) }}</div>
    </div>
    <div class="summary-card">
      <div class="summary-label">每月需净收入</div>
      <div class="summary-value">{{ formatAmount(report?.required_monthly_net_income) }}</div>
    </div>
    <div class="summary-card">
      <div class="summary-label">剩余月份</div>
      <div class="summary-value">{{ report?.remaining_months ?? '-' }}</div>
    </div>
    <div class="summary-card highlight">
      <div class="summary-label">总体完成度</div>
      <div class="summary-value">{{ formatPercent(report?.progress) }}</div>
    </div>
  </div>

  <div class="card" v-loading="loadingReport">
    <div class="card-title-row">
      <div class="card-title">月度完成度（{{ year }}）</div>
      <div class="light-text">统计日期：{{ report?.as_of || asOf }}</div>
    </div>
    <el-table :data="report?.months ?? []" stripe border style="width: 100%">
      <el-table-column prop="month" label="月份" width="120" />
      <el-table-column label="收入" min-width="130" align="right">
        <template #default="{ row }">
          <span class="amount-positive">{{ formatAmount(row.income) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="支出" min-width="130" align="right">
        <template #default="{ row }">
          <span class="amount-negative">{{ formatAmount(row.expense) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="净收入" min-width="130" align="right">
        <template #default="{ row }">
          <span :class="row.net_income >= 0 ? 'amount-positive' : 'amount-negative'">
            {{ formatAmount(row.net_income) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="月目标净收入" min-width="130" align="right">
        <template #default="{ row }">{{ formatAmount(row.required_net_income) }}</template>
      </el-table-column>
      <el-table-column label="完成度" min-width="240">
        <template #default="{ row }">
          <div class="completion-cell">
            <el-progress
              :percentage="toProgressPercent(row.completion)"
              :status="row.completion >= 1 ? 'success' : row.net_income < 0 ? 'exception' : ''"
            />
            <span class="completion-text">{{ formatPercent(row.completion) }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.is_future ? 'info' : 'success'" effect="plain">
            {{ row.is_future ? '未来' : '已发生' }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { fetchAnnualAssetGoal, upsertAnnualAssetGoal } from '@/api/goal';
import { fetchAnnualAssetProgress } from '@/api/report';
import type { AnnualAssetProgressResponse } from '@/types/report';

const today = new Date();
const year = ref(today.getFullYear());
const asOf = ref(formatDateISO(today));

const loadingGoal = ref(false);
const loadingReport = ref(false);
const saving = ref(false);
const hasGoal = ref(false);
const report = ref<AnnualAssetProgressResponse | null>(null);

const goalForm = reactive({
  targetNetWorth: 0,
  baselineDate: defaultBaselineDate(today.getFullYear()),
  note: ''
});

async function reloadAll() {
  await loadGoal();
  await loadProgress();
}

async function loadGoal() {
  loadingGoal.value = true;
  try {
    const goal = await fetchAnnualAssetGoal(year.value);
    hasGoal.value = true;
    goalForm.targetNetWorth = goal.target_net_worth;
    goalForm.baselineDate = goal.baseline_date;
    goalForm.note = goal.note || '';
  } catch (error) {
    const message = (error as Error).message || '';
    if (message.includes('annual goal not found')) {
      hasGoal.value = false;
      goalForm.targetNetWorth = 0;
      goalForm.baselineDate = defaultBaselineDate(year.value);
      goalForm.note = '';
      report.value = null;
      return;
    }
    ElMessage.error(message);
  } finally {
    loadingGoal.value = false;
  }
}

async function saveGoal() {
  if (goalForm.targetNetWorth <= 0) {
    ElMessage.warning('目标净资产必须大于 0');
    return;
  }
  if (!goalForm.baselineDate) {
    ElMessage.warning('请选择基准日');
    return;
  }
  const allowedPrefix = [`${year.value}-`, `${year.value + 1}-`];
  if (!allowedPrefix.some((prefix) => goalForm.baselineDate.startsWith(prefix))) {
    ElMessage.warning(`基准日必须在 ${year.value} 或 ${year.value + 1} 年内`);
    return;
  }

  saving.value = true;
  try {
    await upsertAnnualAssetGoal(year.value, {
      target_net_worth: goalForm.targetNetWorth,
      baseline_date: goalForm.baselineDate,
      note: goalForm.note || undefined
    });
    hasGoal.value = true;
    ElMessage.success('年度目标已保存');
    await loadProgress();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

function onYearChange() {
  goalForm.baselineDate = defaultBaselineDate(year.value);
  reloadAll();
}

function isBaselineDateDisabled(date: Date): boolean {
  const y = date.getFullYear();
  return y !== year.value && y !== year.value + 1;
}

async function loadProgress() {
  if (!hasGoal.value) {
    report.value = null;
    return;
  }

  loadingReport.value = true;
  try {
    report.value = await fetchAnnualAssetProgress({
      year: year.value,
      as_of: asOf.value
    });
  } catch (error) {
    const message = (error as Error).message || '';
    if (message.includes('annual goal not found')) {
      report.value = null;
      hasGoal.value = false;
      return;
    }
    ElMessage.error(message);
  } finally {
    loadingReport.value = false;
  }
}

function formatDateISO(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

function defaultBaselineDate(targetYear: number): string {
  return `${targetYear}-01-01`;
}

function formatAmount(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return value.toFixed(2);
}

function formatPercent(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return `${(value * 100).toFixed(2)}%`;
}

function toProgressPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  if (value <= 0) return 0;
  if (value >= 1) return 100;
  return Number((value * 100).toFixed(2));
}

onMounted(() => {
  reloadAll();
});
</script>

<style scoped>
.card-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.card-title {
  font-weight: 700;
}

.goal-form {
  display: flex;
  flex-wrap: wrap;
}

.goal-input {
  width: 200px;
}

.goal-note-input {
  width: 260px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin: 16px 0;
}

.summary-card {
  background: var(--app-surface-solid);
  border-radius: 14px;
  padding: 16px 18px;
  box-shadow: var(--app-shadow);
  border: 1px solid var(--app-border-soft);
}

.summary-card.highlight {
  background: var(--app-highlight-surface);
}

.summary-label {
  font-size: 12px;
  color: var(--app-text-secondary);
  margin-bottom: 6px;
}

.summary-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--app-text-primary);
}

.amount-positive {
  color: var(--app-positive);
  font-weight: 600;
}

.amount-negative {
  color: var(--app-negative);
  font-weight: 600;
}

.completion-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.completion-text {
  min-width: 70px;
  text-align: right;
  font-weight: 600;
  color: var(--app-text-primary);
}

@media (max-width: 960px) {
  .summary-grid {
    grid-template-columns: repeat(1, minmax(0, 1fr));
  }

  .goal-note-input {
    width: 200px;
  }
}
</style>
