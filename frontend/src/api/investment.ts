import client from './client';
import {
  ApiInvestmentLot,
  CreateBuyPayload,
  CreateBuyResponse,
  CreateSalePayload,
  CreateSaleResponse,
  InvestmentLot,
  InvestmentStrategy,
  ApiInvestmentStrategy,
  mapInvestmentLot,
  mapInvestmentStrategy
} from '@/types/investment';

export type LotStatus = 'open' | 'closed';

export type FetchInvestmentLotsParams = {
  ledger_id?: number;
  security_id?: number;
  cash_account_id?: number;
  status?: LotStatus;
  tag?: string;
  keyword?: string;
  buy_date_from?: string;
  buy_date_to?: string;
  page?: number;
  page_size?: number;
};

export type FetchInvestmentLotsSummary = {
  total_quantity: number;
  total_cost: number;
  total_market: number;
  profit: number;
  profit_pct: number;
  has_market: boolean;
  partial_market: boolean;
};

export type FetchInvestmentLotsResponse = {
  data: ApiInvestmentLot[];
  total: number;
  summary: FetchInvestmentLotsSummary;
};

export async function fetchInvestmentLots(
  params: FetchInvestmentLotsParams = {}
): Promise<{ data: InvestmentLot[]; total: number; summary: FetchInvestmentLotsSummary }> {
  const { data } = await client.get<FetchInvestmentLotsResponse>('/investments/lots', { params });
  return {
    data: data.data.map(mapInvestmentLot),
    total: data.total,
    summary: data.summary
  };
}

export async function createInvestmentSale(payload: CreateSalePayload): Promise<CreateSaleResponse> {
  const { data } = await client.post<CreateSaleResponse>('/investments/sales', payload);
  return data;
}

export async function createInvestmentBuy(payload: CreateBuyPayload): Promise<CreateBuyResponse> {
  const { data } = await client.post<CreateBuyResponse>('/investments/buys', payload);
  return data;
}

export async function updateInvestmentBuy(lotId: number, payload: CreateBuyPayload): Promise<CreateBuyResponse> {
  const { data } = await client.patch<CreateBuyResponse>(`/investments/buys/${lotId}`, payload);
  return data;
}

export async function deleteInvestmentBuy(lotId: number): Promise<void> {
  await client.delete(`/investments/buys/${lotId}`);
}

export type RefreshInvestmentPricesResponse = {
  requested: number;
  updated: number;
  skipped: number;
  failed?: string[];
};

export async function refreshInvestmentPrices(): Promise<RefreshInvestmentPricesResponse> {
  const { data } = await client.post<RefreshInvestmentPricesResponse>('/investments/prices/refresh');
  return data;
}

export async function refreshInvestmentPricesWithHistory(
  historyDays = 60
): Promise<RefreshInvestmentPricesResponse> {
  const { data } = await client.post<RefreshInvestmentPricesResponse>('/investments/prices/refresh', null, {
    params: { history_days: historyDays }
  });
  return data;
}

export async function fetchInvestmentStrategies(
  params: { ledger_id?: number } = {}
): Promise<InvestmentStrategy[]> {
  const { data } = await client.get<ApiInvestmentStrategy[]>('/investments/strategies', { params });
  return data.map(mapInvestmentStrategy);
}
