<template>
  <div class="section-header">
    <div>
      <h1>账户快照</h1>
      <div class="light-text">管理期初余额快照（按账户+日期）</div>
    </div>
    <div class="toolbar">
      <el-select v-model="accountFilter" clearable placeholder="全部账户" style="min-width: 180px">
        <el-option v-for="account in accounts" :key="account.id" :label="account.name" :value="account.id" />
      </el-select>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建快照</el-button>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadSnapshots">刷新</el-button>
    </div>
  </div>

  <div class="card">
    <div class="table-scroll">
      <el-table :data="tableRows" stripe border v-loading="loading" style="width: 100%; min-width: 1100px">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="账户" min-width="220">
          <template #default="{ row }">
            <span>{{ row.accountName }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="asOf" label="日期" width="150" />
        <el-table-column label="期初余额" width="180" align="right">
          <template #default="{ row }">{{ formatNumber(row.amount) }}</template>
        </el-table-column>
        <el-table-column prop="note" label="备注" min-width="240" />
        <el-table-column label="创建时间" min-width="220">
          <template #default="{ row }">
            <span>{{ formatDate(row.createdAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" align="right">
          <template #default="{ row }">
            <div class="table-actions">
              <el-button size="small" :icon="Edit" @click="openEdit(row)">编辑</el-button>
              <el-button
                size="small"
                type="danger"
                :icon="Delete"
                :loading="deleting === row.id"
                @click="confirmDelete(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>

  <el-dialog v-model="dialogVisible" :title="dialogMode === 'edit' ? '编辑快照' : '新建快照'" width="540px" destroy-on-close>
    <AccountSnapshotForm
      v-model="formModel"
      :accounts="accounts"
      :mode="dialogMode"
      :loading="saving"
      @submit="handleSubmit"
    />
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Delete, Edit, Plus, RefreshRight } from '@element-plus/icons-vue';
import AccountSnapshotForm from '@/components/AccountSnapshotForm.vue';
import { createAccountSnapshot, deleteAccountSnapshot, fetchAccountSnapshots, toApiPayload, updateAccountSnapshot } from '@/api/snapshot';
import { fetchAccounts } from '@/api/account';
import type { Account } from '@/types/account';
import type { AccountSnapshot, AccountSnapshotFormInput } from '@/types/snapshot';

type TableRow = AccountSnapshot & { accountName: string };

const formatDateISO = (date: Date) => {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
};

function buildDefaultForm(): AccountSnapshotFormInput {
  return {
    accountId: 0,
    asOf: formatDateISO(new Date()),
    amount: 0,
    note: ''
  };
}

const snapshots = ref<AccountSnapshot[]>([]);
const accounts = ref<Account[]>([]);
const accountFilter = ref<number | null>(null);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<'create' | 'edit'>('create');
const formModel = ref<AccountSnapshotFormInput>(buildDefaultForm());
const selectedId = ref<number | null>(null);
const deleting = ref<number | null>(null);

const accountMap = computed(() => {
  const map = new Map<number, Account>();
  accounts.value.forEach((account) => map.set(account.id, account));
  return map;
});

const tableRows = computed<TableRow[]>(() =>
  snapshots.value
    .filter((item) => (accountFilter.value ? item.accountId === accountFilter.value : true))
    .map((item) => ({
    ...item,
    accountName: accountMap.value.get(item.accountId)?.name ?? `#${item.accountId}`
  }))
);

const loadSnapshots = async () => {
  loading.value = true;
  try {
    snapshots.value = await fetchAccountSnapshots();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

const loadAccounts = async () => {
  try {
    accounts.value = await fetchAccounts();
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
};

const openCreate = () => {
  dialogMode.value = 'create';
  selectedId.value = null;
  const defaultForm = buildDefaultForm();
  if (accounts.value.length > 0) {
    defaultForm.accountId = accounts.value[0].id;
  }
  formModel.value = defaultForm;
  dialogVisible.value = true;
};

const openEdit = (row: AccountSnapshot) => {
  dialogMode.value = 'edit';
  selectedId.value = row.id;
  formModel.value = {
    accountId: row.accountId,
    asOf: row.asOf,
    amount: row.amount,
    note: row.note ?? ''
  };
  dialogVisible.value = true;
};

const handleSubmit = async (payload: AccountSnapshotFormInput) => {
  saving.value = true;
  try {
    if (dialogMode.value === 'create') {
      const created = await createAccountSnapshot(toApiPayload(payload));
      snapshots.value = [created, ...snapshots.value];
      ElMessage.success('创建成功');
    } else if (selectedId.value !== null) {
      const updated = await updateAccountSnapshot(selectedId.value, {
        as_of: payload.asOf,
        amount: payload.amount,
        note: payload.note
      });
      snapshots.value = snapshots.value.map((item) => (item.id === updated.id ? updated : item));
      ElMessage.success('更新成功');
    }
    dialogVisible.value = false;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
};

const confirmDelete = (row: AccountSnapshot) => {
  ElMessageBox.confirm(`确认删除账户快照「${row.asOf}」？`, '提示', { type: 'warning' })
    .then(async () => {
      deleting.value = row.id;
      try {
        await deleteAccountSnapshot(row.id);
        snapshots.value = snapshots.value.filter((item) => item.id !== row.id);
        ElMessage.success('已删除');
      } catch (error) {
        ElMessage.error((error as Error).message);
      } finally {
        deleting.value = null;
      }
    })
    .catch(() => undefined);
};

const formatNumber = (value: number) => (Number.isFinite(value) ? value.toFixed(2) : '-');

const formatDate = (value?: string) => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
};

onMounted(() => {
  loadAccounts();
  loadSnapshots();
});
</script>

<style scoped>
.table-scroll {
  width: 100%;
  overflow-x: auto;
}
</style>
