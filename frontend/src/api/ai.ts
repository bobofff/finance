import client from './client';
import type { AiParseTransactionRequest, AiParseTransactionResponse } from '@/types/ai';

const AI_PARSE_TRANSACTION_TIMEOUT_MS = 75000;

export async function parseTransactionDraft(payload: AiParseTransactionRequest): Promise<AiParseTransactionResponse> {
  const { data } = await client.post<AiParseTransactionResponse>('/ai/parse-transaction', payload, {
    timeout: AI_PARSE_TRANSACTION_TIMEOUT_MS
  });
  return data;
}
