import client from './client';
import type { TransactionListResponse, TransactionFormInput, TransactionRow } from '@/types/transaction';

export type FetchTransactionsParams = {
  ledger_id?: number;
  account_id?: number;
  category_id?: number;
  kind?: 'income' | 'expense';
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
};

export type CreateTransactionPayload = {
  ledger_id?: number;
  occurred_on: string;
  account_id: number;
  category_id: number;
  amount: number;
  description?: string;
  note?: string;
};

export type UpdateTransactionPayload = Partial<CreateTransactionPayload>;

export async function fetchTransactions(params: FetchTransactionsParams = {}): Promise<TransactionListResponse> {
  const { data } = await client.get<TransactionListResponse>('/transactions', { params });
  return data;
}

export async function createTransaction(payload: CreateTransactionPayload): Promise<TransactionRow> {
  const { data } = await client.post<TransactionRow>('/transactions', payload);
  return data;
}

export async function updateTransaction(id: number, payload: UpdateTransactionPayload): Promise<void> {
  await client.patch(`/transactions/${id}`, payload);
}

export async function deleteTransaction(id: number): Promise<void> {
  await client.delete(`/transactions/${id}`);
}

export function toApiPayload(input: TransactionFormInput): CreateTransactionPayload {
  const amount = input.kind === 'expense' ? -Math.abs(input.amount) : Math.abs(input.amount);
  return {
    occurred_on: input.occurredOn,
    account_id: input.accountId,
    category_id: input.categoryId,
    amount,
    description: input.description?.trim() || undefined,
    note: input.note?.trim() || undefined
  };
}
