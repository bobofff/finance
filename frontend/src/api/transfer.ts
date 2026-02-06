import client from './client';

export type CreateTransferPayload = {
  ledger_id?: number;
  occurred_on: string;
  from_account_id: number;
  to_account_id: number;
  amount: number;
  description?: string;
  note?: string;
};

export type CreateTransferResponse = {
  transaction_id: number;
};

export async function createTransfer(payload: CreateTransferPayload): Promise<CreateTransferResponse> {
  const { data } = await client.post<CreateTransferResponse>('/transfers', payload);
  return data;
}
