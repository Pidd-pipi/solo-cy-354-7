package router

import (
	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/handler"
)

// RegisterFavoriteRoutes registers favorite (bookmark) endpoints.
//
//	POST   /products/:id/favorite  收藏商品（登录）
//	DELETE /products/:id/favorite  取消收藏（登录）
//	GET    /favorites              我的收藏（登录，支持 status 筛选）
//	POST   /favorites/state        批量收藏状态与数量（登录）
//	GET    /favorites/count/:id    某商品收藏数量（公开）
func RegisterFavoriteRoutes(g *gin.RouterGroup, h *handler.FavoriteHandler, auth, apiLimiter gin.HandlerFunc) {
	// Nested under /products so the path param matches the product group.
	authedProduct := g.Group("/products", auth)
	{
		authedProduct.POST("/:id/favorite", apiLimiter, h.Add)
		authedProduct.DELETE("/:id/favorite", apiLimiter, h.Cancel)
	}

	favorites := g.Group("/favorites")
	{
		// Public count kept outside auth; listing/state require login.
		favorites.GET("/count/:id", apiLimiter, h.Count)
		authed := favorites.Group("", auth)
		{
			authed.GET("", apiLimiter, h.Mine)
			authed.POST("/state", apiLimiter, h.State)
		}
	}
}
