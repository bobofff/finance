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

export interface AnnualAssetGoalSummary {
  target_net_worth: number;
  baseline_date: string;
}

export interface AnnualAssetProgressMonth {
  month: string;
  income: number;
  expense: number;
  net_income: number;
  required_net_income: number;
  completion: number;
  is_future: boolean;
}

export interface AnnualAssetProgressResponse {
  ledger_id: number;
  year: number;
  as_of: string;
  goal: AnnualAssetGoalSummary;
  current_net_worth: number;
  baseline_net_worth: number;
  remaining_gap: number;
  remaining_months: number;
  required_monthly_net_income: number;
  progress: number;
  months: AnnualAssetProgressMonth[];
}

export interface IncomeExpenseSummaryDay {
  date: string;
  income: number;
  expense: number;
  net_income: number;
}

export interface IncomeExpenseSummarySubCategory {
  id: number;
  name: string;
  amount: number;
  ratio: number;
}

export interface IncomeExpenseSummaryCategory {
  id: number;
  name: string;
  amount: number;
  ratio: number;
  children: IncomeExpenseSummarySubCategory[];
}

export interface IncomeExpenseSummaryResponse {
  ledger_id: number;
  date_from: string;
  date_to: string;
  totals: {
    income: number;
    expense: number;
    net_income: number;
    transaction_count: number;
  };
  days: IncomeExpenseSummaryDay[];
  breakdown: {
    income: IncomeExpenseSummaryCategory[];
    expense: IncomeExpenseSummaryCategory[];
  };
}
