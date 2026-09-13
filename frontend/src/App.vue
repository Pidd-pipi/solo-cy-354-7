<template>
  <el-container class="app-shell">
    <el-header class="app-header">
      <div class="brand" @click="$router.push('/products')">
        <span class="brand-icon">🎒</span>
        <span class="brand-name">校园二手交易平台</span>
      </div>
      <el-menu mode="horizontal" :router="true" :default-active="$route.path" class="nav-menu" :ellipsis="false">
        <el-menu-item index="/products">商品广场</el-menu-item>
        <el-menu-item index="/publish">发布商品</el-menu-item>
        <el-menu-item index="/messages">私信</el-menu-item>
        <el-menu-item index="/orders">我的交易</el-menu-item>
        <el-menu-item v-if="authStore.token" index="/favorites">
          我的收藏<i v-if="favStore.mineCount > 0" class="fav-badge">{{ favStore.mineCount }}</i>
        </el-menu-item>
        <el-menu-item index="/book-exchange">书籍交换</el-menu-item>
        <el-menu-item index="/graduation">毕业季专场</el-menu-item>
        <el-menu-item index="/profile">个人中心</el-menu-item>
      </el-menu>
      <div class="header-right">
        <template v-if="authStore.token">
          <span class="user-chip">👤 {{ authStore.user?.nickname }}</span>
          <el-button link type="danger" @click="logout">退出</el-button>
        </template>
        <template v-else>
          <el-button link type="primary" @click="$router.push('/login')">登录</el-button>
          <el-button link @click="$router.push('/register')">注册</el-button>
        </template>
      </div>
    </el-header>
    <el-main class="app-main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from './stores/authStore'
import { useFavoriteStore } from './stores/favoriteStore'

const authStore = useAuthStore()
const favStore = useFavoriteStore()
const router = useRouter()

function logout() {
  authStore.logout()
  favStore.reset()
  ElMessage.success('已退出登录')
  router.push('/products')
}

// 登录状态变化（登录/退出/重新登录）时同步收藏缓存与角标数量
watch(
  () => authStore.token,
  (token, oldToken) => {
    if (!token) {
      favStore.reset()
    } else if (token !== oldToken) {
      favStore.reset()
      favStore.refreshMineCount()
    }
  },
)

onMounted(() => {
  authStore.restore()
  if (authStore.token) {
    favStore.refreshMineCount()
  }
})
</script>

<style>
body {
  margin: 0;
  font-family: 'Helvetica Neue', Arial, 'PingFang SC', sans-serif;
  background: #f5f7fa;
}
.app-shell {
  min-height: 100vh;
}
.app-header {
  display: flex;
  align-items: center;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  gap: 24px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}
.brand-icon {
  font-size: 28px;
}
.brand-name {
  font-size: 18px;
  font-weight: 700;
  color: #303133;
  white-space: nowrap;
}
.nav-menu {
  flex: 1;
  border-bottom: none;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}
.user-chip {
  color: #606266;
  font-size: 14px;
}
.fav-badge {
  display: inline-block;
  margin-left: 6px;
  min-width: 18px;
  padding: 0 5px;
  height: 18px;
  line-height: 18px;
  text-align: center;
  font-style: normal;
  font-size: 12px;
  color: #fff;
  background: #f56c6c;
  border-radius: 9px;
}
.app-main {
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 24px;
}
</style>
