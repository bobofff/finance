export interface ApiInvestmentLot {
  lot_id?: number;
  lotId?: number;
  ledger_id?: number;
  ledgerId?: number;
  security_id?: number;
  securityId?: number;
  security_ticker?: string;
  securityTicker?: string;
  security_name?: string;
  securityName?: string;
  quantity?: number;
  price?: number;
  trade_price?: number;
  tradePrice?: number;
  fee?: number;
  tax?: number;
  transaction_line_id?: number;
  transactionLineId?: number;
  transaction_id?: number;
  transactionId?: number;
  occurred_on?: string;
  occurredOn?: string;
  allocated_quantity?: number;
  allocatedQuantity?: number;
  remaining_quantity?: number;
  remainingQuantity?: number;
  status?: string;
}

export interface InvestmentLot {
  lotId: number;
  ledgerId: number;
  securityId: number;
  securityTicker: string;
  securityName: string;
  quantity: number;
  price: number;
  tradePrice: number;
  fee: number;
  tax: number;
  transactionLineId: number;
  transactionId: number;
  occurredOn: string;
  allocatedQuantity: number;
  remainingQuantity: number;
  status: 'open' | 'closed';
}

export interface SaleAllocationInput {
  buy_lot_id: number;
  quantity: number;
}

export interface CreateSalePayload {
  ledger_id?: number;
  occurred_on: string;
  security_id: number;
  cash_account_id: number;
  investment_account_id: number;
  price: number;
  fee?: number;
  fee_category_id?: number | null;
  tax?: number;
  tax_category_id?: number | null;
  description?: string;
  note?: string;
  allocations: SaleAllocationInput[];
}

export interface CreateSaleResponse {
  transaction_id: number;
  sale_id: number;
  quantity: number;
  price: number;
  gross_amount: number;
  cost_amount: number;
  fee: number;
  tax: number;
}

export interface CreateBuyPayload {
  ledger_id?: number;
  occurred_on: string;
  security_id?: number;
  security_ticker?: string;
  security_name?: string;
  cash_account_id: number;
  investment_account_id: number;
  quantity: number;
  price: number;
  fee?: number;
  fee_category_id?: number | null;
  tax?: number;
  tax_category_id?: number | null;
  description?: string;
  note?: string;
}

export interface CreateBuyResponse {
  transaction_id: number;
  lot_id: number;
  quantity: number;
  price: number;
  cost_price: number;
  gross_amount: number;
  cost_amount: number;
  fee: number;
  tax: number;
}

export function mapInvestmentLot(data: ApiInvestmentLot): InvestmentLot {
  const status = (data.status ?? 'open') as 'open' | 'closed';
  return {
    lotId: data.lot_id ?? data.lotId ?? 0,
    ledgerId: data.ledger_id ?? data.ledgerId ?? 1,
    securityId: data.security_id ?? data.securityId ?? 0,
    securityTicker: data.security_ticker ?? data.securityTicker ?? '-',
    securityName: data.security_name ?? data.securityName ?? '-',
    quantity: data.quantity ?? 0,
    price: data.price ?? 0,
    tradePrice: data.trade_price ?? data.tradePrice ?? 0,
    fee: data.fee ?? 0,
    tax: data.tax ?? 0,
    transactionLineId: data.transaction_line_id ?? data.transactionLineId ?? 0,
    transactionId: data.transaction_id ?? data.transactionId ?? 0,
    occurredOn: data.occurred_on ?? data.occurredOn ?? '',
    allocatedQuantity: data.allocated_quantity ?? data.allocatedQuantity ?? 0,
    remainingQuantity: data.remaining_quantity ?? data.remainingQuantity ?? 0,
    status: status === 'closed' ? 'closed' : 'open'
  };
}
