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
  tag?: string;
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
  current_price?: number;
  currentPrice?: number;
  ma_5?: number | null;
  high_55?: number | null;
  high_20?: number | null;
  low_10?: number | null;
  low_20?: number | null;
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
  tag: string;
  fee: number;
  tax: number;
  transactionLineId: number;
  transactionId: number;
  occurredOn: string;
  allocatedQuantity: number;
  remainingQuantity: number;
  status: 'open' | 'closed';
  currentPrice: number;
  ma5: number | null;
  high55: number | null;
  high20: number | null;
  low10: number | null;
  low20: number | null;
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
  tag?: string;
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

export interface ApiInvestmentStrategy {
  id?: number;
  ledger_id?: number;
  ledgerId?: number;
  name?: string;
  kind?: string;
  params?: unknown;
  is_active?: boolean;
  isActive?: boolean;
  created_at?: string;
  createdAt?: string;
  updated_at?: string;
  updatedAt?: string;
}

export interface InvestmentStrategy {
  id: number;
  ledgerId: number;
  name: string;
  kind: string;
  params: unknown;
  isActive: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export function mapInvestmentStrategy(data: ApiInvestmentStrategy): InvestmentStrategy {
  return {
    id: data.id ?? 0,
    ledgerId: data.ledger_id ?? data.ledgerId ?? 1,
    name: data.name ?? '',
    kind: data.kind ?? '',
    params: data.params ?? {},
    isActive: data.is_active ?? data.isActive ?? true,
    createdAt: data.created_at ?? data.createdAt,
    updatedAt: data.updated_at ?? data.updatedAt
  };
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
    tag: data.tag ?? '',
    fee: data.fee ?? 0,
    tax: data.tax ?? 0,
    transactionLineId: data.transaction_line_id ?? data.transactionLineId ?? 0,
    transactionId: data.transaction_id ?? data.transactionId ?? 0,
    occurredOn: data.occurred_on ?? data.occurredOn ?? '',
    allocatedQuantity: data.allocated_quantity ?? data.allocatedQuantity ?? 0,
    remainingQuantity: data.remaining_quantity ?? data.remainingQuantity ?? 0,
    status: status === 'closed' ? 'closed' : 'open',
    currentPrice: data.current_price ?? data.currentPrice ?? 0,
    ma5: (data.ma_5 ?? null) as number | null,
    high55: (data.high_55 ?? null) as number | null,
    high20: (data.high_20 ?? null) as number | null,
    low10: (data.low_10 ?? null) as number | null,
    low20: (data.low_20 ?? null) as number | null
  };
}
