<template>
  <div class="login-shell">
    <div class="login-card">
      <div class="login-title">登录 Finance</div>
      <div class="login-subtitle">请输入账号密码</div>

      <el-form>
        <el-form-item label="账号">
          <el-input v-model="username" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="password" type="password" autocomplete="current-password" show-password />
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="remember">记住我</el-checkbox>
        </el-form-item>
      </el-form>

      <el-button type="primary" :loading="loading" class="login-button" @click="submit">登录</el-button>
      <div v-if="error" class="login-error">{{ error }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { login } from '@/api/auth';

const emit = defineEmits<{
  (e: 'success'): void;
}>();

const username = ref('');
const password = ref('');
const remember = ref(true);
const loading = ref(false);
const error = ref('');

const submit = async () => {
  error.value = '';
  if (!username.value.trim() || !password.value) {
    error.value = '请输入账号和密码';
    return;
  }
  loading.value = true;
  try {
    await login({ username: username.value.trim(), password: password.value, remember: remember.value });
    emit('success');
  } catch (err) {
    error.value = (err as Error).message;
  } finally {
    loading.value = false;
  }
};
</script>

<style scoped>
.login-shell {
  min-height: calc(100vh - 72px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.login-card {
  width: min(420px, 92vw);
  background: #fff;
  border-radius: 16px;
  padding: 28px 26px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.1);
}

.login-title {
  font-size: 22px;
  font-weight: 700;
}

.login-subtitle {
  margin-top: 6px;
  color: #64748b;
  margin-bottom: 16px;
}

.login-button {
  width: 100%;
}

.login-error {
  margin-top: 12px;
  color: #dc2626;
  font-size: 12px;
}
</style>
