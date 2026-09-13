package dto

// AddFavoriteRequest is the payload for bookmarking a product. The product id
// may come from the JSON body; the URL path id takes precedence in the handler.
type AddFavoriteRequest struct {
	ProductID uint `json:"product_id"`
}

// ListFavoriteQuery adds a product-status filter to the favorites pagination.
type ListFavoriteQuery struct {
	PageQuery
	Status string `form:"status"`
}

// FavoriteStateResponse maps a product id to whether the current user
// favorited it and how many users favorited it in total.
type FavoriteStateResponse struct {
	Favorited map[uint]bool  `json:"favorited"`
	Counts    map[uint]int64 `json:"counts"`
}

// FavoriteCountResponse is the favorite count of a single product.
type FavoriteCountResponse struct {
	ProductID uint  `json:"product_id"`
	Count     int64 `json:"count"`
}

// FavoriteActionResponse is returned after add/cancel so the client can sync
// the button state and the displayed count without a second round trip.
// Duplicated is true when an add hit an existing favorite (still one row), or
// when a cancel found nothing to delete.
type FavoriteActionResponse struct {
	ProductID  uint   `json:"product_id"`
	Favorited  bool   `json:"favorited"`
	Count      int64  `json:"count"`
	Duplicated bool   `json:"duplicated"`
}
