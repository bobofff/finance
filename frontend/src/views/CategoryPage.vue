<template>
  <div class="section-header">
    <div>
      <h1>分类管理</h1>
      <div class="light-text">基于后端 /api/categories 提供的 CRUD 界面</div>
    </div>
    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="openCreate">新建分类</el-button>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadCategories">刷新</el-button>
    </div>
  </div>

  <div class="card">
    <el-table :data="tableRows" stripe border v-loading="loading" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column label="名称" min-width="200">
        <template #default="{ row }">
          <div :style="{ paddingLeft: `${row.depth * 18}px` }">
            <span>{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="类型" min-width="140">
        <template #default="{ row }">
          <el-tag :type="getCategoryKindTagType(row.kind)" effect="light">
            {{ formatCategoryKind(row.kind) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="父级" min-width="180">
        <template #default="{ row }">
          <span>{{ formatParentName(row.parentId) }}</span>
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

  <el-dialog v-model="dialogVisible" :title="dialogMode === 'edit' ? '编辑分类' : '新建分类'" width="540px" destroy-on-close>
    <CategoryForm
      v-model="formModel"
      :mode="dialogMode"
      :loading="saving"
      :categories="categories"
      :current-id="selectedId"
      @submit="handleSubmit"
    />
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Delete, Edit, Plus, RefreshRight } from '@element-plus/icons-vue';
import CategoryForm from '@/components/CategoryForm.vue';
import { createCategory, deleteCategory, fetchCategories, toApiPayload, updateCategory } from '@/api/category';
import {
  CATEGORY_KINDS,
  type Category,
  type CategoryFormInput,
  formatCategoryKind,
  getCategoryKindTagType
} from '@/types/category';

type CategoryRow = Category & { depth: number };

const categories = ref<Category[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<'create' | 'edit'>('create');
const formModel = ref<CategoryFormInput>(buildDefaultForm());
const selectedId = ref<number | null>(null);
const deleting = ref<number | null>(null);

function buildDefaultForm(): CategoryFormInput {
  return {
    name: '',
    kind: CATEGORY_KINDS[0].value,
    parentId: null
  };
}

const categoryMap = computed(() => {
  const map = new Map<number, Category>();
  categories.value.forEach((cat) => map.set(cat.id, cat));
  return map;
});

const formatParentName = (parentId: number | null) => {
  if (!parentId) return '-';
  return categoryMap.value.get(parentId)?.name ?? `#${parentId}`;
};

const tableRows = computed<CategoryRow[]>(() => {
  const list = categories.value;
  const idSet = new Set(list.map((cat) => cat.id));
  const byParent = new Map<number | null, Category[]>();

  for (const cat of list) {
    const parentKey = cat.parentId && idSet.has(cat.parentId) ? cat.parentId : null;
    const group = byParent.get(parentKey) ?? [];
    group.push(cat);
    byParent.set(parentKey, group);
  }

  for (const group of byParent.values()) {
    group.sort((a, b) => a.name.localeCompare(b.name));
  }

  const result: CategoryRow[] = [];
  const visited = new Set<number>();

  const walk = (parentId: number | null, depth: number) => {
    const children = byParent.get(parentId) ?? [];
    for (const child of children) {
      if (visited.has(child.id)) continue;
      visited.add(child.id);
      result.push({ ...child, depth });
      walk(child.id, depth + 1);
    }
  };

  walk(null, 0);

  for (const cat of list) {
    if (!visited.has(cat.id)) {
      result.push({ ...cat, depth: 0 });
    }
  }

  return result;
});

const loadCategories = async () => {
  loading.value = true;
  try {
    categories.value = await fetchCategories();
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

const openEdit = (category: Category) => {
  dialogMode.value = 'edit';
  selectedId.value = category.id;
  formModel.value = {
    name: category.name,
    kind: category.kind,
    parentId: category.parentId ?? null
  };
  dialogVisible.value = true;
};

const handleSubmit = async (payload: CategoryFormInput) => {
  saving.value = true;
  try {
    if (dialogMode.value === 'create') {
      await createCategory(toApiPayload(payload));
      ElMessage.success('创建成功');
    } else if (selectedId.value !== null) {
      await updateCategory(selectedId.value, toApiPayload(payload));
      ElMessage.success('更新成功');
    }
    await loadCategories();
    dialogVisible.value = false;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
};

const confirmDelete = (category: Category) => {
  ElMessageBox.confirm(`确认删除分类「${category.name}」？`, '提示', { type: 'warning' })
    .then(async () => {
      deleting.value = category.id;
      try {
        await deleteCategory(category.id);
        categories.value = categories.value.filter((item) => item.id !== category.id);
        ElMessage.success('已删除');
      } catch (error) {
        ElMessage.error((error as Error).message);
      } finally {
        deleting.value = null;
      }
    })
    .catch(() => undefined);
};

onMounted(loadCategories);
</script>
