export interface BalanceSheetAccount {
  id: number;
  name: string;
  type: string;
  currency: string;
  is_active: boolean;
  balance: number;
}

export interface BalanceSheetGroup {
  key: string;
  label: string;
  total: number;
  accounts: BalanceSheetAccount[];
}

export interface BalanceSheetResponse {
  ledger_id: number;
  as_of: string;
  totals: {
    assets: number;
    liabilities: number;
    net_worth: number;
  };
  groups: BalanceSheetGroup[];
}
