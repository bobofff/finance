import client from './client';
import type { WorldCupResponse } from '@/types/worldCup';

export type FetchWorldCupParams = {
  refresh?: boolean;
};

export async function fetchWorldCup(params: FetchWorldCupParams = {}): Promise<WorldCupResponse> {
  const { data } = await client.get<WorldCupResponse>('/world-cup', {
    params: {
      refresh: params.refresh ? 1 : undefined
    },
    timeout: 60000
  });
  return data;
}
