import client, { authStorage } from './client';

export type LoginPayload = {
  username: string;
  password: string;
  remember: boolean;
};

export type LoginResponse = {
  token: string;
  expires_at: string;
};

export async function login(payload: LoginPayload): Promise<LoginResponse> {
  const { data } = await client.post<LoginResponse>('/auth/login', payload);
  authStorage.setToken(data.token, payload.remember);
  return data;
}

export function logout() {
  authStorage.clear();
}

export async function fetchMe(): Promise<{ username: string }> {
  const { data } = await client.get<{ username: string }>('/auth/me');
  return data;
}
