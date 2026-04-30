<template>
  <div class="section-header">
    <div>
      <h1>交易记录</h1>
      <div class="light-text">仅收支单据，按日期与分类查询</div>
    </div>
    <div class="toolbar toolbar-transactions">
      <div class="toolbar-filters">
        <el-select v-model="kindFilter" clearable placeholder="类型" style="min-width: 120px" @change="reload">
          <el-option label="收入" value="income" />
          <el-option label="支出" value="expense" />
        </el-select>
        <el-select v-model="accountFilter" clearable filterable placeholder="账户" style="min-width: 160px" @change="reload">
          <el-option v-for="account in accounts" :key="account.id" :label="account.name" :value="account.id" />
        </el-select>
        <el-select v-model="categoryFilter" clearable filterable placeholder="分类" style="min-width: 160px" @change="reload">
          <el-option v-for="category in filteredCategories" :key="category.id" :label="category.name" :value="category.id" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="min-width: 320px"
          @change="reload"
        />
      </div>
      <div class="toolbar-actions">
        <el-button @click="openAiDialog">AI 记账</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增交易</el-button>
        <el-button :loading="loading" @click="loadTransactions">刷新</el-button>
      </div>
    </div>
  </div>

  <el-dialog v-model="aiDialogVisible" title="AI 记账" :width="pickerDialogWidth" destroy-on-close class="picker-dialog">
    <el-form label-width="90px">
      <el-form-item label="原话">
        <el-input
          v-model="aiInput"
          type="textarea"
          :rows="6"
          maxlength="400"
          show-word-limit
          placeholder="例如：我今天早上买了一个煎饼果子，花了6元"
        />
      </el-form-item>
      <el-form-item label="提示">
        <div class="ai-dialog-tip">
          输入一句自然语言描述，系统会先生成草稿，你确认后才会入账。
        </div>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="aiDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="aiParsing" @click="submitAiParse">解析</el-button>
      </div>
    </template>
  </el-dialog>

  <div class="card">
    <div class="table-scroll">
      <el-table :data="rows" stripe border v-loading="loading" style="width: 100%; min-width: 1100px">
        <el-table-column prop="occurred_on" label="日期" width="120" />
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-tag :type="row.category_kind === 'income' ? 'success' : 'danger'" effect="light">
              {{ row.category_kind === 'income' ? '收入' : '支出' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="account_name" label="账户" min-width="180" />
        <el-table-column prop="category_name" label="分类" min-width="160" />
        <el-table-column label="金额" width="160" align="right">
          <template #default="{ row }">
            <span :class="row.category_kind === 'income' ? 'amount-positive' : 'amount-negative'">
              {{ formatAmount(row.amount) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="220" />
        <el-table-column prop="note" label="备注" min-width="200" />
        <el-table-column label="操作" width="280" align="right">
          <template #default="{ row }">
            <div class="table-actions">
              <el-button size="small" @click="openQuickCreateFromRow(row)">快速添加</el-button>
              <el-button size="small" :icon="Edit" @click="openEdit(row)">编辑</el-button>
              <el-button size="small" type="danger" :icon="Delete" :loading="deletingId === row.transaction_id" @click="confirmDelete(row)">
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="pagination-row">
      <el-pagination
        background
        layout="prev, pager, next, sizes, total"
        :page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :current-page="page"
        :total="total"
        @size-change="onPageSizeChange"
        @current-change="onPageChange"
      />
    </div>
  </div>

  <el-dialog
    v-model="dialogVisible"
    :title="transactionDialogTitle"
    :width="dialogWidth"
    destroy-on-close
    class="transaction-dialog"
  >
    <el-form :label-width="dialogLabelWidth" class="dialog-form">
      <div
        v-if="createSource === 'ai'"
        class="ai-draft-banner"
        :class="{
          'ai-draft-banner--countdown': aiAutoSubmitActive,
          'ai-draft-banner--cancelled': aiAutoSubmitCancelled && canAutoSubmitAiDraft,
          'ai-draft-banner--blocked': !canAutoSubmitAiDraft
        }"
      >
        <div class="ai-draft-banner-head">
          <div class="ai-draft-copy">
            <div class="ai-draft-banner-kicker">AI 智能草稿</div>
            <div class="ai-draft-banner-title">{{ aiDraftTitle }}</div>
            <div class="ai-draft-banner-subtitle">{{ aiDraftSubtitle }}</div>
          </div>
          <div class="ai-draft-banner-actions">
            <div v-if="aiAutoSubmitActive" class="ai-countdown-pill">
              <span class="ai-countdown-pill-label">自动入账</span>
              <strong>{{ aiAutoSubmitRemaining }}s</strong>
            </div>
            <el-button
              v-if="aiAutoSubmitActive"
              plain
              class="ai-cancel-button"
              @click="cancelAiAutoSubmit"
            >
              取消自动确认
            </el-button>
          </div>
        </div>

        <div class="ai-draft-summary">
          <div class="ai-draft-summary-item">
            <span>日期</span>
            <strong>{{ form.occurredOn || '待补充' }}</strong>
          </div>
          <div class="ai-draft-summary-item">
            <span>账户</span>
            <strong>{{ accounts.find((item) => item.id === form.accountId)?.name || '待补充' }}</strong>
          </div>
          <div class="ai-draft-summary-item">
            <span>分类</span>
            <strong>{{ selectedCategoryLabel || '待补充' }}</strong>
          </div>
          <div class="ai-draft-summary-item ai-draft-summary-item--amount">
            <span>金额</span>
            <strong :class="form.kind === 'income' ? 'amount-positive' : 'amount-negative'">
              {{ formatAmount(form.amount || 0) }}
            </strong>
          </div>
        </div>

        <div v-if="aiAutoSubmitActive" class="ai-countdown-track">
          <div class="ai-countdown-track-fill" :style="{ width: `${aiAutoSubmitProgress}%` }" />
        </div>

        <div v-if="aiSourceText" class="ai-draft-banner-line">原话：{{ aiSourceText }}</div>
        <div v-if="aiDraftMissingFields.length" class="ai-draft-banner-line ai-draft-banner-line--warning">
          待补信息：{{ formatFieldLabels(aiDraftMissingFields) }}
        </div>
        <div v-if="aiDraftLowConfidenceFields.length" class="ai-draft-banner-line ai-draft-banner-line--muted">
          建议重点核对：{{ formatFieldLabels(aiDraftLowConfidenceFields) }}
        </div>
      </div>
      <el-form-item label="类型">
        <div class="tag-group">
          <el-button :type="form.kind === 'income' ? 'primary' : 'default'" :disabled="isAiDraftLocked" plain @click="form.kind = 'income'">
            收入
          </el-button>
          <el-button :type="form.kind === 'expense' ? 'primary' : 'default'" :disabled="isAiDraftLocked" plain @click="form.kind = 'expense'">
            支出
          </el-button>
        </div>
      </el-form-item>
      <el-form-item label="日期">
        <el-date-picker v-model="form.occurredOn" type="date" value-format="YYYY-MM-DD" :disabled="isAiDraftLocked" />
      </el-form-item>
      <el-form-item label="账户">
        <div class="account-picker-block">
          <div class="picker-field">
            <el-select :key="accountSelectVersion" v-model="form.accountId" filterable placeholder="选择账户" style="width: 100%" :disabled="isAiDraftLocked">
              <el-option v-for="account in accounts" :key="account.id" :label="account.name" :value="account.id" />
            </el-select>
            <el-button class="picker-button" :disabled="isAiDraftLocked" @click="openAccountDialog">新增</el-button>
          </div>
        </div>
      </el-form-item>
      <el-form-item label="分类">
        <div class="picker-field">
          <el-input
            :model-value="selectedCategoryLabel"
            placeholder="选择分类"
            readonly
            :disabled="isAiDraftLocked"
            @click="openCategoryPicker"
          />
          <el-button class="picker-button" :disabled="isAiDraftLocked" @click="openCategoryPicker">选择</el-button>
        </div>
      </el-form-item>
      <el-form-item label="金额">
        <el-input-number
          v-model="form.amount"
          :min="0"
          :step="0.01"
          :precision="2"
          controls-position="right"
          class="amount-input"
          :disabled="isAiDraftLocked"
        />
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" placeholder="可选：描述" :disabled="isAiDraftLocked" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.note" placeholder="可选：备注" :disabled="isAiDraftLocked" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm()">
          {{ submitButtonText }}
        </el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="categoryPickerVisible" title="选择分类" :width="pickerDialogWidth" destroy-on-close class="picker-dialog">
    <div class="picker-header">
      <el-input v-model="categoryKeyword" placeholder="搜索分类" clearable />
    </div>
    <div class="tag-grid tag-grid-scroll">
      <el-button
        v-for="category in filteredFormCategories"
        :key="category.id"
        :type="form.categoryId === category.id ? 'primary' : 'default'"
        plain
        class="tag-option"
        @click="selectCategory(category.id)"
      >
        {{ category.name }}
      </el-button>
    </div>

    <template #footer>
      <div class="dialog-footer picker-footer">
        <div class="picker-footer-row picker-footer-row-3">
          <el-button @click="categoryPickerVisible = false">取消</el-button>
          <el-button @click="openCategoryDialog">新增分类</el-button>
          <el-button type="primary" @click="categoryPickerVisible = false">完成</el-button>
        </div>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="accountDialogVisible" title="新增账户" :width="pickerDialogWidth" destroy-on-close class="picker-dialog">
    <el-form label-width="90px">
      <el-form-item label="名称">
        <el-input v-model="accountForm.name" placeholder="账户名称" />
      </el-form-item>
      <el-form-item label="类型">
        <el-select v-model="accountForm.type" placeholder="选择类型" style="width: 100%">
          <el-option v-for="item in ACCOUNT_TYPES" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="accountForm.type === 'cash'" label="现金类型">
        <el-select v-model="accountForm.cashKind" placeholder="选择现金类型" style="width: 100%">
          <el-option v-for="item in CASH_KINDS" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="币种">
        <el-input v-model="accountForm.currency" placeholder="默认 CNY" />
      </el-form-item>
      <el-form-item label="启用">
        <el-switch v-model="accountForm.isActive" active-text="启用" inactive-text="停用" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="accountDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="accountSaving" @click="submitAccount">保存</el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="categoryDialogVisible" title="新增分类" :width="pickerDialogWidth" destroy-on-close class="picker-dialog">
    <el-form label-width="90px">
      <el-form-item label="名称">
        <el-input v-model="categoryForm.name" placeholder="分类名称" />
      </el-form-item>
      <el-form-item label="类型">
        <el-select v-model="categoryForm.kind" placeholder="选择类型" style="width: 100%">
          <el-option label="收入" value="income" />
          <el-option label="支出" value="expense" />
        </el-select>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="categoryDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="categorySaving" @click="submitCategory">保存</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Delete, Edit, Plus } from '@element-plus/icons-vue';
import { parseTransactionDraft } from '@/api/ai';
import { fetchTransactions, createTransaction, updateTransaction, deleteTransaction, toApiPayload } from '@/api/transaction';
import { createAccount, fetchAccounts } from '@/api/account';
import { createCategory, fetchCategories } from '@/api/category';
import { ACCOUNT_TYPES, CASH_KINDS } from '@/types/account';
import type { Account, CashKind } from '@/types/account';
import type { Category } from '@/types/category';
import type { AiParseTransactionResponse } from '@/types/ai';
import type { TransactionFormInput, TransactionRow } from '@/types/transaction';

const accounts = ref<Account[]>([]);
const categories = ref<Category[]>([]);
const rows = ref<TransactionRow[]>([]);
const total = ref(0);
const loading = ref(false);
const saving = ref(false);
const deletingId = ref<number | null>(null);
const page = ref(1);
const pageSize = ref(20);
const currentLedgerId = 1;

const kindFilter = ref<'income' | 'expense' | ''>('');
const accountFilter = ref<number | null>(null);
const categoryFilter = ref<number | null>(null);
const dateRange = ref<[string, string] | null>(null);
const categoryKeyword = ref('');

const aiDialogVisible = ref(false);
const aiInput = ref('');
const aiParsing = ref(false);
const createSource = ref<'manual' | 'ai'>('manual');
const aiSourceText = ref('');
const aiDraftAccountId = ref<number | null>(null);
const aiDraftAccountName = ref('');
const aiDraftCategoryId = ref<number | null>(null);
const aiDraftCategoryName = ref('');
const aiDraftMissingFields = ref<string[]>([]);
const aiDraftLowConfidenceFields = ref<string[]>([]);
const accountSelectVersion = ref(0);
const AI_AUTO_SUBMIT_SECONDS = 10;
const aiAutoSubmitRemaining = ref(AI_AUTO_SUBMIT_SECONDS);
const aiAutoSubmitActive = ref(false);
const aiAutoSubmitCancelled = ref(false);
let aiAutoSubmitTimer: number | null = null;

const dialogVisible = ref(false);
const dialogMode = ref<'create' | 'edit'>('create');
const editingId = ref<number | null>(null);
const isMobile = ref(window.innerWidth <= 900);
const dialogLabelWidth = computed(() => (isMobile.value ? '76px' : '100px'));
const dialogWidth = computed(() => (isMobile.value ? '80%' : '560px'));
const pickerDialogWidth = computed(() => (isMobile.value ? '86%' : '520px'));
const transactionDialogTitle = computed(() => {
  if (dialogMode.value === 'edit') {
    return '编辑交易';
  }
  return createSource.value === 'ai' ? 'AI 预填交易' : '新增交易';
});
const submitButtonText = computed(() => {
  if (dialogMode.value === 'edit') {
    return '保存修改';
  }
  if (createSource.value === 'ai') {
    return aiAutoSubmitActive.value ? '立即入账' : '手动确认入账';
  }
  return '创建交易';
});
const categoryPickerVisible = ref(false);
const accountDialogVisible = ref(false);
const categoryDialogVisible = ref(false);
const accountSaving = ref(false);
const categorySaving = ref(false);

const form = reactive<TransactionFormInput>({
  kind: 'income',
  occurredOn: formatDateISO(new Date()),
  accountId: 0,
  categoryId: 0,
  amount: 0,
  description: '',
  note: ''
});

const accountForm = reactive<{
  name: string;
  type: (typeof ACCOUNT_TYPES)[number]['value'];
  currency: string;
  cashKind: CashKind;
  isActive: boolean;
}>({
  name: '',
  type: ACCOUNT_TYPES[0].value as (typeof ACCOUNT_TYPES)[number]['value'],
  currency: 'CNY',
  cashKind: 'bank' as CashKind,
  isActive: true
});

const categoryForm = reactive({
  name: '',
  kind: 'income' as 'income' | 'expense'
});

const filteredCategories = computed(() => {
  if (!kindFilter.value) return categories.value;
  return categories.value.filter((item) => item.kind === kindFilter.value);
});

const formCategories = computed(() => categories.value.filter((item) => item.kind === form.kind));
const filteredFormCategories = computed(() => {
  const list = formCategories.value;
  const keyword = categoryKeyword.value.trim().toLowerCase();
  if (!keyword) return list;
  return list.filter((item) => item.name.toLowerCase().includes(keyword));
});
const selectedCategoryLabel = computed(() => {
  const selected = categories.value.find((item) => item.id === form.categoryId);
  return selected?.name ?? '';
});
const canAutoSubmitAiDraft = computed(() => {
  if (createSource.value !== 'ai' || dialogMode.value !== 'create') {
    return false;
  }
  return Boolean(form.occurredOn) && form.accountId > 0 && form.categoryId > 0 && form.amount > 0;
});
const isAiDraftLocked = computed(() => createSource.value === 'ai' && dialogMode.value === 'create' && aiAutoSubmitActive.value);
const aiAutoSubmitProgress = computed(() => {
  if (!aiAutoSubmitActive.value) {
    return 0;
  }
  return (aiAutoSubmitRemaining.value / AI_AUTO_SUBMIT_SECONDS) * 100;
});
const aiDraftTitle = computed(() => {
  if (!canAutoSubmitAiDraft.value) {
    return '草稿已生成，还需要补全关键信息';
  }
  if (aiAutoSubmitActive.value) {
    return '草稿已核对完毕，准备自动入账';
  }
  return '自动确认已暂停';
});
const aiDraftSubtitle = computed(() => {
  if (!canAutoSubmitAiDraft.value) {
    return '这笔草稿暂时还不能自动入账，请先补齐必填信息，再手动确认。';
  }
  if (aiAutoSubmitActive.value) {
    return `如果这笔记录无需调整，系统会在 ${aiAutoSubmitRemaining.value} 秒后自动完成入账。需要修改时，先取消自动确认。`;
  }
  return '你现在可以修改账户、分类、金额或备注，确认无误后再手动入账。';
});

const fieldLabelMap: Record<string, string> = {
  account: '账户',
  amount: '金额',
  category: '分类',
  description: '描述',
  kind: '类型',
  note: '备注',
  occurred_on: '日期'
};

const fieldOrder = ['occurred_on', 'account', 'category', 'amount', 'kind', 'description', 'note'];

const formatFieldLabels = (fields: string[]) =>
  [...fields]
    .sort((left, right) => {
      const leftIndex = fieldOrder.indexOf(left);
      const rightIndex = fieldOrder.indexOf(right);
      if (leftIndex === rightIndex) return left.localeCompare(right);
      if (leftIndex === -1) return 1;
      if (rightIndex === -1) return -1;
      return leftIndex - rightIndex;
    })
    .map((field) => fieldLabelMap[field] ?? field)
    .join('、');

const normalizeLookup = (value: string) => value.trim().toLowerCase();

const resolveAiAccountId = () => {
  const draftAccountId = aiDraftAccountId.value;
  if (draftAccountId && accounts.value.some((item) => item.id === draftAccountId)) {
    return draftAccountId;
  }

  const draftAccountName = normalizeLookup(aiDraftAccountName.value);
  if (draftAccountName) {
    const matched = accounts.value.find((item) => normalizeLookup(item.name) === draftAccountName);
    if (matched) {
      return matched.id;
    }
  }

  return draftAccountId ?? 0;
};

const resolveAiCategoryId = () => {
  const draftCategoryId = aiDraftCategoryId.value;
  if (draftCategoryId && categories.value.some((item) => item.id === draftCategoryId)) {
    return draftCategoryId;
  }

  const draftCategoryName = normalizeLookup(aiDraftCategoryName.value);
  if (draftCategoryName) {
    const matched = categories.value.find((item) => normalizeLookup(item.name) === draftCategoryName);
    if (matched) {
      return matched.id;
    }
  }

  return draftCategoryId ?? 0;
};

const syncAiDraftSelection = () => {
  if (createSource.value !== 'ai') {
    return;
  }

  const accountId = resolveAiAccountId();
  const categoryId = resolveAiCategoryId();

  accountSelectVersion.value += 1;
  if (accountId && form.accountId !== accountId) {
    form.accountId = accountId;
  }
  if (categoryId && form.categoryId !== categoryId) {
    form.categoryId = categoryId;
  }
};

const clearAiAutoSubmitTimer = () => {
  if (aiAutoSubmitTimer !== null) {
    window.clearInterval(aiAutoSubmitTimer);
    aiAutoSubmitTimer = null;
  }
};

const resetAiAutoSubmitState = () => {
  clearAiAutoSubmitTimer();
  aiAutoSubmitActive.value = false;
  aiAutoSubmitCancelled.value = false;
  aiAutoSubmitRemaining.value = AI_AUTO_SUBMIT_SECONDS;
};

const stopAiAutoSubmit = (cancelled = false) => {
  clearAiAutoSubmitTimer();
  aiAutoSubmitActive.value = false;
  aiAutoSubmitCancelled.value = cancelled;
  aiAutoSubmitRemaining.value = AI_AUTO_SUBMIT_SECONDS;
};

const cancelAiAutoSubmit = () => {
  if (!aiAutoSubmitActive.value) {
    return;
  }
  stopAiAutoSubmit(true);
  ElMessage.info('自动确认已取消，你可以先调整草稿，确认无误后再手动入账。');
};

const startAiAutoSubmit = () => {
  if (!canAutoSubmitAiDraft.value) {
    stopAiAutoSubmit(false);
    return;
  }

  clearAiAutoSubmitTimer();
  aiAutoSubmitActive.value = true;
  aiAutoSubmitCancelled.value = false;
  const deadline = Date.now() + AI_AUTO_SUBMIT_SECONDS * 1000;

  const syncRemaining = () => {
    aiAutoSubmitRemaining.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
  };

  syncRemaining();
  aiAutoSubmitTimer = window.setInterval(() => {
    syncRemaining();
    if (aiAutoSubmitRemaining.value > 0 || saving.value) {
      return;
    }
    clearAiAutoSubmitTimer();
    aiAutoSubmitActive.value = false;
    void submitForm('auto');
  }, 200);
};

const loadMeta = async () => {
  try {
    const [accountList, categoryList] = await Promise.all([fetchAccounts(), fetchCategories()]);
    accounts.value = accountList;
    categories.value = categoryList;
    syncAiDraftSelection();
    if (createSource.value !== 'ai' && !form.accountId && accounts.value.length > 0) {
      form.accountId = accounts.value[0].id;
    }
    if (createSource.value !== 'ai' && !form.categoryId && formCategories.value.length > 0) {
      form.categoryId = formCategories.value[0].id;
    }
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
};

const loadTransactions = async () => {
  loading.value = true;
  try {
    const params = {
      ledger_id: currentLedgerId,
      page: page.value,
      page_size: pageSize.value,
      kind: kindFilter.value || undefined,
      account_id: accountFilter.value ?? undefined,
      category_id: categoryFilter.value ?? undefined,
      date_from: dateRange.value?.[0],
      date_to: dateRange.value?.[1]
    };
    const resp = await fetchTransactions(params);
    rows.value = resp.data;
    total.value = resp.total;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

const reload = () => {
  page.value = 1;
  loadTransactions();
};

const resetCreateForm = () => {
  resetAiAutoSubmitState();
  createSource.value = 'manual';
  aiSourceText.value = '';
  aiDraftAccountId.value = null;
  aiDraftAccountName.value = '';
  aiDraftCategoryId.value = null;
  aiDraftCategoryName.value = '';
  aiDraftMissingFields.value = [];
  aiDraftLowConfidenceFields.value = [];
};

const openCreate = () => {
  resetCreateForm();
  dialogMode.value = 'create';
  editingId.value = null;
  form.kind = 'income';
  form.occurredOn = formatDateISO(new Date());
  form.accountId = accounts.value[0]?.id || 0;
  form.categoryId = formCategories.value[0]?.id || 0;
  form.amount = 0;
  form.description = '';
  form.note = '';
  dialogVisible.value = true;
};

const openEdit = (row: TransactionRow) => {
  resetCreateForm();
  dialogMode.value = 'edit';
  editingId.value = row.transaction_id;
  form.kind = row.category_kind;
  form.occurredOn = row.occurred_on;
  form.accountId = row.account_id;
  form.categoryId = row.category_id;
  form.amount = Math.abs(row.amount);
  form.description = row.description || '';
  form.note = row.note || '';
  dialogVisible.value = true;
};

const openQuickCreateFromRow = (row: TransactionRow) => {
  resetCreateForm();
  dialogMode.value = 'create';
  editingId.value = null;
  form.kind = row.category_kind;
  form.occurredOn = row.occurred_on || formatDateISO(new Date());
  form.accountId = accounts.value.some((item) => item.id === row.account_id) ? row.account_id : accounts.value[0]?.id || 0;
  const matchedCategory = categories.value.find((item) => item.id === row.category_id && item.kind === row.category_kind);
  form.categoryId = matchedCategory?.id || categories.value.find((item) => item.kind === row.category_kind)?.id || 0;
  form.amount = Math.abs(row.amount);
  form.description = row.description || '';
  form.note = row.note || '';
  dialogVisible.value = true;
};

const openAiDialog = () => {
  aiInput.value = '';
  aiDialogVisible.value = true;
};

const applyAiDraft = (response: AiParseTransactionResponse) => {
  const draft = response.draft;
  resetCreateForm();
  createSource.value = 'ai';
  dialogMode.value = 'create';
  editingId.value = null;
  aiSourceText.value = response.source_text;
  aiDraftAccountId.value = draft.account_id ?? null;
  aiDraftAccountName.value = draft.account_name || '';
  aiDraftCategoryId.value = draft.category_id ?? null;
  aiDraftCategoryName.value = draft.category_name || '';
  aiDraftMissingFields.value = draft.missing_fields ?? [];
  aiDraftLowConfidenceFields.value = draft.low_confidence_fields ?? [];
  form.kind = draft.kind === 'expense' || draft.amount < 0 ? 'expense' : 'income';
  form.occurredOn = draft.occurred_on || formatDateISO(new Date());
  form.accountId = resolveAiAccountId();
  form.categoryId = resolveAiCategoryId();
  form.amount = Math.abs(draft.amount);
  form.description = draft.description || '';
  form.note = draft.note || '';
  syncAiDraftSelection();
};

const submitAiParse = async () => {
  const text = aiInput.value.trim();
  if (!text) {
    ElMessage.error('请输入要解析的内容');
    return;
  }

  aiParsing.value = true;
  try {
    const [response] = await Promise.all([
      parseTransactionDraft({
        ledger_id: currentLedgerId,
        text
      }),
      loadMeta()
    ]);
    applyAiDraft(response);
    aiDialogVisible.value = false;
    await nextTick();
    dialogVisible.value = true;
    await nextTick();
    startAiAutoSubmit();
    ElMessage.success(
      canAutoSubmitAiDraft.value
        ? 'AI 草稿已生成，默认 10 秒后自动入账；如需调整，请先取消自动确认。'
        : 'AI 草稿已生成，请补全后再确认入账。'
    );
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    aiParsing.value = false;
  }
};

const openAccountDialog = () => {
  accountForm.name = '';
  accountForm.type = ACCOUNT_TYPES[0].value;
  accountForm.currency = 'CNY';
  accountForm.cashKind = 'bank';
  accountForm.isActive = true;
  accountDialogVisible.value = true;
};

const openCategoryDialog = () => {
  categoryForm.name = '';
  categoryForm.kind = form.kind;
  categoryDialogVisible.value = true;
};

const openCategoryPicker = () => {
  categoryKeyword.value = '';
  categoryPickerVisible.value = true;
};

const selectCategory = (id: number) => {
  form.categoryId = id;
  if (isMobile.value) {
    categoryPickerVisible.value = false;
  }
};

const submitAccount = async () => {
  if (!accountForm.name.trim()) {
    ElMessage.error('请输入账户名称');
    return;
  }
  accountSaving.value = true;
  try {
    const created = await createAccount({
      ledger_id: currentLedgerId,
      name: accountForm.name.trim(),
      type: accountForm.type,
      currency: accountForm.currency?.trim() || 'CNY',
      cash_kind: accountForm.type === 'cash' ? accountForm.cashKind : undefined,
      is_active: accountForm.isActive
    });
    accounts.value = [...accounts.value, created];
    form.accountId = created.id;
    accountDialogVisible.value = false;
    ElMessage.success('账户已创建');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    accountSaving.value = false;
  }
};

const submitCategory = async () => {
  if (!categoryForm.name.trim()) {
    ElMessage.error('请输入分类名称');
    return;
  }
  categorySaving.value = true;
  try {
    const created = await createCategory({
      ledger_id: currentLedgerId,
      name: categoryForm.name.trim(),
      kind: categoryForm.kind
    });
    categories.value = [...categories.value, created];
    form.categoryId = created.id;
    categoryDialogVisible.value = false;
    ElMessage.success('分类已创建');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    categorySaving.value = false;
  }
};

const submitForm = async (mode: 'manual' | 'auto' = 'manual') => {
  if (aiAutoSubmitActive.value) {
    stopAiAutoSubmit(false);
  }

  if (!form.occurredOn) {
    if (mode === 'auto') {
      aiAutoSubmitCancelled.value = true;
    }
    ElMessage.error('请选择日期');
    return;
  }
  if (!form.accountId) {
    if (mode === 'auto') {
      aiAutoSubmitCancelled.value = true;
    }
    ElMessage.error('请选择账户');
    return;
  }
  if (!form.categoryId) {
    if (mode === 'auto') {
      aiAutoSubmitCancelled.value = true;
    }
    ElMessage.error('请选择分类');
    return;
  }
  if (form.amount <= 0) {
    if (mode === 'auto') {
      aiAutoSubmitCancelled.value = true;
    }
    ElMessage.error('请输入金额');
    return;
  }

  saving.value = true;
  try {
    const payload = toApiPayload(form, currentLedgerId);
    if (dialogMode.value === 'edit' && editingId.value) {
      await updateTransaction(editingId.value, payload);
      ElMessage.success('更新成功');
    } else {
      await createTransaction(payload);
      if (mode === 'auto') {
        ElMessage.success('已自动入账');
      } else if (createSource.value === 'ai') {
        ElMessage.success('入账成功');
      } else {
        ElMessage.success('创建成功');
      }
    }
    resetCreateForm();
    dialogVisible.value = false;
    await loadTransactions();
  } catch (error) {
    if (mode === 'auto') {
      aiAutoSubmitCancelled.value = true;
    }
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
};

const confirmDelete = (row: TransactionRow) => {
  ElMessageBox.confirm(`确认删除 ${row.category_name} ${row.occurred_on} 的记录？`, '提示', { type: 'warning' })
    .then(async () => {
      deletingId.value = row.transaction_id;
      try {
        await deleteTransaction(row.transaction_id);
        rows.value = rows.value.filter((item) => item.transaction_id !== row.transaction_id);
        total.value = Math.max(total.value - 1, 0);
        ElMessage.success('已删除');
      } catch (error) {
        ElMessage.error((error as Error).message);
      } finally {
        deletingId.value = null;
      }
    })
    .catch(() => undefined);
};

const onPageChange = (nextPage: number) => {
  page.value = nextPage;
  loadTransactions();
};

const onPageSizeChange = (nextSize: number) => {
  pageSize.value = nextSize;
  page.value = 1;
  loadTransactions();
};

function formatAmount(value: number): string {
  return value.toFixed(2);
}

function formatDateISO(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

const handleResize = () => {
  isMobile.value = window.innerWidth <= 900;
};

watch([accounts, categories], () => {
  syncAiDraftSelection();
}, { flush: 'post' });

watch(dialogVisible, (visible) => {
  if (!visible) {
    resetAiAutoSubmitState();
  }
});

onMounted(() => {
  loadMeta();
  loadTransactions();
  window.addEventListener('resize', handleResize);
});

onBeforeUnmount(() => {
  clearAiAutoSubmitTimer();
  window.removeEventListener('resize', handleResize);
});
</script>

<style scoped>
.transaction-dialog :deep(.el-dialog) {
  width: 560px;
}

.transaction-dialog :deep(.el-dialog__body) {
  padding-top: 8px;
}

.table-scroll {
  width: 100%;
  overflow-x: auto;
}

.pagination-row {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.amount-positive {
  color: var(--app-positive);
  font-weight: 600;
}

.amount-negative {
  color: var(--app-negative);
  font-weight: 600;
}

.toolbar-transactions {
  flex-direction: row;
  align-items: center;
  flex-wrap: wrap;
}

.toolbar-filters {
  display: flex;
  gap: 10px;
  flex: 1;
  flex-wrap: nowrap;
  min-width: 560px;
}

.toolbar-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.ai-dialog-tip {
  color: var(--app-muted, #6b7280);
  line-height: 1.6;
  font-size: 13px;
}

.ai-draft-banner {
  border: 1px solid rgba(14, 116, 144, 0.16);
  background:
    radial-gradient(circle at top right, rgba(16, 185, 129, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(240, 249, 255, 0.94));
  border-radius: 20px;
  padding: 16px;
  margin-bottom: 16px;
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.06);
}

.ai-draft-banner--countdown {
  border-color: rgba(14, 116, 144, 0.24);
}

.ai-draft-banner--cancelled {
  border-color: rgba(245, 158, 11, 0.28);
  background:
    radial-gradient(circle at top right, rgba(251, 191, 36, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(255, 251, 235, 0.98), rgba(255, 247, 237, 0.96));
}

.ai-draft-banner--blocked {
  border-color: rgba(249, 115, 22, 0.28);
  background:
    radial-gradient(circle at top right, rgba(251, 146, 60, 0.15), transparent 34%),
    linear-gradient(180deg, rgba(255, 251, 235, 0.98), rgba(255, 245, 245, 0.94));
}

.ai-draft-banner-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.ai-draft-copy {
  flex: 1;
  min-width: 0;
}

.ai-draft-banner-kicker {
  color: #0f766e;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.ai-draft-banner-title {
  font-weight: 700;
  color: var(--app-text, #111827);
  font-size: 16px;
  margin-top: 6px;
}

.ai-draft-banner-subtitle {
  margin-top: 8px;
  color: var(--app-muted, #64748b);
  line-height: 1.65;
  font-size: 13px;
}

.ai-draft-banner-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
}

.ai-countdown-pill {
  min-width: 108px;
  padding: 10px 14px;
  border-radius: 999px;
  background: linear-gradient(135deg, #0f766e, #0ea5e9);
  color: #f8fafc;
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 10px;
  box-shadow: 0 10px 18px rgba(14, 116, 144, 0.18);
}

.ai-countdown-pill-label {
  font-size: 11px;
  opacity: 0.92;
}

.ai-countdown-pill strong {
  font-size: 22px;
  line-height: 1;
}

.ai-cancel-button {
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
}

.ai-draft-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin: 16px 0 12px;
}

.ai-draft-summary-item {
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.78);
  border-radius: 14px;
  padding: 10px 12px;
  min-width: 0;
}

.ai-draft-summary-item span {
  display: block;
  font-size: 12px;
  color: var(--app-muted, #64748b);
  margin-bottom: 6px;
}

.ai-draft-summary-item strong {
  display: block;
  color: var(--app-text, #0f172a);
  font-size: 13px;
  line-height: 1.45;
  word-break: break-word;
}

.ai-draft-summary-item--amount strong {
  font-size: 16px;
}

.ai-countdown-track {
  height: 8px;
  border-radius: 999px;
  overflow: hidden;
  background: rgba(148, 163, 184, 0.2);
  margin-bottom: 12px;
}

.ai-countdown-track-fill {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #10b981, #0ea5e9);
  transition: width 0.2s linear;
}

.ai-draft-banner-line {
  color: var(--app-muted, #6b7280);
  line-height: 1.55;
  font-size: 13px;
  word-break: break-word;
}

.ai-draft-banner-line + .ai-draft-banner-line {
  margin-top: 6px;
}

.ai-draft-banner-line--warning {
  color: #b45309;
}

.ai-draft-banner-line--muted {
  color: #475569;
}

@media (max-width: 900px) {
  .pagination-row {
    justify-content: center;
  }
}

.tag-group {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.tag-grid {
  margin-top: 8px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tag-grid-scroll {
  max-height: 320px;
  overflow-y: auto;
  padding-top: 8px;
}

.tag-option {
  border-radius: 12px;
  padding: 4px 12px;
}

.picker-field {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
  min-width: 0;
}

.account-picker-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.picker-button {
  height: 32px;
}

.picker-dialog :deep(.el-dialog__body) {
  padding-top: 12px;
}

.picker-header {
  display: flex;
  gap: 8px;
  align-items: center;
}

.picker-footer-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.picker-footer-row-3 {
  grid-template-columns: 1fr 1fr 1fr;
}

.amount-input {
  width: 100%;
}

@media (max-width: 900px) {
  .transaction-dialog :deep(.el-dialog) {
    width: 80% !important;
    max-width: 360px;
    margin: 10px auto;
  }

  .transaction-dialog :deep(.el-dialog__body) {
    padding: 12px 14px 4px;
  }

  .tag-grid {
    gap: 6px;
  }

  .ai-draft-banner {
    border-radius: 18px;
    padding: 14px;
  }

  .ai-draft-banner-head {
    flex-direction: column;
  }

  .ai-draft-banner-actions {
    width: 100%;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }

  .ai-countdown-pill {
    min-width: 0;
  }

  .ai-draft-summary {
    grid-template-columns: 1fr 1fr;
  }

  .tag-option {
    padding: 4px 10px;
  }

  .dialog-form :deep(.el-form-item) {
    margin-bottom: 12px;
  }

  .dialog-footer {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .dialog-footer :deep(.el-button) {
    width: 100%;
  }

  .picker-dialog :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
    margin: 12px auto;
  }

  .tag-grid-scroll {
    max-height: 360px;
  }

  .picker-header {
    flex-direction: column;
    align-items: stretch;
  }

  .picker-footer {
    display: block;
  }

  .picker-footer-row-3 {
    grid-template-columns: 1fr 1fr 1fr;
  }

  .picker-footer-row :deep(.el-button) {
    width: 100%;
  }
}
</style>
