import request from '../utils/request'
import type { FavoriteAction, FavoriteState, PageResult, Product } from '../types'

// 收藏商品
export function addFavorite(productId: number) {
  return request.post<never, { code: number; message: string; data: FavoriteAction }>(
    `/products/${productId}/favorite`,
  )
}

// 取消收藏
export function cancelFavorite(productId: number) {
  return request.delete<never, { code: number; message: string; data: FavoriteAction }>(
    `/products/${productId}/favorite`,
  )
}

// 我的收藏列表，可按商品状态筛选
export function listFavorites(params: { page?: number; page_size?: number; status?: string }) {
  return request.get<never, { code: number; message: string; data: PageResult<Product> }>('/favorites', { params })
}

// 批量获取当前用户对一批商品的收藏标记与总收藏数
export function getFavoriteState(productIds: number[]) {
  return request.post<never, { code: number; message: string; data: FavoriteState }>('/favorites/state', {
    product_ids: productIds,
  })
}

// 获取单个商品的收藏数（公开接口）
export function getFavoriteCount(productId: number) {
  return request.get<never, { code: number; message: string; data: { product_id: number; count: number } }>(
    `/favorites/count/${productId}`,
  )
}
