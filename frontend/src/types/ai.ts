import type { TransactionKind } from './transaction';

export interface AiParseTransactionRequest {
  ledger_id?: number;
  text: string;
}

export interface AiTransactionDraft {
  kind: TransactionKind;
  occurred_on: string;
  account_id?: number | null;
  account_name: string;
  category_id?: number | null;
  category_name: string;
  amount: number;
  description: string;
  note: string;
  missing_fields: string[];
  low_confidence_fields: string[];
}

export interface AiParseTransactionResponse {
  ledger_id: number;
  source_text: string;
  draft: AiTransactionDraft;
}
