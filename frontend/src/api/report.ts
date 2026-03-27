import client from './client';
import type { AnnualAssetProgressResponse, BalanceSheetResponse, IncomeExpenseSummaryResponse } from '@/types/report';

export type BalanceSheetParams = {
  ledger_id?: number;
  as_of?: string;
};

export async function fetchBalanceSheet(params: BalanceSheetParams = {}): Promise<BalanceSheetResponse> {
  const { data } = await client.get<BalanceSheetResponse>('/reports/balance-sheet', { params });
  return data;
}

export type AnnualAssetProgressParams = {
  ledger_id?: number;
  year?: number;
  as_of?: string;
};

export async function fetchAnnualAssetProgress(
  params: AnnualAssetProgressParams = {}
): Promise<AnnualAssetProgressResponse> {
  const { data } = await client.get<AnnualAssetProgressResponse>('/reports/annual-asset-progress', { params });
  return data;
}

export type IncomeExpenseSummaryParams = {
  ledger_id?: number;
  date_from?: string;
  date_to?: string;
};

export async function fetchIncomeExpenseSummary(
  params: IncomeExpenseSummaryParams = {}
): Promise<IncomeExpenseSummaryResponse> {
  const { data } = await client.get<IncomeExpenseSummaryResponse>('/reports/income-expense-summary', { params });
  return data;
}
