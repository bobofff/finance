<template>
  <div class="section-header">
    <div>
      <h1>资产负债表</h1>
      <div class="light-text">按账户类型汇总资产、负债与净资产</div>
    </div>
    <div class="toolbar">
      <el-date-picker v-model="asOf" type="date" value-format="YYYY-MM-DD" />
      <el-button type="primary" :loading="loading" @click="loadReport">刷新</el-button>
    </div>
  </div>

  <div class="card income-expense-board">
    <div class="card-header">
      <div>
        <div class="card-title">收入支出看板</div>
        <div class="light-text">默认统计当月，可手动选择区间</div>
      </div>
      <div class="toolbar">
        <el-date-picker
          v-model="incomeDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          :clearable="false"
          class="income-range-picker"
          @change="loadIncomeExpenseSummary"
        />
        <el-button :loading="incomeExpenseLoading" @click="loadIncomeExpenseSummary">刷新收支</el-button>
      </div>
    </div>

    <div class="income-summary-grid">
      <div class="income-summary-card income">
        <div class="summary-label">总收入</div>
        <div class="summary-value">{{ formatNumber(incomeExpenseSummary?.totals.income) }}</div>
      </div>
      <div class="income-summary-card expense">
        <div class="summary-label">总支出</div>
        <div class="summary-value">{{ formatNumber(incomeExpenseSummary?.totals.expense) }}</div>
      </div>
      <div class="income-summary-card net">
        <div class="summary-label">净收入</div>
        <div class="summary-value">{{ formatNumber(incomeExpenseSummary?.totals.net_income) }}</div>
      </div>
      <div class="income-summary-card">
        <div class="summary-label">收支笔数</div>
        <div class="summary-value">{{ formatCount(incomeExpenseSummary?.totals.transaction_count) }}</div>
      </div>
    </div>

    <div class="income-board-content">
      <div class="echart-wrapper income-chart">
        <div ref="incomeChartRef" class="echart-canvas"></div>
      </div>
      <div class="income-pie-card">
        <div class="pie-card-title">收入一级分类</div>
        <div ref="incomeCategoryPieRef" class="pie-canvas"></div>
      </div>
      <div class="income-pie-card">
        <div class="pie-card-title">支出一级分类</div>
        <div ref="expenseCategoryPieRef" class="pie-canvas"></div>
      </div>
    </div>
  </div>

  <div class="summary-grid">
    <div class="summary-card">
      <div class="summary-label">总资产</div>
      <div class="summary-value">{{ formatNumber(report?.totals.assets) }}</div>
    </div>
    <div class="summary-card">
      <div class="summary-label">总负债</div>
      <div class="summary-value">{{ formatNumber(report?.totals.liabilities) }}</div>
    </div>
    <div class="summary-card highlight">
      <div class="summary-label">净资产</div>
      <div class="summary-value">{{ formatNumber(report?.totals.net_worth) }}</div>
    </div>
    <div class="summary-card">
      <div class="summary-label">统计日期</div>
      <div class="summary-value">{{ report?.as_of || '-' }}</div>
    </div>
  </div>

  <div class="card" v-for="group in reportGroups" :key="group.key">
    <div class="card-header">
      <div class="card-title">{{ group.label }}</div>
      <div class="card-total">小计 {{ formatNumber(group.total) }}</div>
    </div>
    <div class="group-charts" :class="{ 'with-pie': group.key === 'asset' }">
      <div class="echart-wrapper chart-bar">
        <div class="echart-canvas" :ref="(el) => setGroupChartRef(group.key, el as HTMLDivElement | null)"></div>
      </div>
      <div v-if="group.key === 'asset'" class="echart-wrapper chart-pie">
        <div class="echart-canvas" :ref="(el) => setGroupPieRef(group.key, el as HTMLDivElement | null)"></div>
      </div>
      <div v-if="group.key === 'asset'" class="echart-wrapper chart-type-pie">
        <div class="echart-canvas" :ref="(el) => setGroupTypePieRef(group.key, el as HTMLDivElement | null)"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import * as echarts from 'echarts';
import { fetchBalanceSheet, fetchIncomeExpenseSummary } from '@/api/report';
import type {
  BalanceSheetResponse,
  BalanceSheetGroup,
  IncomeExpenseSummaryCategory,
  IncomeExpenseSummaryResponse
} from '@/types/report';

const report = ref<BalanceSheetResponse | null>(null);
const incomeExpenseSummary = ref<IncomeExpenseSummaryResponse | null>(null);
const loading = ref(false);
const incomeExpenseLoading = ref(false);
const asOf = ref(formatDateISO(new Date()));
const incomeDateRange = ref<[string, string]>(defaultCurrentMonthRange());

const reportGroups = computed<BalanceSheetGroup[]>(() => report.value?.groups ?? []);
const chartRefs = new Map<string, HTMLDivElement>();
const chartInstances = new Map<string, echarts.ECharts>();
const pieRefs = new Map<string, HTMLDivElement>();
const pieInstances = new Map<string, echarts.ECharts>();
const typePieRefs = new Map<string, HTMLDivElement>();
const typePieInstances = new Map<string, echarts.ECharts>();
const incomeChartRef = ref<HTMLDivElement | null>(null);
const incomeCategoryPieRef = ref<HTMLDivElement | null>(null);
const expenseCategoryPieRef = ref<HTMLDivElement | null>(null);
let incomeChart: echarts.ECharts | null = null;
let incomeCategoryPieChart: echarts.ECharts | null = null;
let expenseCategoryPieChart: echarts.ECharts | null = null;

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function getChartTheme() {
  return {
    textPrimary: getCssVar('--app-text-primary') || '#1f2937',
    textSecondary: getCssVar('--app-text-secondary') || '#64748b',
    textMuted: getCssVar('--app-text-muted') || '#94a3b8',
    border: getCssVar('--app-border') || '#e2e8f0',
    grid: getCssVar('--app-chart-grid') || 'rgba(148, 163, 184, 0.22)',
    tooltipBg: getCssVar('--app-chart-tooltip-bg') || 'rgba(255, 255, 255, 0.96)',
    tooltipBorder: getCssVar('--app-chart-tooltip-border') || 'rgba(148, 163, 184, 0.28)'
  };
}

function getTooltipStyle() {
  const theme = getChartTheme();
  return {
    backgroundColor: theme.tooltipBg,
    borderColor: theme.tooltipBorder,
    textStyle: {
      color: theme.textPrimary
    }
  };
}

const loadReport = async () => {
  loading.value = true;
  try {
    report.value = await fetchBalanceSheet({ as_of: asOf.value });
    await nextTick();
    renderCharts();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

const loadIncomeExpenseSummary = async () => {
  const [dateFrom, dateTo] = incomeDateRange.value;
  if (!dateFrom || !dateTo) {
    ElMessage.warning('请选择统计区间');
    return;
  }
  if (dateFrom > dateTo) {
    ElMessage.warning('开始日期不能晚于结束日期');
    return;
  }

  incomeExpenseLoading.value = true;
  try {
    incomeExpenseSummary.value = await fetchIncomeExpenseSummary({
      date_from: dateFrom,
      date_to: dateTo
    });
    await nextTick();
    renderIncomeExpenseChart();
    renderIncomeExpenseCategoryPieCharts();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    incomeExpenseLoading.value = false;
  }
};

function formatNumber(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return value.toFixed(2);
}

function formatCount(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return String(Math.round(value));
}

function formatPercent(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return `${(value * 100).toFixed(2)}%`;
}

function formatAccountTypeLabel(type: string): string {
  switch ((type || '').toLowerCase()) {
    case 'cash':
      return '现金';
    case 'investment':
      return '证券';
    case 'liability':
      return '负债';
    case 'receivable':
      return '债权';
    case 'debt':
      return '债权';
    case 'other_asset':
      return '其他资产';
    default:
      return type || '其他';
  }
}

function setGroupChartRef(key: string, el: HTMLDivElement | null) {
  if (el) {
    chartRefs.set(key, el);
  } else {
    chartRefs.delete(key);
  }
}

function setGroupPieRef(key: string, el: HTMLDivElement | null) {
  if (el) {
    pieRefs.set(key, el);
  } else {
    pieRefs.delete(key);
  }
}

function setGroupTypePieRef(key: string, el: HTMLDivElement | null) {
  if (el) {
    typePieRefs.set(key, el);
  } else {
    typePieRefs.delete(key);
  }
}

function renderCharts() {
  const theme = getChartTheme();

  for (const group of reportGroups.value) {
    const el = chartRefs.get(group.key);
    if (!el) continue;
    let chart = chartInstances.get(group.key);
    if (!chart) {
      chart = echarts.init(el);
      chartInstances.set(group.key, chart);
    }
    const accounts = group.accounts ?? [];
    const categories = accounts.map((item) => item.name);
    const values = accounts.map((item) => item.balance);
    const option: echarts.EChartsOption = {
      grid: { left: 40, right: 24, top: 20, bottom: 60, containLabel: true },
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        ...getTooltipStyle()
      },
      xAxis: {
        type: 'category',
        data: categories,
        axisLabel: {
          interval: 0,
          rotate: 30,
          color: theme.textSecondary
        },
        axisLine: {
          lineStyle: { color: theme.border }
        }
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: theme.textSecondary },
        splitLine: {
          lineStyle: { color: theme.grid }
        }
      },
      series: [
        {
          type: 'bar',
          data: values,
          barWidth: 26,
          itemStyle: {
            color: (params) => (Number(params.value) >= 0 ? '#3b82f6' : '#ef4444')
          }
        }
      ]
    };
    chart.setOption(option);
    chart.resize();
  }

  for (const group of reportGroups.value) {
    const el = pieRefs.get(group.key);
    if (!el) {
      const existing = pieInstances.get(group.key);
      if (existing) {
        existing.dispose();
        pieInstances.delete(group.key);
      }
      continue;
    }
    if (group.key !== 'asset') {
      const existing = pieInstances.get(group.key);
      if (existing) {
        existing.dispose();
        pieInstances.delete(group.key);
      }
      continue;
    }
    let chart = pieInstances.get(group.key);
    if (!chart) {
      chart = echarts.init(el);
      pieInstances.set(group.key, chart);
    }
    const accounts = group.accounts ?? [];
    const data = accounts
      .filter((item) => Number.isFinite(item.balance) && item.balance !== 0)
      .map((item) => ({
        name: item.name,
        value: Math.abs(item.balance),
        raw: item.balance
      }))
      .sort((a, b) => a.value - b.value);

    const option: echarts.EChartsOption = {
      legend: {
        type: 'scroll',
        bottom: 0,
        left: 'center',
        itemWidth: 10,
        itemHeight: 10,
        textStyle: { fontSize: 12, color: theme.textSecondary }
      },
      tooltip: {
        trigger: 'item',
        ...getTooltipStyle(),
        formatter: (params) => {
          const value = Number((params.data as { raw: number } | undefined)?.raw ?? params.value ?? 0);
          const percent = Number(params.percent ?? 0);
          return `${params.name}<br/>${formatNumber(value)}<br/>占比 ${percent.toFixed(2)}%`;
        }
      },
      series: [
        {
          type: 'pie',
          roseType: 'radius',
          radius: ['25%', '78%'],
          center: ['50%', '50%'],
          data,
          label: { show: false },
          emphasis: { scale: true, scaleSize: 6 }
        }
      ]
    };
    chart.setOption(option);
    chart.resize();
  }

  for (const group of reportGroups.value) {
    const el = typePieRefs.get(group.key);
    if (!el) {
      const existing = typePieInstances.get(group.key);
      if (existing) {
        existing.dispose();
        typePieInstances.delete(group.key);
      }
      continue;
    }
    if (group.key !== 'asset') {
      const existing = typePieInstances.get(group.key);
      if (existing) {
        existing.dispose();
        typePieInstances.delete(group.key);
      }
      continue;
    }
    let chart = typePieInstances.get(group.key);
    if (!chart) {
      chart = echarts.init(el);
      typePieInstances.set(group.key, chart);
    }
    const typeMap = new Map<string, number>();
    const accounts = group.accounts ?? [];
    for (const item of accounts) {
      if (!Number.isFinite(item.balance) || item.balance <= 0) continue;
      const label = formatAccountTypeLabel(item.type);
      typeMap.set(label, (typeMap.get(label) ?? 0) + item.balance);
    }
    const data = Array.from(typeMap.entries())
      .map(([name, value]) => ({ name, value }))
      .sort((a, b) => a.value - b.value);

    const option: echarts.EChartsOption = {
      legend: {
        type: 'scroll',
        bottom: 0,
        left: 'center',
        itemWidth: 10,
        itemHeight: 10,
        textStyle: { fontSize: 12, color: theme.textSecondary }
      },
      tooltip: {
        trigger: 'item',
        ...getTooltipStyle(),
        formatter: (params) => `${params.name}<br/>${formatNumber(Number(params.value ?? 0))}<br/>占比 ${Number(params.percent ?? 0).toFixed(2)}%`
      },
      series: [
        {
          type: 'pie',
          radius: ['35%', '70%'],
          center: ['50%', '50%'],
          data,
          label: { show: false },
          emphasis: { scale: true, scaleSize: 6 }
        }
      ]
    };
    chart.setOption(option);
    chart.resize();
  }
}

function renderIncomeExpenseChart() {
  const el = incomeChartRef.value;
  if (!el || !incomeExpenseSummary.value) return;
  const theme = getChartTheme();

  if (!incomeChart) {
    incomeChart = echarts.init(el);
  }

  const days = incomeExpenseSummary.value.days ?? [];
  const labels = days.map((item) => item.date.slice(5));
  const incomeData = days.map((item) => item.income);
  const expenseData = days.map((item) => item.expense);
  const netIncomeData = days.map((item) => item.net_income);
  const labelInterval = days.length > 16 ? Math.ceil(days.length / 10) : 0;

  const option: echarts.EChartsOption = {
    grid: {
      left: 40,
      right: 24,
      top: 42,
      bottom: 36,
      containLabel: true
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      ...getTooltipStyle(),
      formatter: (params) => {
        const rows = Array.isArray(params) ? params : [params];
        const title = String(rows[0]?.axisValueLabel ?? '');
        const lines = rows.map((row) => `${row.marker}${row.seriesName} ${formatNumber(Number(row.data ?? 0))}`);
        return [title, ...lines].join('<br/>');
      }
    },
    xAxis: {
      type: 'category',
      data: labels,
      axisLabel: {
        interval: labelInterval,
        color: theme.textSecondary
      },
      axisLine: {
        lineStyle: { color: theme.border }
      }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: theme.textSecondary },
      splitLine: {
        lineStyle: { color: theme.grid }
      }
    },
    legend: {
      top: 0,
      data: ['收入', '支出', '净收入'],
      textStyle: { color: theme.textSecondary }
    },
    series: [
      {
        name: '收入',
        type: 'bar',
        data: incomeData,
        barMaxWidth: 22,
        itemStyle: { color: '#16a34a' }
      },
      {
        name: '支出',
        type: 'bar',
        data: expenseData,
        barMaxWidth: 22,
        itemStyle: { color: '#dc2626' }
      },
      {
        name: '净收入',
        type: 'line',
        smooth: true,
        symbolSize: 6,
        data: netIncomeData,
        itemStyle: { color: '#2563eb' },
        lineStyle: { color: '#2563eb', width: 2 }
      }
    ]
  };

  incomeChart.setOption(option);
  incomeChart.resize();
}

function renderIncomeExpenseCategoryPieCharts() {
  const summary = incomeExpenseSummary.value;
  if (!summary) return;

  if (incomeCategoryPieRef.value) {
    if (!incomeCategoryPieChart) {
      incomeCategoryPieChart = echarts.init(incomeCategoryPieRef.value);
    }
    renderCategoryPieChart(incomeCategoryPieChart, summary.breakdown?.income ?? [], '#16a34a');
  }

  if (expenseCategoryPieRef.value) {
    if (!expenseCategoryPieChart) {
      expenseCategoryPieChart = echarts.init(expenseCategoryPieRef.value);
    }
    renderCategoryPieChart(expenseCategoryPieChart, summary.breakdown?.expense ?? [], '#dc2626');
  }
}

function renderCategoryPieChart(
  chart: echarts.ECharts,
  categories: IncomeExpenseSummaryCategory[],
  baseColor: string
) {
  const theme = getChartTheme();
  const hasData = categories.some((item) => Number(item.amount) > 0);
  const data = categories.map((item) => ({
    name: item.name,
    value: item.amount,
    children: item.children
  }));

  const option: echarts.EChartsOption = {
    color: [baseColor, '#22c55e', '#38bdf8', '#f59e0b', '#a78bfa', '#06b6d4', '#fb7185', '#94a3b8'],
    legend: {
      type: 'scroll',
      orient: 'vertical',
      right: 0,
      top: 8,
      bottom: 8,
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { fontSize: 12, color: theme.textSecondary }
    },
    title: hasData
      ? undefined
      : {
          text: '暂无数据',
          left: '34%',
          top: '45%',
          textStyle: {
            color: theme.textMuted,
            fontSize: 13,
            fontWeight: 500
          }
        },
    tooltip: {
      trigger: 'item',
      ...getTooltipStyle(),
      formatter: (params: any) => {
        const children = ((params?.data?.children ?? []) as Array<{ name: string; amount: number; ratio: number }>).slice(0, 8);
        const header = `${params.marker}${params.name}<br/>金额 ${formatNumber(Number(params.value ?? 0))}<br/>一级占比 ${Number(params.percent ?? 0).toFixed(2)}%`;
        if (!children.length) return header;
        const lines = children.map(
          (item) => `${item.name} ${formatNumber(item.amount)} (${formatPercent(item.ratio)})`
        );
        return `${header}<br/>二级占比（该一级）<br/>${lines.join('<br/>')}`;
      }
    },
    series: [
      {
        type: 'pie',
        radius: ['34%', '72%'],
        center: ['36%', '50%'],
        data,
        label: { show: false },
        emphasis: { scale: true, scaleSize: 6 }
      }
    ]
  };

  chart.setOption(option);
  chart.resize();
}

function handleResize() {
  for (const chart of chartInstances.values()) {
    chart.resize();
  }
  for (const chart of pieInstances.values()) {
    chart.resize();
  }
  for (const chart of typePieInstances.values()) {
    chart.resize();
  }
  if (incomeChart) {
    incomeChart.resize();
  }
  if (incomeCategoryPieChart) {
    incomeCategoryPieChart.resize();
  }
  if (expenseCategoryPieChart) {
    expenseCategoryPieChart.resize();
  }
}

async function handleThemeChange() {
  await nextTick();
  renderCharts();
  renderIncomeExpenseChart();
  renderIncomeExpenseCategoryPieCharts();
}

function formatDateISO(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function defaultCurrentMonthRange(): [string, string] {
  const today = new Date();
  const monthStart = new Date(today.getFullYear(), today.getMonth(), 1);
  return [formatDateISO(monthStart), formatDateISO(today)];
}

onMounted(() => {
  loadReport();
  loadIncomeExpenseSummary();
  window.addEventListener('resize', handleResize);
  window.addEventListener('app:theme-change', handleThemeChange);
});

watch(reportGroups, async () => {
  await nextTick();
  renderCharts();
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
  window.removeEventListener('app:theme-change', handleThemeChange);
  for (const chart of chartInstances.values()) {
    chart.dispose();
  }
  chartInstances.clear();
  for (const chart of pieInstances.values()) {
    chart.dispose();
  }
  pieInstances.clear();
  for (const chart of typePieInstances.values()) {
    chart.dispose();
  }
  typePieInstances.clear();
  if (incomeChart) {
    incomeChart.dispose();
    incomeChart = null;
  }
  if (incomeCategoryPieChart) {
    incomeCategoryPieChart.dispose();
    incomeCategoryPieChart = null;
  }
  if (expenseCategoryPieChart) {
    expenseCategoryPieChart.dispose();
    expenseCategoryPieChart = null;
  }
});
</script>

<style scoped>
.income-expense-board {
  margin-bottom: 20px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.income-summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.summary-card {
  background: var(--app-surface-solid);
  border-radius: 16px;
  padding: 18px 20px;
  box-shadow: var(--app-shadow);
  border: 1px solid var(--app-border-soft);
}

.summary-card.highlight {
  background: var(--app-highlight-surface);
}

.income-summary-card {
  border-radius: 14px;
  padding: 14px 16px;
  border: 1px solid var(--app-border);
  background: var(--app-surface-muted);
}

.income-summary-card .summary-value {
  margin-top: 4px;
}

.income-summary-card.income .summary-value {
  color: var(--app-positive);
}

.income-summary-card.expense .summary-value {
  color: var(--app-negative);
}

.income-summary-card.net .summary-value {
  color: var(--app-info);
}

.income-board-content {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr;
  gap: 12px;
  align-items: stretch;
}

.group-charts {
  display: flex;
  gap: 16px;
  align-items: stretch;
}

.echart-wrapper {
  height: 260px;
}

.income-chart {
  width: 100%;
  min-width: 0;
  height: 320px;
}

.income-pie-card {
  border-radius: 14px;
  border: 1px solid var(--app-border);
  background: var(--app-surface-solid);
  padding: 10px 10px 8px;
  display: flex;
  flex-direction: column;
}

.pie-card-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text-primary);
  margin-bottom: 6px;
}

.pie-canvas {
  flex: 1;
  min-height: 320px;
}

.income-range-picker {
  min-width: 320px;
}

.chart-bar {
  flex: 1;
  min-width: 0;
}

.group-charts.with-pie .chart-bar {
  flex: 1;
}

.chart-pie {
  flex: 1;
  min-width: 0;
}

.chart-type-pie {
  flex: 1;
  min-width: 0;
}

.echart-canvas {
  width: 100%;
  height: 100%;
}

.summary-label {
  font-size: 12px;
  color: var(--app-text-secondary);
}

.summary-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--app-text-primary);
  margin-top: 6px;
}

.card + .card {
  margin-top: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.card-title {
  font-weight: 700;
  font-size: 16px;
}

.card-total {
  font-size: 14px;
  color: var(--app-text-secondary);
}

@media (max-width: 1100px) {
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .income-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .income-board-content {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .income-chart {
    grid-column: 1 / -1;
  }
}

@media (max-width: 720px) {
  .income-summary-grid {
    grid-template-columns: repeat(1, minmax(0, 1fr));
  }

  .income-range-picker {
    min-width: 260px;
  }

  .income-board-content {
    grid-template-columns: repeat(1, minmax(0, 1fr));
  }

  .income-chart {
    grid-column: auto;
  }

  .pie-canvas {
    min-height: 260px;
  }
}
</style>
