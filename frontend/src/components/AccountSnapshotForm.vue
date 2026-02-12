<template>
  <el-form ref="formRef" :model="formState" :rules="rules" label-width="110px" status-icon>
    <el-form-item label="账户" prop="accountId">
      <el-select v-model="formState.accountId" placeholder="选择账户" :disabled="mode === 'edit'" @change="emitChange">
        <el-option v-for="account in accounts" :key="account.id" :label="account.name" :value="account.id" />
      </el-select>
    </el-form-item>

    <el-form-item label="日期" prop="asOf">
      <el-date-picker v-model="formState.asOf" type="date" value-format="YYYY-MM-DD" @change="emitChange" />
    </el-form-item>

    <el-form-item label="期初余额" prop="amount">
      <el-input-number v-model="formState.amount" :step="0.01" :precision="2" controls-position="right" @change="emitChange" />
    </el-form-item>

    <el-form-item label="备注">
      <el-input v-model="formState.note" placeholder="可选：说明" @input="emitChange" />
    </el-form-item>

    <el-form-item>
      <el-button type="primary" :loading="loading" @click="handleSubmit">
        {{ mode === 'edit' ? '保存修改' : '创建快照' }}
      </el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import type { Account } from '@/types/account';
import type { AccountSnapshotFormInput } from '@/types/snapshot';

const props = defineProps<{
  modelValue: AccountSnapshotFormInput;
  accounts: Account[];
  loading?: boolean;
  mode?: 'create' | 'edit';
}>();

const emit = defineEmits<{
  (e: 'submit', payload: AccountSnapshotFormInput): void;
  (e: 'update:modelValue', value: AccountSnapshotFormInput): void;
}>();

const formRef = ref<FormInstance>();
const formState = reactive<AccountSnapshotFormInput>({ ...props.modelValue });
const mode = computed(() => props.mode ?? 'create');
const loading = computed(() => props.loading ?? false);
const accounts = computed(() => props.accounts ?? []);

watch(
  () => props.modelValue,
  (val) => {
    Object.assign(formState, val);
  },
  { deep: true }
);

const rules: FormRules<AccountSnapshotFormInput> = {
  accountId: [{ required: true, message: '请选择账户', trigger: 'change' }],
  asOf: [{ required: true, message: '请选择日期', trigger: 'change' }],
  amount: [{ required: true, message: '请输入余额', trigger: 'change' }]
};

const emitChange = () => emit('update:modelValue', { ...formState });

const handleSubmit = async () => {
  if (!formRef.value) return;
  await formRef.value.validate((valid) => {
    if (valid) {
      emit('submit', { ...formState });
    }
  });
};
</script>
