export type TransactionKind = 'income' | 'expense';

export interface TransactionRow {
  transaction_id: number;
  line_id: number;
  occurred_on: string;
  account_id: number;
  account_name: string;
  category_id: number;
  category_name: string;
  category_kind: TransactionKind;
  amount: number;
  description: string;
  note: string;
  created_at: string;
}

export interface TransactionListResponse {
  data: TransactionRow[];
  total: number;
}

export interface TransactionFormInput {
  kind: TransactionKind;
  occurredOn: string;
  accountId: number;
  categoryId: number;
  amount: number;
  description: string;
  note: string;
}
