import client from './client';
import {
  ApiAccountSnapshot,
  AccountSnapshot,
  AccountSnapshotFormInput,
  mapAccountSnapshot
} from '@/types/snapshot';

export type CreateSnapshotPayload = {
  ledger_id?: number;
  account_id: number;
  as_of: string;
  amount: number;
  note?: string;
};

export type UpdateSnapshotPayload = Partial<Pick<CreateSnapshotPayload, 'as_of' | 'amount' | 'note'>>;

export type FetchSnapshotParams = {
  ledger_id?: number;
  account_id?: number;
};

export async function fetchAccountSnapshots(params: FetchSnapshotParams = {}): Promise<AccountSnapshot[]> {
  const { data } = await client.get<ApiAccountSnapshot[]>('/account-snapshots', { params });
  return data.map(mapAccountSnapshot);
}

export async function createAccountSnapshot(payload: CreateSnapshotPayload): Promise<AccountSnapshot> {
  const { data } = await client.post<ApiAccountSnapshot>('/account-snapshots', payload);
  return mapAccountSnapshot(data);
}

export async function updateAccountSnapshot(id: number, payload: UpdateSnapshotPayload): Promise<AccountSnapshot> {
  const { data } = await client.patch<ApiAccountSnapshot>(`/account-snapshots/${id}`, payload);
  return mapAccountSnapshot(data);
}

export async function deleteAccountSnapshot(id: number): Promise<void> {
  await client.delete(`/account-snapshots/${id}`);
}

export function toApiPayload(input: AccountSnapshotFormInput): CreateSnapshotPayload {
  return {
    account_id: input.accountId,
    as_of: input.asOf,
    amount: input.amount,
    note: input.note
  };
}
