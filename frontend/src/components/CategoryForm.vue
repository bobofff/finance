<template>
  <el-form ref="formRef" :model="formState" :rules="rules" label-width="110px" status-icon>
    <el-form-item label="名称" prop="name">
      <el-input v-model="formState.name" placeholder="输入分类名称" @input="emitChange" />
    </el-form-item>

    <el-form-item label="类型" prop="kind">
      <el-select v-model="formState.kind" placeholder="选择分类类型" @change="emitChange">
        <el-option v-for="item in CATEGORY_KINDS" :key="item.value" :label="item.label" :value="item.value" />
      </el-select>
    </el-form-item>

    <el-form-item label="父级" prop="parentId">
      <el-select
        v-model="formState.parentId"
        placeholder="无父级"
        clearable
        filterable
        @change="emitChange"
      >
        <el-option :value="null" label="无父级" />
        <el-option v-for="option in parentOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
    </el-form-item>

    <el-form-item>
      <el-button type="primary" :loading="loading" @click="handleSubmit">
        {{ mode === 'edit' ? '保存修改' : '创建分类' }}
      </el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import { CATEGORY_KINDS, type Category, type CategoryFormInput } from '@/types/category';

type ParentOption = {
  value: number;
  label: string;
};

const props = defineProps<{
  modelValue: CategoryFormInput;
  categories: Category[];
  currentId?: number | null;
  loading?: boolean;
  mode?: 'create' | 'edit';
}>();

const emit = defineEmits<{
  (e: 'submit', payload: CategoryFormInput): void;
  (e: 'update:modelValue', value: CategoryFormInput): void;
}>();

const formRef = ref<FormInstance>();
const formState = reactive<CategoryFormInput>({ ...props.modelValue });
const mode = computed(() => props.mode ?? 'create');
const loading = computed(() => props.loading ?? false);
const currentId = computed(() => props.currentId ?? null);

watch(
  () => props.modelValue,
  (val) => {
    Object.assign(formState, val);
  },
  { deep: true }
);

const parentOptions = computed(() => buildParentOptions(props.categories, formState.kind, currentId.value));

const rules: FormRules<CategoryFormInput> = {
  name: [
    { required: true, message: '请输入名称', trigger: 'blur' },
    { min: 1, max: 64, message: '名称需在 1-64 字符之间', trigger: 'blur' }
  ],
  kind: [{ required: true, message: '请选择类型', trigger: 'change' }]
};

const emitChange = () => {
  emit('update:modelValue', { ...formState, parentId: formState.parentId ?? null });
};

watch(parentOptions, (options) => {
  if (formState.parentId === null) return;
  const exists = options.some((option) => option.value === formState.parentId);
  if (!exists) {
    formState.parentId = null;
    emitChange();
  }
});

const handleSubmit = async () => {
  if (!formRef.value) return;
  await formRef.value.validate((valid) => {
    if (valid) {
      emit('submit', { ...formState, parentId: formState.parentId ?? null });
    }
  });
};

function buildParentOptions(categories: Category[], kind: string, excludedId: number | null): ParentOption[] {
  const filtered = categories.filter((cat) => cat.kind === kind && cat.id !== excludedId);
  if (!filtered.length) return [];

  const idSet = new Set(filtered.map((cat) => cat.id));
  const byParent = new Map<number | null, Category[]>();

  for (const cat of filtered) {
    const parentId = cat.parentId && idSet.has(cat.parentId) ? cat.parentId : null;
    const group = byParent.get(parentId) ?? [];
    group.push(cat);
    byParent.set(parentId, group);
  }

  for (const group of byParent.values()) {
    group.sort((a, b) => a.name.localeCompare(b.name));
  }

  const result: ParentOption[] = [];
  const visited = new Set<number>();

  const walk = (parentId: number | null, depth: number) => {
    const children = byParent.get(parentId) ?? [];
    for (const child of children) {
      if (visited.has(child.id)) continue;
      visited.add(child.id);
      result.push({ value: child.id, label: `${'—'.repeat(depth)}${child.name}` });
      walk(child.id, depth + 1);
    }
  };

  walk(null, 0);

  for (const cat of filtered) {
    if (!visited.has(cat.id)) {
      result.push({ value: cat.id, label: cat.name });
    }
  }

  return result;
}
</script>
