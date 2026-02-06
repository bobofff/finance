import client from './client';
import { Category, CategoryFormInput, CategoryKind, ApiCategory, mapCategory } from '@/types/category';

export type CreateCategoryPayload = {
  name: string;
  kind: CategoryKind;
  parent_id?: number | null;
  ledger_id?: number;
};

export type UpdateCategoryPayload = Partial<CreateCategoryPayload>;

type CategoryListResponse =
  | ApiCategory[]
  | { data?: ApiCategory[]; list?: ApiCategory[]; categories?: ApiCategory[] }
  | null
  | undefined;

export async function fetchCategories(): Promise<Category[]> {
  const { data } = await client.get<CategoryListResponse>('/categories');
  const list = Array.isArray(data)
    ? data
    : Array.isArray(data?.data)
      ? data?.data
      : Array.isArray(data?.list)
        ? data?.list
        : Array.isArray(data?.categories)
          ? data?.categories
          : [];

  return list.map(mapCategory);
}

export async function createCategory(payload: CreateCategoryPayload): Promise<Category> {
  const { data } = await client.post<ApiCategory>('/categories', payload);
  return mapCategory(data);
}

export async function updateCategory(id: number, payload: UpdateCategoryPayload): Promise<Category> {
  const { data } = await client.patch<ApiCategory>(`/categories/${id}`, payload);
  return mapCategory(data);
}

export async function deleteCategory(id: number): Promise<void> {
  await client.delete(`/categories/${id}`);
}

export function toApiPayload(input: CategoryFormInput): CreateCategoryPayload {
  return {
    name: input.name,
    kind: input.kind,
    parent_id: input.parentId ?? null
  };
}
