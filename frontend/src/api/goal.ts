import client from './client';
import type { AnnualAssetGoal } from '@/types/goal';

export type AnnualAssetGoalPayload = {
  ledger_id?: number;
  target_net_worth: number;
  baseline_date?: string;
  note?: string;
};

export async function fetchAnnualAssetGoal(year: number, ledgerID?: number): Promise<AnnualAssetGoal> {
  const { data } = await client.get<AnnualAssetGoal>(`/goals/annual-asset/${year}`, {
    params: ledgerID ? { ledger_id: ledgerID } : undefined
  });
  return data;
}

export async function upsertAnnualAssetGoal(
  year: number,
  payload: AnnualAssetGoalPayload
): Promise<AnnualAssetGoal> {
  const { data } = await client.put<AnnualAssetGoal>(`/goals/annual-asset/${year}`, payload);
  return data;
}
