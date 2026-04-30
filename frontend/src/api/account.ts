import client from './client';
import { ACCOUNT_TYPES, Account, AccountFormInput, ApiAccount, CashKind, mapAccount } from '@/types/account';

export type CreateAccountPayload = {
  ledger_id?: number;
  name: string;
  type: (typeof ACCOUNT_TYPES)[number]['value'];
  currency?: string;
  cash_kind?: CashKind;
  is_active?: boolean;
};

export type UpdateAccountPayload = Partial<CreateAccountPayload>;

export async function fetchAccounts(): Promise<Account[]> {
  const { data } = await client.get<ApiAccount[]>('/accounts');
  return data.map(mapAccount);
}

export async function createAccount(payload: CreateAccountPayload): Promise<Account> {
  const { data } = await client.post<ApiAccount>('/accounts', payload);
  return mapAccount(data);
}

export async function updateAccount(id: number, payload: UpdateAccountPayload): Promise<Account> {
  const { data } = await client.patch<ApiAccount>(`/accounts/${id}`, payload);
  return mapAccount(data);
}

export async function deleteAccount(id: number): Promise<void> {
  await client.delete(`/accounts/${id}`);
}

export function toApiPayload(input: AccountFormInput, ledgerId = 1): CreateAccountPayload {
  return {
    ledger_id: ledgerId,
    name: input.name,
    type: input.type as (typeof ACCOUNT_TYPES)[number]['value'],
    currency: input.currency,
    cash_kind: input.type === 'cash' ? input.cashKind : undefined,
    is_active: input.isActive
  };
}
