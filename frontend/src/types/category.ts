export const CATEGORY_KINDS = [
  { value: 'income', label: '收入', tagType: 'success' },
  { value: 'expense', label: '支出', tagType: 'danger' },
  { value: 'transfer', label: '转账', tagType: 'warning' },
  { value: 'investment', label: '投资', tagType: 'info' }
] as const;

export type CategoryKind = (typeof CATEGORY_KINDS)[number]['value'];

export interface ApiCategory {
  ID?: number;
  id?: number;
  Name?: string;
  name?: string;
  Kind?: string;
  kind?: string;
  ParentID?: number | null;
  parent_id?: number | null;
  Parent?: ApiCategory;
  parent?: ApiCategory;
}

export interface Category {
  id: number;
  name: string;
  kind: CategoryKind;
  parentId: number | null;
}

export interface CategoryFormInput {
  name: string;
  kind: CategoryKind;
  parentId: number | null;
}

type CategoryKindMeta = (typeof CATEGORY_KINDS)[number];

const normalizeParentId = (value: number | null | undefined): number | null => {
  if (value === null || value === undefined) return null;
  if (value <= 0) return null;
  return value;
};

const extractParentId = (data: ApiCategory): number | null => {
  const rawParentId = data.ParentID ?? data.parent_id ?? data.Parent?.ID ?? data.parent?.id ?? null;
  return normalizeParentId(rawParentId ?? null);
};

export function mapCategory(data: ApiCategory): Category {
  return {
    id: data.ID ?? data.id ?? 0,
    name: data.Name ?? data.name ?? '',
    kind: (data.Kind ?? data.kind ?? '') as CategoryKind,
    parentId: extractParentId(data)
  };
}

export function getCategoryKindMeta(value: string): CategoryKindMeta | undefined {
  return CATEGORY_KINDS.find((item) => item.value === value);
}

export function formatCategoryKind(value: string): string {
  return getCategoryKindMeta(value)?.label ?? value;
}

export function getCategoryKindTagType(value: string): CategoryKindMeta['tagType'] {
  return getCategoryKindMeta(value)?.tagType ?? 'info';
}
