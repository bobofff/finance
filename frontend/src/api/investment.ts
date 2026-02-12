import client from './client';
import {
  ApiInvestmentLot,
  CreateBuyPayload,
  CreateBuyResponse,
  CreateSalePayload,
  CreateSaleResponse,
  InvestmentLot,
  mapInvestmentLot
} from '@/types/investment';

export type LotStatus = 'open' | 'closed';

export type FetchInvestmentLotsParams = {
  ledger_id?: number;
  security_id?: number;
  status?: LotStatus;
};

export async function fetchInvestmentLots(params: FetchInvestmentLotsParams = {}): Promise<InvestmentLot[]> {
  const { data } = await client.get<ApiInvestmentLot[]>('/investments/lots', { params });
  return data.map(mapInvestmentLot);
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
