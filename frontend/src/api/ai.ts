import client from './client';
import type { AiParseTransactionRequest, AiParseTransactionResponse } from '@/types/ai';

export async function parseTransactionDraft(payload: AiParseTransactionRequest): Promise<AiParseTransactionResponse> {
  const { data } = await client.post<AiParseTransactionResponse>('/ai/parse-transaction', payload);
  return data;
}
