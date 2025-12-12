<template>
  <el-form ref="formRef" :model="formState" :rules="rules" label-width="110px" status-icon>
    <el-form-item label="名称" prop="name">
      <el-input v-model="formState.name" placeholder="输入账户名称" @input="emitChange" />
    </el-form-item>

    <el-form-item label="类型" prop="type">
      <el-select v-model="formState.type" placeholder="选择账户类型" @change="emitChange">
        <el-option v-for="item in ACCOUNT_TYPES" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
    </el-form-item>

    <el-form-item label="币种" prop="currency">
      <el-input v-model="formState.currency" placeholder="默认 CNY" maxlength="10" @input="emitChange" />
    </el-form-item>

    <el-form-item label="是否启用" prop="isActive">
      <el-switch
        v-model="formState.isActive"
        active-text="启用"
        inactive-text="停用"
        @change="emitChange"
      />
    </el-form-item>

    <el-form-item>
      <el-button type="primary" :loading="loading" @click="handleSubmit">
        {{ mode === 'edit' ? '保存修改' : '创建账户' }}
      </el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import { ACCOUNT_TYPES, type AccountFormInput } from '@/types/account';

const props = defineProps<{
  modelValue: AccountFormInput;
  loading?: boolean;
  mode?: 'create' | 'edit';
}>();

const emit = defineEmits<{
  (e: 'submit', payload: AccountFormInput): void;
  (e: 'update:modelValue', value: AccountFormInput): void;
}>();

const formRef = ref<FormInstance>();
const formState = reactive<AccountFormInput>({ ...props.modelValue });
const mode = computed(() => props.mode ?? 'create');
const loading = computed(() => props.loading ?? false);

watch(
  () => props.modelValue,
  (val) => {
    Object.assign(formState, val);
  },
  { deep: true }
);

const rules: FormRules<AccountFormInput> = {
  name: [
    { required: true, message: '请输入名称', trigger: 'blur' },
    { min: 1, max: 64, message: '名称需在 1-64 字符之间', trigger: 'blur' }
  ],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  currency: [{ max: 10, message: '币种长度不超过 10', trigger: 'blur' }]
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
