<template>
  <div class="section-header">
    <div>
      <h1>账户管理</h1>
      <div class="light-text">基于后端 /api/accounts 提供的 CRUD 界面</div>
    </div>
    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="openCreate">新建账户</el-button>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadAccounts">刷新</el-button>
    </div>
  </div>

  <div class="card">
    <el-table :data="accounts" stripe border v-loading="loading" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="类型" min-width="140">
        <template #default="{ row }">
          <span class="badge">{{ formatAccountType(row.type) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="currency" label="币种" width="120" />
      <el-table-column label="启用" width="160">
        <template #default="{ row }">
          <el-switch
            :model-value="row.isActive"
            :loading="rowUpdating === row.id"
            active-text="启用"
            inactive-text="停用"
            @change="(val: boolean) => toggleActive(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="创建时间" min-width="200">
        <template #default="{ row }">
          <span>{{ formatDate(row.createdAt) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" align="right">
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

  <el-dialog v-model="dialogVisible" :title="dialogMode === 'edit' ? '编辑账户' : '新建账户'" width="540px" destroy-on-close>
    <AccountForm v-model="formModel" :mode="dialogMode" :loading="saving" @submit="handleSubmit" />
  </el-dialog>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Delete, Edit, Plus, RefreshRight } from '@element-plus/icons-vue';
import AccountForm from '@/components/AccountForm.vue';
import { createAccount, deleteAccount, fetchAccounts, toApiPayload, updateAccount } from '@/api/account';
import { ACCOUNT_TYPES, type Account, type AccountFormInput, formatAccountType } from '@/types/account';

const accounts = ref<Account[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<'create' | 'edit'>('create');
const formModel = ref<AccountFormInput>(buildDefaultForm());
const selectedId = ref<number | null>(null);
const rowUpdating = ref<number | null>(null);
const deleting = ref<number | null>(null);

function buildDefaultForm(): AccountFormInput {
  return {
    name: '',
    type: ACCOUNT_TYPES[0].value,
    currency: 'CNY',
    isActive: true
  };
}

const loadAccounts = async () => {
  loading.value = true;
  try {
    accounts.value = await fetchAccounts();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
};

const openCreate = () => {
  dialogMode.value = 'create';
  selectedId.value = null;
  formModel.value = buildDefaultForm();
  dialogVisible.value = true;
};

const openEdit = (account: Account) => {
  dialogMode.value = 'edit';
  selectedId.value = account.id;
  formModel.value = {
    name: account.name,
    type: account.type,
    currency: account.currency,
    isActive: account.isActive
  };
  dialogVisible.value = true;
};

const handleSubmit = async (payload: AccountFormInput) => {
  saving.value = true;
  try {
    if (dialogMode.value === 'create') {
      const created = await createAccount(toApiPayload(payload));
      accounts.value = [...accounts.value, created];
      ElMessage.success('创建成功');
    } else if (selectedId.value !== null) {
      const updated = await updateAccount(selectedId.value, toApiPayload(payload));
      accounts.value = accounts.value.map((item) => (item.id === updated.id ? updated : item));
      ElMessage.success('更新成功');
    }
    dialogVisible.value = false;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
};

const toggleActive = async (account: Account, nextValue: boolean) => {
  const prev = account.isActive;
  account.isActive = nextValue;
  rowUpdating.value = account.id;
  try {
    const updated = await updateAccount(account.id, { is_active: nextValue });
    accounts.value = accounts.value.map((item) => (item.id === updated.id ? updated : item));
    ElMessage.success(nextValue ? '已启用' : '已停用');
  } catch (error) {
    account.isActive = prev;
    ElMessage.error((error as Error).message);
  } finally {
    rowUpdating.value = null;
  }
};

const confirmDelete = (account: Account) => {
  ElMessageBox.confirm(`确认删除账户「${account.name}」？`, '提示', { type: 'warning' })
    .then(async () => {
      deleting.value = account.id;
      try {
        await deleteAccount(account.id);
        accounts.value = accounts.value.filter((item) => item.id !== account.id);
        ElMessage.success('已删除');
      } catch (error) {
        ElMessage.error((error as Error).message);
      } finally {
        deleting.value = null;
      }
    })
    .catch(() => undefined);
};

const formatDate = (value?: string) => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
};

onMounted(loadAccounts);
</script>
