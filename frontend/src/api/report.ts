import client from './client';
import type { BalanceSheetResponse } from '@/types/report';

export type BalanceSheetParams = {
  ledger_id?: number;
  as_of?: string;
};

export async function fetchBalanceSheet(params: BalanceSheetParams = {}): Promise<BalanceSheetResponse> {
  const { data } = await client.get<BalanceSheetResponse>('/reports/balance-sheet', { params });
  return data;
}
