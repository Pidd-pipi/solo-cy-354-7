<template>
  <el-tooltip :content="tip" placement="top">
    <el-button
      size="small"
      :type="state === 'on' ? 'warning' : 'default'"
      :loading="state === 'loading'"
      @click.stop="onClick"
    >
      <span v-if="state !== 'loading'" :class="starClass">
        <template v-if="state === 'unknown'">?</template>
        <template v-else-if="state === 'on'">★</template>
        <template v-else>☆</template>
      </span>
      <span class="fav-count">{{ countText }}</span>
    </el-button>
  </el-tooltip>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useFavoriteStore } from '../../stores/favoriteStore'
import type { Product } from '../../types'

const props = defineProps<{ product: Product }>()
const emit = defineEmits<{ (e: 'changed', favorited: boolean): void }>()

const favStore = useFavoriteStore()
const loading = ref(false)

onMounted(() => favStore.hydrate([props.product.id]))
watch(() => props.product.id, (id) => favStore.hydrate([id]))

const state = computed(() => favStore.favState(props.product.id))

const countText = computed(() => {
  // 加载中按钮自身转圈，不再显示数字；未知显示“?”；已确认显示真实数量
  if (state.value === 'loading') return ''
  return favStore.countText(props.product.id)
})

const tip = computed(() => {
  switch (state.value) {
    case 'on':
      return '取消收藏'
    case 'off':
      return '收藏'
    case 'loading':
      return '收藏状态加载中…'
    default:
      return '收藏状态加载失败，点击重试'
  }
})

const starClass = computed(() => {
  if (state.value === 'on') return 'fav-star fav-on'
  if (state.value === 'unknown') return 'fav-star fav-unknown'
  return 'fav-star'
})

async function onClick() {
  if (state.value === 'loading') {
    return
  }
  loading.value = true
  try {
    // 未知态（加载失败）时，store 不会误收藏，而是触发重新拉取状态。
    const ok = await favStore.toggle(props.product)
    if (ok) {
      emit('changed', favStore.isFavorited(props.product.id))
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.fav-star {
  font-size: 14px;
  color: #c0c4cc;
}
.fav-star.fav-on {
  color: #e6a23c;
}
/* 状态未知（加载失败）：弱化 + 虚线感，提示与普通“未收藏 ☆”区分 */
.fav-star.fav-unknown {
  color: #909399;
  font-weight: 700;
  opacity: 0.75;
}
.fav-count {
  margin-left: 4px;
  font-size: 12px;
}
</style>
