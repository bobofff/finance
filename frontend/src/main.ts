import { createApp } from 'vue';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import App from './App.vue';
import './style.css';

const THEME_STORAGE_KEY = 'finance.theme';
const savedTheme = localStorage.getItem(THEME_STORAGE_KEY);
const initialTheme =
  savedTheme === 'dark' || (savedTheme !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches)
    ? 'dark'
    : 'light';

document.documentElement.classList.toggle('dark', initialTheme === 'dark');
document.documentElement.dataset.theme = initialTheme;
document.documentElement.style.colorScheme = initialTheme;

const app = createApp(App);
app.use(ElementPlus);
app.mount('#app');
