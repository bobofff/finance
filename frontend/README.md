# Finance Frontend (Vue 3 + TS + Element Plus)

基于后端 `/api/accounts` 的账户管理界面，支持查看、创建、编辑、启用/停用以及删除账户，并提供左侧导航布局。

## 技术栈
- Vite + Vue 3 + TypeScript
- Element Plus 组件库
- Axios 请求封装

## 开发
```bash
cd frontend
npm install
npm run dev
```
默认开发端口 `5173`，`vite.config.ts` 已将 `/api` 代理到 `http://localhost:8888`，请确保后端在该端口运行。

## 构建
```bash
npm run build
npm run preview
```

## 主要结构
- `src/views/AccountPage.vue`：账户列表、操作入口
- `src/components/AccountForm.vue`：创建/编辑表单
- `src/api/`：Axios 客户端与账户 API 封装
- `src/types/`：类型与枚举
