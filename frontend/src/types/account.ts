export interface ApiAccount {
  ID?: number;
  id?: number;
  Name?: string;
  name?: string;
  Type?: string;
  type?: string;
  Currency?: string;
  currency?: string;
  IsActive?: boolean;
  is_active?: boolean;
  CreatedAt?: string;
  created_at?: string;
}

export interface Account {
  id: number;
  name: string;
  type: string;
  currency: string;
  isActive: boolean;
  createdAt?: string;
}

export interface AccountFormInput {
  name: string;
  type: string;
  currency: string;
  isActive: boolean;
}

export const ACCOUNT_TYPES: Array<{ value: string; label: string }> = [
  { value: 'cash', label: 'Cash' },
  { value: 'liability', label: 'Liability' },
  { value: 'debt', label: 'Receivable' },
  { value: 'investment', label: 'Investment' },
  { value: 'other_asset', label: 'Other Asset' }
];

export function mapAccount(data: ApiAccount): Account {
  return {
    id: data.ID ?? data.id ?? 0,
    name: data.Name ?? data.name ?? '',
    type: data.Type ?? data.type ?? '',
    currency: data.Currency ?? data.currency ?? '',
    isActive: data.IsActive ?? data.is_active ?? false,
    createdAt: data.CreatedAt ?? data.created_at
  };
}

export function formatAccountType(value: string): string {
  return ACCOUNT_TYPES.find((item) => item.value === value)?.label ?? value;
}
