import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Product } from '../types'
import { useFavoriteStore } from './favoriteStore'

// 可控假接口与假登录态（vi.hoisted 保证 vi.mock 工厂提升后仍可安全引用）。
const mocks = vi.hoisted(() => ({
  // 批量状态/数量接口
  state: vi.fn(),
  add: vi.fn(),
  cancel: vi.fn(),
  list: vi.fn(),
  // 记录 ElMessage 调用，避免在 node 环境下触碰真实 DOM 组件
  messages: [] as Array<[string, string]>,
  auth: { token: 'tok', user: { id: 100 } as { id: number } },
}))

vi.mock('../api/favorite', () => ({
  addFavorite: (...args: unknown[]) => mocks.add(...args),
  cancelFavorite: (...args: unknown[]) => mocks.cancel(...args),
  getFavoriteState: (...args: unknown[]) => mocks.state(...args),
  listFavorites: (...args: unknown[]) => mocks.list(...args),
}))

vi.mock('element-plus', () => ({
  ElMessage: {
    success: (m: string) => mocks.messages.push(['success', m]),
    warning: (m: string) => mocks.messages.push(['warning', m]),
    error: (m: string) => mocks.messages.push(['error', m]),
    info: (m: string) => mocks.messages.push(['info', m]),
  },
}))

vi.mock('./authStore', () => ({
  useAuthStore: () => mocks.auth,
}))

const flush = (ms = 5) => new Promise((r) => setTimeout(r, ms))

function product(id: number, sellerId = 7): Product {
  return {
    id,
    seller_id: sellerId,
    title: `商品${id}`,
    description: '',
    price: 10,
    category: 'books',
    condition: '全新',
    campus: '东校区',
    trade_location: '东门',
    images: '',
    status: 'on_sale',
    created_at: '',
  }
}

function store() {
  return useFavoriteStore()
}

describe('favoriteStore 状态缓存', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.auth.token = 'tok'
    mocks.auth.user = { id: 100 }
    mocks.messages.length = 0
    mocks.state.mockReset()
    mocks.add.mockReset()
    mocks.cancel.mockReset()
    mocks.list.mockReset()
  })

  it('首次加载成功：批量返回状态与数量，0 收藏补 0', async () => {
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: true, 2: false }, counts: { 1: 5 } },
    })
    const fav = store()

    // 入队即进入加载态：此时不能伪装成未收藏/0
    fav.hydrate([1, 2])
    expect(fav.favState(1)).toBe('loading')
    expect(fav.countDisplay(1).kind).toBe('loading')
    expect(fav.countText(1)).toBe('—')
    await flush()

    expect(mocks.state).toHaveBeenCalledTimes(1)
    expect(mocks.state).toHaveBeenCalledWith([1, 2])
    expect(fav.favState(1)).toBe('on')
    expect(fav.favState(2)).toBe('off')
    expect(fav.isFavorited(1)).toBe(true)
    expect(fav.isFavorited(2)).toBe(false)
    expect(fav.countOf(1)).toBe(5)
    expect(fav.countOf(2)).toBe(0) // 后端未返回的数量补 0
    expect(fav.countText(1)).toBe('5')
    expect(fav.countText(2)).toBe('0')
  })

  it('同一 tick 内多次 hydrate 合并为一次请求', async () => {
    mocks.state.mockResolvedValue({ data: { favorited: {}, counts: {} } })
    const fav = store()

    fav.hydrate([1])
    fav.hydrate([1])
    fav.hydrate([2])
    await flush()

    expect(mocks.state).toHaveBeenCalledTimes(1)
    const ids = mocks.state.mock.calls[0][0] as number[]
    expect(ids.sort()).toEqual([1, 2])
  })

  it('首次加载失败后状态保持“未知”，不会被锁成未收藏或显示为 0', async () => {
    mocks.state.mockRejectedValue(new Error('network down'))
    const fav = store()

    fav.hydrate([1])
    // 请求结束前是加载态
    expect(fav.favState(1)).toBe('loading')
    expect(fav.countDisplay(1).kind).toBe('loading')
    await flush()

    expect(mocks.state).toHaveBeenCalledTimes(1)
    expect(Object.prototype.hasOwnProperty.call(fav.favorited, 1)).toBe(false)
    expect(fav.favorited[1]).toBeUndefined()
    expect(fav.counts[1]).toBeUndefined()
    // 失败后：按钮为未知态（不是未收藏 off），数量为未知（不是 0）
    expect(fav.favState(1)).toBe('unknown')
    expect(fav.countDisplay(1).kind).toBe('unknown')
    expect(fav.countText(1)).toBe('?')
  })

  it('失败后下一次加载重新请求并恢复正确状态与数量', async () => {
    // 第一次：失败
    mocks.state.mockRejectedValueOnce(new Error('network down'))
    // 第二次（重试）：恢复
    mocks.state.mockResolvedValueOnce({
      data: { favorited: { 1: true }, counts: { 1: 3 } },
    })
    const fav = store()

    fav.hydrate([1])
    await flush()
    expect(fav.favorited[1]).toBeUndefined()
    expect(fav.favState(1)).toBe('unknown')

    // 再次加载（重新进入列表/收藏页）——因状态未知应重新请求
    fav.hydrate([1])
    expect(fav.favState(1)).toBe('loading')
    await flush()

    expect(mocks.state).toHaveBeenCalledTimes(2)
    expect(fav.favState(1)).toBe('on')
    expect(fav.isFavorited(1)).toBe(true)
    expect(fav.countOf(1)).toBe(3)
    expect(fav.countText(1)).toBe('3')
  })

  it('已成功加载的状态不会重复请求', async () => {
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: false }, counts: { 1: 0 } },
    })
    const fav = store()

    fav.hydrate([1])
    await flush()
    fav.hydrate([1]) // 已知，不再请求
    await flush()

    expect(mocks.state).toHaveBeenCalledTimes(1)
  })

  it('未登录时 hydrate 不发请求', async () => {
    mocks.auth.token = ''
    const fav = store()
    fav.hydrate([1])
    await flush()
    expect(mocks.state).not.toHaveBeenCalled()
  })
})

describe('favoriteStore 收藏/取消失败回滚', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.auth.token = 'tok'
    mocks.auth.user = { id: 100 }
    mocks.messages.length = 0
    mocks.state.mockReset()
    mocks.add.mockReset()
    mocks.cancel.mockReset()
    mocks.list.mockReset()
  })

  it('未知状态下点击不盲目收藏，而是触发重新加载状态', async () => {
    // 状态此前加载失败（未知）：点击应触发 hydrate，而不是发 addFavorite
    mocks.state.mockResolvedValueOnce({
      data: { favorited: { 1: true }, counts: { 1: 1 } },
    })
    const fav = store()
    expect(fav.favState(1)).toBe('unknown')

    const ok = await fav.toggle(product(1))

    expect(ok).toBe(false) // 本次点击不执行收藏
    expect(mocks.add).not.toHaveBeenCalled()
    expect(mocks.state).toHaveBeenCalledTimes(1) // 改为重新拉取状态
    // 重新拉取已在微任务中入队，等待其完成
    await flush()
    expect(fav.favState(1)).toBe('on')
    expect(fav.countOf(1)).toBe(1)
    expect(mocks.messages.some(([t]) => t === 'info')).toBe(true)
  })

  it('已知“未收藏”时收藏失败：回滚为未收藏与原数量', async () => {
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: false }, counts: { 1: 2 } },
    })
    mocks.add.mockRejectedValue(new Error('boom'))
    const fav = store()
    fav.hydrate([1])
    await flush()

    const ok = await fav.toggle(product(1))

    expect(ok).toBe(false)
    expect(fav.isFavorited(1)).toBe(false)
    expect(fav.countOf(1)).toBe(2)
  })

  it('已收藏时取消失败：回滚为已收藏与原数量', async () => {
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: true }, counts: { 1: 5 } },
    })
    mocks.cancel.mockRejectedValue(new Error('boom'))
    const fav = store()
    fav.hydrate([1])
    await flush()

    const ok = await fav.toggle(product(1))

    expect(ok).toBe(false)
    expect(fav.isFavorited(1)).toBe(true)
    expect(fav.countOf(1)).toBe(5)
  })

  it('收藏成功：写入服务端状态、数量，角标 +1', async () => {
    // 先确认状态为已知的“未收藏”，再收藏
    mocks.state.mockResolvedValueOnce({
      data: { favorited: { 1: false }, counts: { 1: 0 } },
    })
    const fav = store()
    fav.hydrate([1])
    await flush()

    mocks.add.mockResolvedValueOnce({
      data: { product_id: 1, favorited: true, count: 1, duplicated: false },
    })
    expect(await fav.toggle(product(1))).toBe(true)
    expect(fav.isFavorited(1)).toBe(true)
    expect(fav.countOf(1)).toBe(1)
    expect(fav.mineCount).toBe(1)
  })

  it('重复收藏（服务端返回 duplicated=true）：按钮置为已收藏，但角标不再 +1', async () => {
    // 本地状态为“未收藏”，服务端却发现该收藏早已存在（如加载失败后首次点击）
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: false }, counts: { 1: 0 } },
    })
    const fav = store()
    fav.hydrate([1])
    await flush()

    mocks.add.mockResolvedValueOnce({
      data: { product_id: 1, favorited: true, count: 1, duplicated: true },
    })
    expect(await fav.toggle(product(1))).toBe(true)
    expect(fav.isFavorited(1)).toBe(true)
    expect(fav.countOf(1)).toBe(1)
    expect(fav.mineCount).toBe(0) // 没有新增记录，角标不虚增
  })

  it('取消成功：写入未收藏与新数量，角标 -1；空取消不递减', async () => {
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: true }, counts: { 1: 5 } },
    })
    const fav = store()
    fav.hydrate([1])
    await flush()
    fav.mineCount = 3

    mocks.cancel.mockResolvedValueOnce({
      data: { product_id: 1, favorited: false, count: 4, duplicated: false },
    })
    expect(await fav.toggle(product(1))).toBe(true)
    expect(fav.isFavorited(1)).toBe(false)
    expect(fav.countOf(1)).toBe(4)
    expect(fav.mineCount).toBe(2)

    // 再次取消（本就没有记录）：duplicated=true，角标保持不变
    mocks.cancel.mockResolvedValueOnce({
      data: { product_id: 1, favorited: false, count: 4, duplicated: true },
    })
    await fav.toggle(product(1))
    expect(fav.mineCount).toBe(2)
  })

  it('不能收藏自己发布的商品：直接拦截且不发请求', async () => {
    const fav = store()
    const ok = await fav.toggle(product(1, 100)) // seller 即当前用户
    expect(ok).toBe(false)
    expect(mocks.add).not.toHaveBeenCalled()
    expect(mocks.messages.some(([t, m]) => t === 'warning' && m.includes('自己'))).toBe(true)
  })

  it('未登录收藏被拦截并提示', async () => {
    mocks.auth.token = ''
    const fav = store()
    const ok = await fav.toggle(product(1))
    expect(ok).toBe(false)
    expect(mocks.add).not.toHaveBeenCalled()
  })
})

describe('favoriteStore 账号切换与角标', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.auth.token = 'tok'
    mocks.auth.user = { id: 100 }
    mocks.messages.length = 0
    mocks.state.mockReset()
    mocks.add.mockReset()
    mocks.cancel.mockReset()
    mocks.list.mockReset()
  })

  it('reset 清空旧账号的状态、数量、加载标记与角标', async () => {
    mocks.state.mockResolvedValue({
      data: { favorited: { 1: true, 2: false }, counts: { 1: 9, 2: 0 } },
    })
    const fav = store()
    fav.hydrate([1, 2])
    await flush()
    fav.mineCount = 7

    fav.reset()

    expect(fav.favorited[1]).toBeUndefined()
    expect(fav.favorited[2]).toBeUndefined()
    expect(fav.counts[1]).toBeUndefined()
    expect(fav.counts[2]).toBeUndefined()
    expect(fav.loadingMap[1]).toBeUndefined()
    expect(fav.isLoading(1)).toBe(false)
    expect(fav.favState(1)).toBe('unknown')
    expect(fav.mineCount).toBe(0)

    // 清空后重新 hydrate 会再次请求（相当于新账号加载自己的状态）
    fav.hydrate([1])
    await flush()
    expect(mocks.state).toHaveBeenCalledTimes(2)
  })

  it('refreshMineCount 登录时用列表 total 同步角标', async () => {
    mocks.list.mockResolvedValue({ data: { total: 4 } })
    const fav = store()
    await fav.refreshMineCount()
    expect(fav.mineCount).toBe(4)
    expect(mocks.list).toHaveBeenCalledTimes(1)
  })

  it('refreshMineCount 未登录时角标置 0 且不发请求', async () => {
    mocks.auth.token = '' // 先退出，再创建 store（等价于退出登录后的新会话）
    const fav = store()
    await fav.refreshMineCount()
    expect(fav.mineCount).toBe(0)
    expect(mocks.list).not.toHaveBeenCalled()
  })
})
