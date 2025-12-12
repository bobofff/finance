import axios from 'axios';

const client = axios.create({
  baseURL: '/api',
  timeout: 8000
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error?.response?.data?.error || error?.message || 'Request failed';
    return Promise.reject(new Error(message));
  }
);

export default client;
