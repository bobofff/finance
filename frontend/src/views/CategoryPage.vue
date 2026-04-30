<template>
  <div class="section-header">
    <div>
      <h1>分类管理</h1>
      <div class="light-text">基于后端 /api/categories 提供的树形 CRUD 界面，支持按层级展开和折叠</div>
    </div>
    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="openCreate">新建分类</el-button>
      <el-button @click="expandAll">展开全部</el-button>
      <el-button @click="collapseAll">收起全部</el-button>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadCategories">刷新</el-button>
    </div>
  </div>

  <div class="card">
    <el-table
      ref="categoryTableRef"
      :data="treeRows"
      stripe
      border
      row-key="id"
      :default-expand-all="false"
      :tree-props="{ children: 'children' }"
      class="category-tree-table"
      v-loading="loading"
      style="width: 100%"
    >
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column
        label="名称"
        min-width="260"
        class-name="category-name-column"
        label-class-name="category-name-column"
      >
        <template #default="{ row }">
          <div class="category-name-cell">
            <span class="category-name-text">{{ row.name }}</span>
            <span v-if="row.children?.length" class="category-child-count">{{ row.children.length }} 个子分类</span>
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
import { computed, nextTick, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox, type TableInstance } from 'element-plus';
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

type CategoryTreeRow = Category & { children?: CategoryTreeRow[] };

const categories = ref<Category[]>([]);
const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<'create' | 'edit'>('create');
const formModel = ref<CategoryFormInput>(buildDefaultForm());
const selectedId = ref<number | null>(null);
const deleting = ref<number | null>(null);
const categoryTableRef = ref<TableInstance>();

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

const compareCategories = (left: Category, right: Category) => {
  const nameOrder = left.name.localeCompare(right.name, 'zh-Hans-CN');
  if (nameOrder !== 0) return nameOrder;
  return left.id - right.id;
};

const formatParentName = (parentId: number | null) => {
  if (!parentId) return '顶级分类';
  return categoryMap.value.get(parentId)?.name ?? `#${parentId}`;
};

const treeRows = computed<CategoryTreeRow[]>(() => buildCategoryTree(categories.value));

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
        await loadCategories();
        ElMessage.success('已删除');
      } catch (error) {
        ElMessage.error((error as Error).message);
      } finally {
        deleting.value = null;
      }
    })
    .catch(() => undefined);
};

const toggleTreeExpansion = async (expanded: boolean) => {
  await nextTick();
  const table = categoryTableRef.value;
  if (!table) return;

  const walk = (rows: CategoryTreeRow[]) => {
    rows.forEach((row) => {
      if (!row.children?.length) return;
      table.toggleRowExpansion(row, expanded);
      walk(row.children);
    });
  };

  walk(treeRows.value);
};

const expandAll = () => toggleTreeExpansion(true);
const collapseAll = () => toggleTreeExpansion(false);

function buildCategoryTree(list: Category[]): CategoryTreeRow[] {
  const idSet = new Set(list.map((cat) => cat.id));
  const byParent = new Map<number | null, Category[]>();

  for (const cat of list) {
    const parentKey = cat.parentId !== null && idSet.has(cat.parentId) ? cat.parentId : null;
    const group = byParent.get(parentKey) ?? [];
    group.push(cat);
    byParent.set(parentKey, group);
  }

  byParent.forEach((group) => {
    group.sort(compareCategories);
  });

  const visited = new Set<number>();

  const walk = (parentId: number | null): CategoryTreeRow[] => {
    const children = byParent.get(parentId) ?? [];
    const result: CategoryTreeRow[] = [];

    for (const child of children) {
      if (visited.has(child.id)) continue;
      visited.add(child.id);
      const nestedChildren = walk(child.id);
      result.push(nestedChildren.length ? { ...child, children: nestedChildren } : { ...child });
    }

    return result;
  };

  const roots = walk(null);
  const leftovers = list.filter((cat) => !visited.has(cat.id)).sort(compareCategories);

  for (const cat of leftovers) {
    if (visited.has(cat.id)) continue;
    visited.add(cat.id);
    const nestedChildren = walk(cat.id);
    roots.push(nestedChildren.length ? { ...cat, children: nestedChildren } : { ...cat });
  }

  return roots;
}

onMounted(loadCategories);
</script>

<style scoped>
.category-name-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.category-name-text {
  font-weight: 600;
  color: var(--app-text-primary);
}

.category-child-count {
  color: var(--app-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}

:deep(.category-tree-table .category-name-column .cell) {
  text-align: left !important;
}
</style>
