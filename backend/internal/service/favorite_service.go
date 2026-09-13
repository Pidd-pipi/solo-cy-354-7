package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
)

// FavoriteRepository is the data access contract for favorite rows.
type FavoriteRepository interface {
	Create(ctx context.Context, f *model.Favorite) error
	Delete(ctx context.Context, userID, productID uint) (bool, error)
	Exists(ctx context.Context, userID, productID uint) (bool, error)
	ListProductIDsByUser(ctx context.Context, userID uint) (map[uint]bool, error)
	ListByUser(ctx context.Context, userID uint, status string, page, pageSize int) ([]model.Product, int64, error)
	CountByProduct(ctx context.Context, productID uint) (int64, error)
	CountsByProducts(ctx context.Context, productIDs []uint) (map[uint]int64, error)
	CountByUser(ctx context.Context, userID uint) (int64, error)
}

// FavoriteService manages a user's product favorites (bookmarks).
type FavoriteService struct {
	favorites FavoriteRepository
	products  ProductRepository
	logger    *slog.Logger
}

// NewFavoriteService wires the favorite service dependencies.
func NewFavoriteService(favorites FavoriteRepository, products ProductRepository, logger *slog.Logger) *FavoriteService {
	return &FavoriteService{favorites: favorites, products: products, logger: logger}
}

// Add bookmarks a product for the user. Favoriting one's own product is
// rejected; favoriting an already-favorited product is idempotent and keeps a
// single row, returning duplicated=true so the client can give precise feedback.
func (s *FavoriteService) Add(ctx context.Context, userID, productID uint) (*dto.FavoriteActionResponse, error) {
	p, err := s.products.FindByID(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] find product: %w", userID, productID, err), 404, constants.CodeNotFound, constants.MsgFavoriteProductGone)
	}
	if p.SellerID == userID {
		s.logger.Warn(fmt.Sprintf("favorite own product rejected: user_id=%d product_id=%d", userID, productID))
		return nil, util.NewAppError(403, constants.CodeForbidden, constants.MsgFavoriteOwnProduct,
			fmt.Errorf("favorite[user=%d product=%d] seller not match: cannot favorite own product", userID, productID))
	}

	duplicated := false
	if err := s.favorites.Create(ctx, &model.Favorite{UserID: userID, ProductID: productID}); err != nil {
		if errors.Is(err, util.ErrConflict) {
			// 同一商品重复收藏只保留一条：幂等成功，不新建记录。
			duplicated = true
			s.logger.Info(fmt.Sprintf(constants.LogFavoriteAddDuplicate, userID, productID))
		} else {
			s.logger.Error(fmt.Sprintf(constants.LogFavoriteAddFailed, userID, productID, err))
			return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] create: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
		}
	}
	if !duplicated {
		s.logger.Info(fmt.Sprintf(constants.LogFavoriteAddSuccess, userID, productID))
	}

	count, err := s.favorites.CountByProduct(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] count: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return &dto.FavoriteActionResponse{ProductID: productID, Favorited: true, Count: count, Duplicated: duplicated}, nil
}

// Cancel removes a favorite. It is idempotent: canceling a product that was not
// favorited still returns the current (false) state and count.
func (s *FavoriteService) Cancel(ctx context.Context, userID, productID uint) (*dto.FavoriteActionResponse, error) {
	deleted, err := s.favorites.Delete(ctx, userID, productID)
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogFavoriteCancelFailed, userID, productID, err))
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] delete: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogFavoriteCancelSuccess, userID, productID))

	count, err := s.favorites.CountByProduct(ctx, productID)
	if err != nil {
		// 商品可能已被物理删除，此时数量视为 0，不阻断取消操作。
		if errors.Is(err, util.ErrNotFound) {
			count = 0
		} else if err != nil {
			return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d product=%d] count: %w", userID, productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
		}
	}
	return &dto.FavoriteActionResponse{ProductID: productID, Favorited: false, Count: count, Duplicated: !deleted}, nil
}

// List returns the user's favorited products, optionally filtered by product
// status, so sold and removed items stay visible on the favorites page.
func (s *FavoriteService) List(ctx context.Context, userID uint, q *dto.ListFavoriteQuery) (*dto.PageResult, error) {
	q.Normalize()
	if q.Status != "" && !constants.IsProductStatus(q.Status) {
		return nil, util.NewAppError(400, constants.CodeValidation, constants.MsgFavoriteStatusInvalid,
			fmt.Errorf("favorite[user=%d] invalid status filter: %s", userID, q.Status))
	}
	items, total, err := s.favorites.ListByUser(ctx, userID, q.Status, q.Page, q.PageSize)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] list: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	s.logger.Info(fmt.Sprintf(constants.LogFavoriteListSuccess, userID, q.Status, len(items)))
	return &dto.PageResult{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// State returns the favorite flags and total counts for a batch of products,
// letting list/detail pages render state and counts in one request.
func (s *FavoriteService) State(ctx context.Context, userID uint, productIDs []uint) (*dto.FavoriteStateResponse, error) {
	favorited, err := s.favorites.ListProductIDsByUser(ctx, userID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] state ids: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	counts, err := s.favorites.CountsByProducts(ctx, productIDs)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[user=%d] state counts: %w", userID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	flagged := make(map[uint]bool, len(productIDs))
	for _, id := range productIDs {
		flagged[id] = favorited[id]
	}
	return &dto.FavoriteStateResponse{Favorited: flagged, Counts: counts}, nil
}

// Count returns the favorite count of one product.
func (s *FavoriteService) Count(ctx context.Context, productID uint) (*dto.FavoriteCountResponse, error) {
	count, err := s.favorites.CountByProduct(ctx, productID)
	if err != nil {
		return nil, util.WrapAppError(fmt.Errorf("favorite[product=%d] count: %w", productID, err), 500, constants.CodeInternalError, constants.MsgInternalError)
	}
	return &dto.FavoriteCountResponse{ProductID: productID, Count: count}, nil
}
