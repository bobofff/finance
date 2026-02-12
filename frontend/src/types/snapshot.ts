export interface ApiAccountSnapshot {
  ID?: number;
  id?: number;
  LedgerID?: number;
  ledger_id?: number;
  AccountID?: number;
  account_id?: number;
  AsOf?: string;
  as_of?: string;
  Amount?: number;
  amount?: number;
  Note?: string;
  note?: string;
  CreatedAt?: string;
  created_at?: string;
}

export interface AccountSnapshot {
  id: number;
  ledgerId: number;
  accountId: number;
  asOf: string;
  amount: number;
  note?: string;
  createdAt?: string;
}

export interface AccountSnapshotFormInput {
  accountId: number;
  asOf: string;
  amount: number;
  note?: string;
}

export function mapAccountSnapshot(data: ApiAccountSnapshot): AccountSnapshot {
  return {
    id: data.ID ?? data.id ?? 0,
    ledgerId: data.LedgerID ?? data.ledger_id ?? 1,
    accountId: data.AccountID ?? data.account_id ?? 0,
    asOf: data.AsOf ?? data.as_of ?? '',
    amount: data.Amount ?? data.amount ?? 0,
    note: data.Note ?? data.note ?? '',
    createdAt: data.CreatedAt ?? data.created_at
  };
}
