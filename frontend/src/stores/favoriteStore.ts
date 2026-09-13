import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { addFavorite, cancelFavorite, getFavoriteState, listFavorites } from '../api/favorite'
import { useAuthStore } from './authStore'
import type { Product } from '../types'

// 单个商品对当前登录用户的收藏状态：
//  - on/off：已确认的“已收藏 / 未收藏”（数量也已确认）
//  - loading：状态批量加载中（此时 UI 显示加载态，而不是未收藏/0）
//  - unknown：从未加载或加载失败（UI 显示未知态，可点击重试，而不是未收藏/0）
export type FavoriteStatusKind = 'on' | 'off' | 'loading' | 'unknown'

// 收藏状态集中管理：商品列表、详情、收藏页共享同一份“是否已收藏 + 收藏数”，
// 任一处收藏/取消都会即时同步到所有使用该 store 的组件。
export const useFavoriteStore = defineStore('favorite', () => {
  const authStore = useAuthStore()

  // productId -> 是否已收藏（当前登录用户）；键不存在表示“状态未知”
  const favorited = reactive<Record<number, boolean>>({})
  // productId -> 收藏总数；键不存在表示“数量未知”
  const counts = reactive<Record<number, number>>({})
  // productId -> 批量状态是否加载中
  const loadingMap = reactive<Record<number, boolean>>({})
  // 进行中的收藏/取消请求，防止按钮连点
  const pending = reactive<Record<number, boolean>>({})
  // 我的收藏总数（用于导航角标）
  const mineCount = ref(0)

  // 待拉取状态的商品 id（非响应式，仅用于同一 tick 内的批量合并与去重）。
  // 刻意不把“加载中/失败”写进响应式的 favorited：favorited[id] === undefined
  // 表示“状态未知”，这样请求失败后下一次 hydrate 才会重新拉取。
  const queued = new Set<number>()
  let flushScheduled = false
  function scheduleFlush() {
    if (flushScheduled) {
      return
    }
    flushScheduled = true
    Promise.resolve().then(flush)
  }
  async function flush() {
    flushScheduled = false
    if (queued.size === 0) {
      return
    }
    const ids = Array.from(queued)
    // 先取出本批 id 并清空队列；失败的 id 因未写入 favorited 仍为未知，
    // 下次 hydrate 会再次入队重试。
    queued.clear()
    try {
      const res = await getFavoriteState(ids)
      Object.entries(res.data.favorited).forEach(([id, v]) => {
        favorited[Number(id)] = v
      })
      Object.entries(res.data.counts).forEach(([id, n]) => {
        counts[Number(id)] = n
      })
      // 收藏数为 0 的商品后端不返回，这里补 0，标记为“已确认的已知状态”。
      ids.forEach((id) => {
        if (counts[id] === undefined) {
          counts[id] = 0
        }
      })
    } catch {
      // 不写任何占位值：favorited[id] 保持 undefined（未知），
      // 重新进入商品列表/收藏页或再次加载时会重新获取状态与数量；
      // 错误提示已由 request 拦截器统一弹出。
    } finally {
      // 无论成功失败都结束“加载中”：成功后为已知态，失败后为未知态。
      ids.forEach((id) => {
        loadingMap[id] = false
      })
    }
  }

  const isLogin = computed(() => !!authStore.token)

  function isFavorited(id: number): boolean {
    return !!favorited[id]
  }

  function isLoading(id: number): boolean {
    return !!loadingMap[id]
  }

  function countOf(id: number): number {
    return counts[id] ?? 0
  }

  // 返回按钮应呈现的收藏状态。未登录用户按“未收藏”展示（点击会引导登录）；
  // 登录用户在加载中显示 loading，状态未知（加载失败/尚未加载）显示 unknown，
  // 只有拿到服务端结果后才显示已收藏 on 或未收藏 off。
  function favState(id: number): FavoriteStatusKind {
    if (!isLogin.value) {
      return 'off'
    }
    if (pending[id]) {
      // 乐观操作进行中：沿用操作前的已知方向，避免按钮闪烁
      return favorited[id] ? 'on' : 'off'
    }
    if (loadingMap[id]) {
      return 'loading'
    }
    if (favorited[id] === undefined) {
      return 'unknown'
    }
    return favorited[id] ? 'on' : 'off'
  }

  // 数量展示：加载中返回 loading、未知（加载失败）返回 unknown，
  // 已确认才返回具体数字；避免把“还没拿到/失败”误显示为 0。
  function countDisplay(id: number): { kind: 'value'; value: number } | { kind: 'loading' | 'unknown' } {
    if (!isLogin.value) {
      return { kind: 'value', value: counts[id] ?? 0 }
    }
    if (pending[id]) {
      return { kind: 'value', value: counts[id] ?? 0 }
    }
    if (loadingMap[id]) {
      return { kind: 'loading' }
    }
    if (counts[id] === undefined) {
      return { kind: 'unknown' }
    }
    return { kind: 'value', value: counts[id] }
  }

  // 供详情等文本场景直接渲染数量：已确认显示数字，加载中显示“—”，未知显示“?”。
  function countText(id: number): string {
    const c = countDisplay(id)
    if (c.kind === 'value') return String(c.value)
    return c.kind === 'loading' ? '—' : '?'
  }

  // 拉取当前用户对一批商品的收藏标记与收藏总数（页面加载、刷新后调用）。
  // 同一 tick 的多次调用合并为一次 /favorites/state 请求；仅对“状态未知”
  // （favorited[id] === undefined，含此前加载失败）且未在加载的商品发起。
  function hydrate(ids: number[]) {
    if (!isLogin.value || ids.length === 0) {
      return
    }
    let added = false
    ids.forEach((id) => {
      // favorited[id] 为 undefined 才需要拉取；加载中的不重复入队，
      // 加载失败不写占位值，因此会再次拉取（重试）。
      if (favorited[id] === undefined && !loadingMap[id] && !queued.has(id)) {
        queued.add(id)
        loadingMap[id] = true // 立即进入加载态，供 UI 显示加载中
        added = true
      }
    })
    if (added) {
      scheduleFlush()
    }
  }

  // 收藏或取消收藏，返回是否成功（用于调用方刷新本地列表）。
  async function toggle(product: Product): Promise<boolean> {
    if (!isLogin.value) {
      ElMessage.warning('请先登录后再收藏')
      return false
    }
    if (pending[product.id]) {
      return false
    }
    // 不能收藏自己发布的商品（按钮通常已隐藏，此处再兜底）
    if (authStore.user && product.seller_id === authStore.user.id) {
      ElMessage.warning('不能收藏自己发布的商品')
      return false
    }
    // 状态未知（加载中/加载失败）时不盲目发起收藏：改为重新加载一次状态，
    // 拿到结果后再由用户决定，避免在未知态下产生错误的收藏/取消。
    if (favorited[product.id] === undefined) {
      if (!loadingMap[product.id]) {
        hydrate([product.id])
        ElMessage.info('收藏状态获取中，请稍后')
      }
      return false
    }

    const wasFav = !!favorited[product.id]
    // 记录操作前是否已“已知”状态/数量：若原本未知（如加载失败后直接点击），
    // 请求失败时要回到未知（删除键）而不是写成未收藏，以便下次 hydrate 重试。
    const wasStateKnown = favorited[product.id] !== undefined
    const prevCount = counts[product.id]
    pending[product.id] = true
    // 乐观更新按钮与数量，失败回滚；我的收藏角标在确认成功后再更新
    favorited[product.id] = !wasFav
    counts[product.id] = Math.max(0, countOf(product.id) + (wasFav ? -1 : 1))

    try {
      if (wasFav) {
        const res = await cancelFavorite(product.id)
        favorited[product.id] = res.data.favorited
        counts[product.id] = res.data.count
        // duplicated=true 表示本就没有收藏记录，角标无需变化
        if (!res.data.duplicated) {
          mineCount.value = Math.max(0, mineCount.value - 1)
        }
        ElMessage.success('已取消收藏')
      } else {
        const res = await addFavorite(product.id)
        favorited[product.id] = res.data.favorited
        counts[product.id] = res.data.count
        // 重复收藏不会新增记录，只有真正新建时角标才 +1
        if (!res.data.duplicated) {
          mineCount.value += 1
        }
        ElMessage.success(res.data.duplicated ? '该商品已在收藏夹中' : '收藏成功')
      }
      return true
    } catch {
      // 回滚乐观更新（错误提示已由 request 拦截器统一弹出）。
      // 原本状态未知时恢复为未知（删除键），下次 hydrate 会重新拉取，
      // 而不是把该商品永久当成“未收藏”。
      if (wasStateKnown) {
        favorited[product.id] = wasFav
      } else {
        delete favorited[product.id]
      }
      if (prevCount === undefined) {
        delete counts[product.id]
      } else {
        counts[product.id] = prevCount
      }
      return false
    } finally {
      pending[product.id] = false
    }
  }

  async function refreshMineCount() {
    if (!isLogin.value) {
      mineCount.value = 0
      return
    }
    try {
      const res = await listFavorites({ page: 1, page_size: 1 })
      mineCount.value = res.data.total
    } catch {
      // 忽略，角标非关键数据
    }
  }

  // 切换账号后清空与用户绑定的缓存（含加载标记）
  function reset() {
    Object.keys(favorited).forEach((k) => delete favorited[Number(k)])
    Object.keys(counts).forEach((k) => delete counts[Number(k)])
    Object.keys(loadingMap).forEach((k) => delete loadingMap[Number(k)])
    Object.keys(pending).forEach((k) => delete pending[Number(k)])
    queued.clear()
    mineCount.value = 0
  }

  return {
    favorited,
    counts,
    loadingMap,
    mineCount,
    isFavorited,
    isLoading,
    favState,
    countDisplay,
    countText,
    countOf,
    hydrate,
    toggle,
    refreshMineCount,
    reset,
  }
})
