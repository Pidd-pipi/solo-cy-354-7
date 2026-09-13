package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
	"gorm.io/gorm"
)

// FavoriteRepository persists user favorite rows.
type FavoriteRepository struct {
	db *gorm.DB
}

// NewFavoriteRepository builds a FavoriteRepository.
func NewFavoriteRepository(db *gorm.DB) *FavoriteRepository {
	return &FavoriteRepository{db: db}
}

// Create inserts a favorite row. A duplicate (user, product) pair is converted
// to the conflict sentinel so the service can treat it as idempotent.
func (r *FavoriteRepository) Create(ctx context.Context, f *model.Favorite) error {
	err := db(ctx, r.db).Create(f).Error
	if err != nil && isDuplicateKey(err) {
		return util.ErrConflict
	}
	return err
}

// Delete removes one favorite and reports whether a row was deleted, so the
// service can tell "canceled" apart from "was never favorited".
func (r *FavoriteRepository) Delete(ctx context.Context, userID, productID uint) (bool, error) {
	res := db(ctx, r.db).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&model.Favorite{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// Exists reports whether a user has favorited a product.
func (r *FavoriteRepository) Exists(ctx context.Context, userID, productID uint) (bool, error) {
	var n int64
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&n).Error
	return n > 0, err
}

// ListProductIDsByUser returns the set of product ids the user has favorited,
// used to mark the favorite state on product lists and detail pages.
func (r *FavoriteRepository) ListProductIDsByUser(ctx context.Context, userID uint) (map[uint]bool, error) {
	var ids []uint
	if err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("user_id = ?", userID).
		Pluck("product_id", &ids).Error; err != nil {
		return nil, err
	}
	set := make(map[uint]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// ListByUser joins favorites with products so the "my favorites" page keeps
// showing items even after they are sold or removed. Filtering by product
// status happens here via the optional status argument.
func (r *FavoriteRepository) ListByUser(ctx context.Context, userID uint, status string, page, pageSize int) ([]model.Product, int64, error) {
	base := db(ctx, r.db).
		Table("favorites").
		Joins("JOIN products ON products.id = favorites.product_id").
		Where("favorites.user_id = ?", userID)
	if status != "" {
		base = base.Where("products.status = ?", status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Product
	err := base.
		Select("products.*").
		Order("favorites.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountByProduct returns how many users have favorited a product.
func (r *FavoriteRepository) CountByProduct(ctx context.Context, productID uint) (int64, error) {
	var n int64
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("product_id = ?", productID).
		Count(&n).Error
	return n, err
}

// CountsByProducts returns a productID -> favorite-count map for a batch,
// keeping list pages to a single grouped query.
func (r *FavoriteRepository) CountsByProducts(ctx context.Context, productIDs []uint) (map[uint]int64, error) {
	out := map[uint]int64{}
	if len(productIDs) == 0 {
		return out, nil
	}
	type row struct {
		ProductID uint
		N         int64
	}
	var rows []row
	if err := db(ctx, r.db).Model(&model.Favorite{}).
		Select("product_id, COUNT(*) AS n").
		Where("product_id IN ?", productIDs).
		Group("product_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.ProductID] = rw.N
	}
	return out, nil
}

// CountByUser returns the number of products the user has favorited.
func (r *FavoriteRepository) CountByUser(ctx context.Context, userID uint) (int64, error) {
	var n int64
	err := db(ctx, r.db).Model(&model.Favorite{}).
		Where("user_id = ?", userID).
		Count(&n).Error
	return n, err
}

// isDuplicateKey detects the MySQL 1062 / SQLite UNIQUE constraint violation.
func isDuplicateKey(err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "error 1062") || strings.Contains(msg, "duplicate entry") || strings.Contains(msg, "unique constraint") {
		return true
	}
	return errors.Is(err, gorm.ErrDuplicatedKey)
}
