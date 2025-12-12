<template>
  <el-container class="app-shell">
    <el-aside width="240px" class="sidebar">
      <div class="brand">
        <div class="brand-logo">¥</div>
        <div>
          <div class="brand-title">Finance</div>
          <div class="brand-subtitle">简洁的财务面板</div>
        </div>
      </div>
      <el-menu :default-active="activeMenu" class="sidebar-menu" :router="false" @select="onSelect">
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
      <el-header class="topbar">
        <div class="topbar-left">
          <div class="topbar-title">财务总览</div>
          <div class="topbar-subtitle">Accounts · Transactions · Insights</div>
        </div>
        <div class="topbar-actions">
          <el-button text size="small" :icon="Bell">通知</el-button>
          <el-button text size="small" :icon="User">用户</el-button>
        </div>
      </el-header>
      <el-main class="main-content">
        <div class="page-wrapper">
          <AccountPage />
        </div>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, type Component } from 'vue';
import { ElMessage } from 'element-plus';
import {
  Bell,
  Collection,
  DataAnalysis,
  Setting,
  Tickets,
  User,
  WalletFilled
} from '@element-plus/icons-vue';
import AccountPage from './views/AccountPage.vue';

type MenuItem = {
  key: string;
  label: string;
  icon: Component;
  disabled?: boolean;
  badge?: string;
};

const menuItems: MenuItem[] = [
  { key: 'dashboard', label: '概览', icon: DataAnalysis, disabled: true, badge: 'Soon' },
  { key: 'accounts', label: '账户', icon: WalletFilled },
  { key: 'categories', label: '分类', icon: Collection, disabled: true, badge: '待开发' },
  { key: 'transactions', label: '交易', icon: Tickets, disabled: true, badge: '待开发' },
  { key: 'settings', label: '设置', icon: Setting, disabled: true }
];

const activeMenu = ref('accounts');

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
};
</script>
