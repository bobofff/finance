import client from './client';
import { ACCOUNT_TYPES, Account, AccountFormInput, ApiAccount, mapAccount } from '@/types/account';

export type CreateAccountPayload = {
  name: string;
  type: (typeof ACCOUNT_TYPES)[number]['value'];
  currency?: string;
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

export function toApiPayload(input: AccountFormInput): CreateAccountPayload {
  return {
    name: input.name,
    type: input.type as (typeof ACCOUNT_TYPES)[number]['value'],
    currency: input.currency,
    is_active: input.isActive
  };
}
