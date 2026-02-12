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
    <el-table :data="group.accounts" stripe border style="width: 100%">
      <el-table-column prop="name" label="账户" min-width="200" />
      <el-table-column label="类型" width="160">
        <template #default="{ row }">
          <span>{{ formatAccountType(row.type) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="currency" label="币种" width="120" />
      <el-table-column label="余额" width="180" align="right">
        <template #default="{ row }">{{ formatNumber(row.balance) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.is_active ? 'success' : 'info'" effect="light">
            {{ row.is_active ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { fetchBalanceSheet } from '@/api/report';
import { formatAccountType } from '@/types/account';
import type { BalanceSheetResponse, BalanceSheetGroup } from '@/types/report';

const report = ref<BalanceSheetResponse | null>(null);
const loading = ref(false);
const asOf = ref(formatDateISO(new Date()));

const reportGroups = computed<BalanceSheetGroup[]>(() => report.value?.groups ?? []);

const loadReport = async () => {
  loading.value = true;
  try {
    report.value = await fetchBalanceSheet({ as_of: asOf.value });
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

function formatNumber(value?: number): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return value.toFixed(2);
}

function formatDateISO(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

onMounted(loadReport);
</script>

<style scoped>
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.summary-card {
  background: #fff;
  border-radius: 16px;
  padding: 18px 20px;
  box-shadow: 0 8px 30px rgba(31, 47, 61, 0.08);
}

.summary-card.highlight {
  background: linear-gradient(135deg, rgba(34, 211, 238, 0.2), rgba(96, 165, 250, 0.2));
}

.summary-label {
  font-size: 12px;
  color: #6b7280;
}

.summary-value {
  font-size: 20px;
  font-weight: 700;
  color: #1f2937;
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
  color: #475569;
}

@media (max-width: 1100px) {
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
