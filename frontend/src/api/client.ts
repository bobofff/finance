import axios from 'axios';

const client = axios.create({
  baseURL: '/api',
  timeout: 8000
});

const TOKEN_KEY = 'finance.auth.token';
const REMEMBER_KEY = 'finance.auth.remember';

export const authStorage = {
  getToken() {
    const remember = localStorage.getItem(REMEMBER_KEY) === '1';
    if (remember) return localStorage.getItem(TOKEN_KEY);
    return sessionStorage.getItem(TOKEN_KEY);
  },
  setToken(token: string, remember: boolean) {
    localStorage.setItem(REMEMBER_KEY, remember ? '1' : '0');
    if (remember) {
      localStorage.setItem(TOKEN_KEY, token);
      sessionStorage.removeItem(TOKEN_KEY);
    } else {
      sessionStorage.setItem(TOKEN_KEY, token);
      localStorage.removeItem(TOKEN_KEY);
    }
  },
  clear() {
    localStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(TOKEN_KEY);
  }
};

client.interceptors.request.use((config) => {
  const token = authStorage.getToken();
  if (token) {
    config.headers = config.headers ?? {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error?.response?.status === 401) {
      authStorage.clear();
      window.dispatchEvent(new CustomEvent('auth:logout'));
    }
    const message = error?.response?.data?.error || error?.message || 'Request failed';
    return Promise.reject(new Error(message));
  }
);

export default client;
