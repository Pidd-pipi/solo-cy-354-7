<template>
  <div class="page">
    <div class="fav-header">
      <h2>我的收藏</h2>
      <el-radio-group v-model="status" @change="load">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button label="on_sale">在售</el-radio-button>
        <el-radio-button label="sold">已售出</el-radio-button>
        <el-radio-button label="removed">已下架</el-radio-button>
      </el-radio-group>
    </div>

    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="p in products" :key="p.id" :span="6" class="col">
        <ProductCard :product="p" @detail="showDetail" @buy="buy" @chat="chat" @favorite="onCardFavorite" />
      </el-col>
    </el-row>

    <el-empty v-if="!loading && products.length === 0" :description="emptyText" />

    <el-pagination
      v-if="total > pageSize"
      class="pager"
      layout="prev, pager, next"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="onPageChange"
    />

    <el-dialog v-model="detailVisible" :title="current?.title" width="520px">
      <el-descriptions :column="2" border v-if="current">
        <el-descriptions-item label="分类">{{ categoryLabel(current.category) }}</el-descriptions-item>
        <el-descriptions-item label="成色">{{ current.condition }}</el-descriptions-item>
        <el-descriptions-item label="校区">{{ current.campus }}</el-descriptions-item>
        <el-descriptions-item label="交易地点">{{ current.trade_location }}</el-descriptions-item>
        <el-descriptions-item label="价格">¥{{ current.price.toFixed(2) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ productStatusLabel(current.status) }}</el-descriptions-item>
        <el-descriptions-item label="收藏数">{{ favStore.countText(current.id) }}</el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">{{ current.description }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <FavoriteButton :product="current!" @changed="onFavoriteChanged" />
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import ProductCard from '../components/common/ProductCard.vue'
import FavoriteButton from '../components/common/FavoriteButton.vue'
import { PRODUCT_STATUSES, categoryLabel, productStatusLabel } from '../constants/product'
import { listFavorites } from '../api/favorite'
import { createTradeOrder } from '../api/tradeOrder'
import { createConversation } from '../api/conversation'
import { useFavoriteStore } from '../stores/favoriteStore'
import type { Product } from '../types'

const route = useRoute()
const router = useRouter()
const favStore = useFavoriteStore()

const products = ref<Product[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 12
const loading = ref(false)
const detailVisible = ref(false)
const current = ref<Product | null>(null)

// 支持 /favorites?status=sold 直接进入对应筛选
const status = ref<string>((route.query.status as string) || '')

const emptyText = computed(() => {
  const label = PRODUCT_STATUSES.find((s) => s.value === status.value)?.label
  return label ? `暂无${label}的收藏商品` : '还没有收藏任何商品，去商品广场看看吧'
})

async function load() {
  loading.value = true
  try {
    const res = await listFavorites({ page: page.value, page_size: pageSize, status: status.value || undefined })
    products.value = res.data.items
    total.value = res.data.total
    // 同步每个商品的收藏标记与数量（此前加载失败的会在此重新拉取）
    favStore.hydrate(res.data.items.map((p) => p.id))
    // 列表恢复成功后同步“我的收藏”角标数量
    favStore.refreshMineCount()
  } catch {
    products.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function showDetail(p: Product) {
  current.value = p
  detailVisible.value = true
}

// 在详情弹窗里取消收藏后，从当前列表移除该商品（保持与筛选一致）
function onFavoriteChanged(favorited: boolean) {
  if (!favorited && current.value) {
    removeFromList(current.value.id)
  }
}

// 在卡片上直接取消收藏：从当前筛选列表移除
function onCardFavorite(p: Product, favorited: boolean) {
  if (!favorited) {
    removeFromList(p.id)
  }
}

// 仅维护当前页列表；按钮状态、收藏数、角标已由 favoriteStore.toggle 同步
function removeFromList(id: number) {
  products.value = products.value.filter((p) => p.id !== id)
  total.value = Math.max(0, total.value - 1)
}

async function buy(p: Product) {
  await createTradeOrder(p.id)
  ElMessage.success('已下单，等待卖家确认')
}

async function chat(p: Product) {
  await createConversation(p.id)
  ElMessage.success('已发起私信')
  router.push('/messages')
}

onMounted(load)
</script>

<style scoped>
.fav-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.col {
  margin-bottom: 16px;
}
.pager {
  margin-top: 8px;
  justify-content: center;
}
</style>
