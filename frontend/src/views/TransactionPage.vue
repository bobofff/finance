<template>
  <div class="section-header">
    <div>
      <h1>交易记录</h1>
      <div class="light-text">仅收支单据，按日期与分类查询</div>
    </div>
    <div class="toolbar toolbar-wrap">
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
        @change="reload"
      />
      <el-button type="primary" :icon="Plus" @click="openCreate">新增交易</el-button>
      <el-button :loading="loading" @click="loadTransactions">刷新</el-button>
    </div>
  </div>

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
        <el-table-column label="操作" width="200" align="right">
          <template #default="{ row }">
            <div class="table-actions">
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
    :title="dialogMode === 'edit' ? '编辑交易' : '新增交易'"
    :width="dialogWidth"
    destroy-on-close
    class="transaction-dialog"
  >
    <el-form :label-width="dialogLabelWidth" class="dialog-form">
      <el-form-item label="类型">
        <div class="tag-group">
          <el-button :type="form.kind === 'income' ? 'primary' : 'default'" plain @click="form.kind = 'income'">
            收入
          </el-button>
          <el-button :type="form.kind === 'expense' ? 'primary' : 'default'" plain @click="form.kind = 'expense'">
            支出
          </el-button>
        </div>
      </el-form-item>
      <el-form-item label="日期">
        <el-date-picker v-model="form.occurredOn" type="date" value-format="YYYY-MM-DD" />
      </el-form-item>
      <el-form-item label="账户">
        <div class="picker-field">
          <el-select v-model="form.accountId" filterable placeholder="选择账户" style="width: 100%">
            <el-option v-for="account in accounts" :key="account.id" :label="account.name" :value="account.id" />
          </el-select>
          <el-button class="picker-button" @click="openAccountDialog">新增</el-button>
        </div>
      </el-form-item>
      <el-form-item label="分类">
        <div class="picker-field">
          <el-input
            :model-value="selectedCategoryLabel"
            placeholder="选择分类"
            readonly
            @click="openCategoryPicker"
          />
          <el-button class="picker-button" @click="openCategoryPicker">选择</el-button>
        </div>
      </el-form-item>
      <el-form-item label="金额">
        <el-input-number v-model="form.amount" :min="0" :step="0.01" :precision="2" controls-position="right" class="amount-input" />
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" placeholder="可选：描述" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.note" placeholder="可选：备注" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">
          {{ dialogMode === 'edit' ? '保存修改' : '创建交易' }}
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
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Delete, Edit, Plus } from '@element-plus/icons-vue';
import { fetchTransactions, createTransaction, updateTransaction, deleteTransaction, toApiPayload } from '@/api/transaction';
import { createAccount, fetchAccounts } from '@/api/account';
import { createCategory, fetchCategories } from '@/api/category';
import { ACCOUNT_TYPES } from '@/types/account';
import type { Account } from '@/types/account';
import type { Category } from '@/types/category';
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

const kindFilter = ref<'income' | 'expense' | ''>('');
const accountFilter = ref<number | null>(null);
const categoryFilter = ref<number | null>(null);
const dateRange = ref<[string, string] | null>(null);
const categoryKeyword = ref('');

const dialogVisible = ref(false);
const dialogMode = ref<'create' | 'edit'>('create');
const editingId = ref<number | null>(null);
const isMobile = ref(window.innerWidth <= 900);
const dialogLabelWidth = computed(() => (isMobile.value ? '76px' : '100px'));
const dialogWidth = computed(() => (isMobile.value ? '80%' : '560px'));
const pickerDialogWidth = computed(() => (isMobile.value ? '86%' : '520px'));
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

const accountForm = reactive({
  name: '',
  type: ACCOUNT_TYPES[0].value,
  currency: 'CNY',
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

const loadMeta = async () => {
  try {
    const [accountList, categoryList] = await Promise.all([fetchAccounts(), fetchCategories()]);
    accounts.value = accountList;
    categories.value = categoryList;
    if (!form.accountId && accounts.value.length > 0) {
      form.accountId = accounts.value[0].id;
    }
    if (!form.categoryId && formCategories.value.length > 0) {
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

const openCreate = () => {
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

const openAccountDialog = () => {
  accountForm.name = '';
  accountForm.type = ACCOUNT_TYPES[0].value;
  accountForm.currency = 'CNY';
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
      name: accountForm.name.trim(),
      type: accountForm.type,
      currency: accountForm.currency?.trim() || 'CNY',
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

const submitForm = async () => {
  if (!form.occurredOn) {
    ElMessage.error('请选择日期');
    return;
  }
  if (!form.accountId) {
    ElMessage.error('请选择账户');
    return;
  }
  if (!form.categoryId) {
    ElMessage.error('请选择分类');
    return;
  }
  if (form.amount <= 0) {
    ElMessage.error('请输入金额');
    return;
  }

  saving.value = true;
  try {
    const payload = toApiPayload(form);
    if (dialogMode.value === 'edit' && editingId.value) {
      await updateTransaction(editingId.value, payload);
      ElMessage.success('更新成功');
    } else {
      await createTransaction(payload);
      ElMessage.success('创建成功');
    }
    dialogVisible.value = false;
    await loadTransactions();
  } catch (error) {
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

onMounted(() => {
  loadMeta();
  loadTransactions();
  window.addEventListener('resize', handleResize);
});

onBeforeUnmount(() => {
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
  color: #16a34a;
  font-weight: 600;
}

.amount-negative {
  color: #dc2626;
  font-weight: 600;
}

.toolbar-wrap {
  flex-wrap: wrap;
  gap: 12px;
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
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
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
