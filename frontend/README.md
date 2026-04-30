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
默认开发端口 `5173`，`vite.config.ts` 已将 `/api` 代理到 `http://127.0.0.1:8888`，请确保后端在该端口运行。

如需改到其他后端地址，可在前端目录设置环境变量：
```bash
VITE_API_PROXY_TARGET=http://127.0.0.1:8888
```
使用 `127.0.0.1` 而不是 `localhost` 可以避免部分环境里 `localhost -> ::1` 时出现 `ECONNREFUSED ::1:8888`。

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
