<template>
  <el-container class="app-shell">
    <div v-if="!isAuthed" class="theme-floating">
      <el-tooltip :content="isDarkTheme ? '切换到浅色模式' : '切换到暗黑模式'" placement="bottom">
        <el-button circle :icon="isDarkTheme ? Sunny : Moon" class="theme-floating-button" @click="toggleTheme" />
      </el-tooltip>
    </div>

    <el-aside
      v-if="isAuthed && !isMobile"
      :width="isSidebarCollapsed ? '80px' : '240px'"
      class="sidebar"
      :class="{ 'sidebar-collapsed': isSidebarCollapsed }"
    >
      <div class="brand">
        <div class="brand-logo">¥</div>
        <div>
          <div class="brand-title">Finance</div>
          <div class="brand-subtitle">简洁的财务面板</div>
        </div>
      </div>
      <el-menu
        :default-active="activeMenu"
        class="sidebar-menu"
        :router="false"
        :collapse="isSidebarCollapsed"
        :collapse-transition="false"
        @select="onSelect"
      >
        <el-menu-item v-for="item in menuItems" :key="item.key" :index="item.key" :disabled="item.disabled">
          <component :is="item.icon" class="menu-icon" />
          <span>{{ item.label }}</span>
          <el-tag v-if="item.badge" size="small" type="info" class="menu-badge">{{ item.badge }}</el-tag>
        </el-menu-item>
      </el-menu>
      <div class="sidebar-footer">
        <div class="footer-title">快捷提示</div>
        <div class="footer-text">先新建账户，再通过交易录入期初/日常流水。</div>
      </div>
    </el-aside>

    <el-container class="content-shell">
      <el-header v-if="isAuthed" class="topbar">
        <div class="topbar-left">
          <el-button v-if="isMobile" text size="small" @click="drawerVisible = true">菜单</el-button>
          <el-button
            v-if="!isMobile"
            text
            size="small"
            :icon="isSidebarCollapsed ? Expand : Fold"
            @click="toggleSidebar"
          />
          <div class="topbar-title">{{ activeMenuLabel }}</div>
          <div class="topbar-subtitle">{{ activeMenuSubtitle }}</div>
        </div>
        <div class="topbar-actions">
          <el-tooltip :content="isDarkTheme ? '切换到浅色模式' : '切换到暗黑模式'" placement="bottom">
            <el-button text size="small" :icon="isDarkTheme ? Sunny : Moon" @click="toggleTheme">
              {{ isDarkTheme ? '浅色' : '暗黑' }}
            </el-button>
          </el-tooltip>
          <el-button text size="small" :icon="Bell">通知</el-button>
          <el-button text size="small" :icon="User" @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main-content">
        <div
          class="page-wrapper"
          :class="{
            'page-wrapper-full':
              !isAuthed ||
              activeMenu === 'investments' ||
              activeMenu === 'transactions' ||
              activeMenu === 'snapshots' ||
              activeMenu === 'world-cup'
          }"
        >
          <component :is="activeView" :key="activeMenu" @success="handleLoginSuccess" />
        </div>
      </el-main>
    </el-container>
  </el-container>

  <el-drawer v-model="drawerVisible" title="菜单" size="240px" direction="ltr" :with-header="true" class="sidebar-drawer">
    <el-menu :default-active="activeMenu" class="drawer-menu" :router="false" @select="onSelectMobile">
      <el-menu-item v-for="item in menuItems" :key="item.key" :index="item.key" :disabled="item.disabled">
        <component :is="item.icon" class="menu-icon" />
        <span>{{ item.label }}</span>
        <el-tag v-if="item.badge" size="small" type="info" class="menu-badge">{{ item.badge }}</el-tag>
      </el-menu-item>
    </el-menu>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type Component } from 'vue';
import { ElMessage } from 'element-plus';
import {
  Bell,
  Collection,
  DataAnalysis,
  Expand,
  Fold,
  Moon,
  Setting,
  Soccer,
  Sunny,
  Tickets,
  TrendCharts,
  User,
  WalletFilled
} from '@element-plus/icons-vue';
import AccountPage from './views/AccountPage.vue';
import CategoryPage from './views/CategoryPage.vue';
import AccountSnapshotPage from './views/AccountSnapshotPage.vue';
import InvestmentPage from './views/InvestmentPage.vue';
import BalanceSheetPage from './views/BalanceSheetPage.vue';
import TransactionPage from './views/TransactionPage.vue';
import AnnualGoalPage from './views/AnnualGoalPage.vue';
import WorldCupPage from './views/WorldCupPage.vue';
import LoginPage from './views/LoginPage.vue';
import { authStorage } from '@/api/client';
import { logout } from '@/api/auth';

type MenuItem = {
  key: string;
  label: string;
  icon: Component;
  disabled?: boolean;
  badge?: string;
};

const menuItems: MenuItem[] = [
  { key: 'balance-sheet', label: '资产负债', icon: DataAnalysis },
  { key: 'annual-goal', label: '年度目标', icon: DataAnalysis },
  { key: 'accounts', label: '账户', icon: WalletFilled },
  { key: 'snapshots', label: '期初余额', icon: Tickets },
  { key: 'categories', label: '分类', icon: Collection },
  { key: 'investments', label: '投资', icon: TrendCharts },
  { key: 'transactions', label: '交易', icon: Tickets },
  { key: 'world-cup', label: '世界杯', icon: Soccer },
  { key: 'settings', label: '设置', icon: Setting, disabled: true }
];

const MENU_STORAGE_KEY = 'finance.activeMenu';
const THEME_STORAGE_KEY = 'finance.theme';
const activeMenu = ref(localStorage.getItem(MENU_STORAGE_KEY) || 'balance-sheet');
const isAuthed = ref(!!authStorage.getToken());
const isMobile = ref(window.innerWidth <= 900);
const drawerVisible = ref(false);
const SIDEBAR_COLLAPSE_KEY = 'finance.sidebarCollapsed';
const isSidebarCollapsed = ref(localStorage.getItem(SIDEBAR_COLLAPSE_KEY) === '1');
const themeMode = ref<'light' | 'dark'>(resolveInitialTheme());
const isDarkTheme = computed(() => themeMode.value === 'dark');
const activeMenuItem = computed(() => menuItems.find((item) => item.key === activeMenu.value));
const activeMenuLabel = computed(() => activeMenuItem.value?.label || '财务总览');
const activeMenuSubtitle = computed(() =>
  activeMenu.value === 'world-cup' ? 'Groups · Standings · Fixtures' : 'Accounts · Transactions · Insights'
);
const activeView = computed(() => {
  if (!isAuthed.value) {
    return LoginPage;
  }
  switch (activeMenu.value) {
    case 'balance-sheet':
      return BalanceSheetPage;
    case 'annual-goal':
      return AnnualGoalPage;
    case 'categories':
      return CategoryPage;
    case 'snapshots':
      return AccountSnapshotPage;
    case 'investments':
      return InvestmentPage;
    case 'transactions':
      return TransactionPage;
    case 'world-cup':
      return WorldCupPage;
    default:
      return AccountPage;
  }
});

const onSelect = (key: string) => {
  const previous = activeMenu.value;
  const item = menuItems.find((entry) => entry.key === key);
  if (!item) return;
  if (item.disabled) {
    ElMessage.info('此模块稍后开放，当前可使用账户管理');
    activeMenu.value = previous;
    return;
  }
  activeMenu.value = item.key;
  localStorage.setItem(MENU_STORAGE_KEY, item.key);
};

const onSelectMobile = (key: string) => {
  onSelect(key);
  drawerVisible.value = false;
};

const handleLoginSuccess = () => {
  isAuthed.value = true;
};

const handleLogout = () => {
  logout();
  isAuthed.value = false;
};

const toggleSidebar = () => {
  isSidebarCollapsed.value = !isSidebarCollapsed.value;
  localStorage.setItem(SIDEBAR_COLLAPSE_KEY, isSidebarCollapsed.value ? '1' : '0');
};

const toggleTheme = () => {
  themeMode.value = isDarkTheme.value ? 'light' : 'dark';
};

const handleResize = () => {
  isMobile.value = window.innerWidth <= 900;
  if (!isMobile.value) {
    drawerVisible.value = false;
  }
};

function resolveInitialTheme(): 'light' | 'dark' {
  const savedTheme = localStorage.getItem(THEME_STORAGE_KEY);
  if (savedTheme === 'light' || savedTheme === 'dark') {
    return savedTheme;
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(mode: 'light' | 'dark') {
  const root = document.documentElement;
  root.classList.toggle('dark', mode === 'dark');
  root.dataset.theme = mode;
  root.style.colorScheme = mode;
  window.dispatchEvent(new CustomEvent('app:theme-change', { detail: { mode } }));
}

watch(
  themeMode,
  (mode) => {
    localStorage.setItem(THEME_STORAGE_KEY, mode);
    applyTheme(mode);
  },
  { immediate: true }
);

onMounted(() => {
  window.addEventListener('auth:logout', handleLogout);
  window.addEventListener('resize', handleResize);
});

onBeforeUnmount(() => {
  window.removeEventListener('auth:logout', handleLogout);
  window.removeEventListener('resize', handleResize);
});
</script>
