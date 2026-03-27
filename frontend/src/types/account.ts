export interface ApiAccount {
  ID?: number;
  id?: number;
  Name?: string;
  name?: string;
  Type?: string;
  type?: string;
  Currency?: string;
  currency?: string;
  CashKind?: string;
  cash_kind?: string;
  IsActive?: boolean;
  is_active?: boolean;
  CreatedAt?: string;
  created_at?: string;
}

export type CashKind = 'bank' | 'broker';

export interface Account {
  id: number;
  name: string;
  type: string;
  currency: string;
  cashKind?: CashKind;
  isActive: boolean;
  createdAt?: string;
}

export interface AccountFormInput {
  name: string;
  type: string;
  currency: string;
  cashKind: CashKind;
  isActive: boolean;
}

export const ACCOUNT_TYPES: Array<{ value: string; label: string }> = [
  { value: 'cash', label: 'Cash' },
  { value: 'liability', label: 'Liability' },
  { value: 'debt', label: '债权 (Receivable)' },
  { value: 'investment', label: 'Investment' },
  { value: 'other_asset', label: 'Other Asset' }
];

export const CASH_KINDS: Array<{ value: CashKind; label: string }> = [
  { value: 'bank', label: '银行现金' },
  { value: 'broker', label: '证券资金' }
];

export function mapAccount(data: ApiAccount): Account {
  return {
    id: data.ID ?? data.id ?? 0,
    name: data.Name ?? data.name ?? '',
    type: data.Type ?? data.type ?? '',
    currency: data.Currency ?? data.currency ?? '',
    cashKind: (data.CashKind ?? data.cash_kind) as CashKind | undefined,
    isActive: data.IsActive ?? data.is_active ?? false,
    createdAt: data.CreatedAt ?? data.created_at
  };
}

export function formatAccountType(value: string): string {
  return ACCOUNT_TYPES.find((item) => item.value === value)?.label ?? value;
}

export function normalizeCashKind(value?: string): CashKind {
  return value === 'broker' ? 'broker' : 'bank';
}

export function formatCashKind(value?: string): string {
  const normalized = normalizeCashKind(value);
  return CASH_KINDS.find((item) => item.value === normalized)?.label ?? normalized;
}
