package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/middleware"
	"github.com/lp/campus-market/internal/service"
	"github.com/lp/campus-market/internal/util"
)

// FavoriteHandler exposes product favorite endpoints.
type FavoriteHandler struct {
	svc    *service.FavoriteService
	logger *slog.Logger
}

// NewFavoriteHandler wires the favorite handler dependencies.
func NewFavoriteHandler(svc *service.FavoriteService, logger *slog.Logger) *FavoriteHandler {
	return &FavoriteHandler{svc: svc, logger: logger}
}

// parseFavoriteID parses the product id from a URL path parameter.
func parseFavoriteID(c *gin.Context, param string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || id == 0 {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "商品ID不合法")
		return 0, false
	}
	return uint(id), true
}

// Add handles POST /products/:id/favorite.
func (h *FavoriteHandler) Add(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	productID, ok := parseFavoriteID(c, "id")
	if !ok {
		return
	}
	// Body is optional; it may carry product_id but the path id is authoritative.
	var body dto.AddFavoriteRequest
	_ = c.ShouldBindJSON(&body)

	res, err := h.svc.Add(c.Request.Context(), userID, productID)
	if err != nil {
		// Handler re-wraps the service error to keep error wording layered.
		_ = c.Error(fmt.Errorf("favorite handler add: %w", err))
		return
	}
	util.OK(c, res)
}

// Cancel handles DELETE /products/:id/favorite.
func (h *FavoriteHandler) Cancel(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	productID, ok := parseFavoriteID(c, "id")
	if !ok {
		return
	}
	res, err := h.svc.Cancel(c.Request.Context(), userID, productID)
	if err != nil {
		_ = c.Error(fmt.Errorf("favorite handler cancel: %w", err))
		return
	}
	util.OK(c, res)
}

// Mine handles GET /favorites — the current user's favorite list with a status filter.
func (h *FavoriteHandler) Mine(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	var q dto.ListFavoriteQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidation, constants.MsgValidationFailed)
		return
	}
	result, err := h.svc.List(c.Request.Context(), userID, &q)
	if err != nil {
		_ = c.Error(fmt.Errorf("favorite handler list: %w", err))
		return
	}
	util.OK(c, result)
}

// State handles POST /favorites/state — batch favorite flags + counts.
func (h *FavoriteHandler) State(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		util.Fail(c, http.StatusUnauthorized, constants.CodeUnauthorized, constants.MsgUnauthorized)
		return
	}
	var body struct {
		ProductIDs []uint `json:"product_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidation, constants.MsgValidationFailed)
		return
	}
	if len(body.ProductIDs) == 0 {
		util.OK(c, &dto.FavoriteStateResponse{Favorited: map[uint]bool{}, Counts: map[uint]int64{}})
		return
	}
	res, err := h.svc.State(c.Request.Context(), userID, body.ProductIDs)
	if err != nil {
		_ = c.Error(fmt.Errorf("favorite handler state: %w", err))
		return
	}
	util.OK(c, res)
}

// Count handles GET /favorites/count/:id — public favorite count of one product.
func (h *FavoriteHandler) Count(c *gin.Context) {
	productID, ok := parseFavoriteID(c, "id")
	if !ok {
		return
	}
	res, err := h.svc.Count(c.Request.Context(), productID)
	if err != nil {
		_ = c.Error(fmt.Errorf("favorite handler count: %w", err))
		return
	}
	util.OK(c, res)
}
