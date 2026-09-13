package service

import (
	"context"
	"log/slog"
	"sort"
	"testing"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
)

// fakeFavoriteRepo is an in-memory FavoriteRepository for service tests.
type fakeFavoriteRepo struct {
	rows      []model.Favorite
	nextID    uint
	failCount bool
}

func newFakeFavoriteRepo() *fakeFavoriteRepo {
	return &fakeFavoriteRepo{nextID: 1}
}

func (f *fakeFavoriteRepo) Create(_ context.Context, fav *model.Favorite) error {
	for _, r := range f.rows {
		if r.UserID == fav.UserID && r.ProductID == fav.ProductID {
			return util.ErrConflict
		}
	}
	fav.ID = f.nextID
	f.nextID++
	f.rows = append(f.rows, *fav)
	return nil
}

func (f *fakeFavoriteRepo) Delete(_ context.Context, userID, productID uint) (bool, error) {
	out := f.rows[:0]
	deleted := false
	for _, r := range f.rows {
		if r.UserID == userID && r.ProductID == productID {
			deleted = true
			continue
		}
		out = append(out, r)
	}
	f.rows = out
	return deleted, nil
}

func (f *fakeFavoriteRepo) Exists(_ context.Context, userID, productID uint) (bool, error) {
	for _, r := range f.rows {
		if r.UserID == userID && r.ProductID == productID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeFavoriteRepo) ListProductIDsByUser(_ context.Context, userID uint) (map[uint]bool, error) {
	set := map[uint]bool{}
	for _, r := range f.rows {
		if r.UserID == userID {
			set[r.ProductID] = true
		}
	}
	return set, nil
}

func (f *fakeFavoriteRepo) ListByUser(_ context.Context, userID uint, status string, page, pageSize int) ([]model.Product, int64, error) {
	// The status-filtered list path is exercised by favoriteListRepo below.
	return nil, 0, nil
}

func (f *fakeFavoriteRepo) CountByProduct(_ context.Context, productID uint) (int64, error) {
	var n int64
	for _, r := range f.rows {
		if r.ProductID == productID {
			n++
		}
	}
	return n, nil
}

func (f *fakeFavoriteRepo) CountsByProducts(_ context.Context, ids []uint) (map[uint]int64, error) {
	out := map[uint]int64{}
	for _, id := range ids {
		n, _ := f.CountByProduct(context.Background(), id)
		if n > 0 {
			out[id] = n
		}
	}
	return out, nil
}

func (f *fakeFavoriteRepo) CountByUser(_ context.Context, userID uint) (int64, error) {
	var n int64
	for _, r := range f.rows {
		if r.UserID == userID {
			n++
		}
	}
	return n, nil
}

func setupFavoriteService() (*FavoriteService, *fakeProductRepo, *fakeFavoriteRepo) {
	products := newFakeProductRepo()
	favorites := newFakeFavoriteRepo()
	svc := NewFavoriteService(favorites, products, slog.Default())
	return svc, products, favorites
}

// CreateSvcProduct inserts an on-sale product owned by sellerID directly into
// the fake product repo, bypassing service validation.
func (f *fakeProductRepo) CreateSvcProduct(sellerID uint) (*model.Product, error) {
	p := &model.Product{
		SellerID: sellerID, Title: "测试商品", Price: 10,
		Category: constants.ProductCategoryBooks, Condition: "全新",
		Campus: "东校区", TradeLocation: "东门", Status: constants.ProductStatusOnSale,
	}
	err := f.Create(context.Background(), p)
	return p, err
}

func TestFavoriteAddAndCount(t *testing.T) {
	svc, products, favorites := setupFavoriteService()
	p, _ := products.CreateSvcProduct(7) // product owned by user 7

	res, err := svc.Add(context.Background(), 1, p.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Favorited || res.Count != 1 || res.Duplicated {
		t.Fatalf("unexpected add result: %+v", res)
	}
	if len(favorites.rows) != 1 {
		t.Fatalf("expected 1 favorite row, got %d", len(favorites.rows))
	}
}

func TestFavoriteOwnProductRejected(t *testing.T) {
	svc, products, _ := setupFavoriteService()
	p, _ := products.CreateSvcProduct(1) // owned by the same user 1

	_, err := svc.Add(context.Background(), 1, p.ID)
	if err == nil {
		t.Fatalf("expected error when favoriting own product")
	}
	if appErr, ok := err.(*util.AppError); !ok || appErr.Status != 403 {
		t.Fatalf("expected 403 AppError, got %T %v", err, err)
	}
}

func TestFavoriteDuplicateKeepsOneRow(t *testing.T) {
	svc, products, favorites := setupFavoriteService()
	p, _ := products.CreateSvcProduct(7)

	if _, err := svc.Add(context.Background(), 1, p.ID); err != nil {
		t.Fatalf("first add: %v", err)
	}
	res, err := svc.Add(context.Background(), 1, p.ID)
	if err != nil {
		t.Fatalf("duplicate add should be idempotent, got %v", err)
	}
	if !res.Duplicated {
		t.Fatalf("expected duplicated=true")
	}
	if !res.Favorited || res.Count != 1 {
		t.Fatalf("unexpected duplicate result: %+v", res)
	}
	if len(favorites.rows) != 1 {
		t.Fatalf("expected exactly 1 row after duplicate, got %d", len(favorites.rows))
	}
}

func TestFavoriteCancel(t *testing.T) {
	svc, products, favorites := setupFavoriteService()
	p, _ := products.CreateSvcProduct(7)
	_, _ = svc.Add(context.Background(), 1, p.ID)

	res, err := svc.Cancel(context.Background(), 1, p.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.Favorited || res.Count != 0 {
		t.Fatalf("unexpected cancel result: %+v", res)
	}
	if len(favorites.rows) != 0 {
		t.Fatalf("expected 0 rows after cancel")
	}

	// Canceling again is idempotent and marked duplicated (nothing to delete).
	res2, err := svc.Cancel(context.Background(), 1, p.ID)
	if err != nil {
		t.Fatalf("second cancel should be idempotent: %v", err)
	}
	if !res2.Duplicated || res2.Favorited {
		t.Fatalf("unexpected second cancel: %+v", res2)
	}
}

func TestFavoriteAddMissingProduct(t *testing.T) {
	svc, _, _ := setupFavoriteService()
	if _, err := svc.Add(context.Background(), 1, 9999); err == nil {
		t.Fatalf("expected error favoriting nonexistent product")
	}
}

func TestFavoriteInvalidStatusFilter(t *testing.T) {
	svc, _, _ := setupFavoriteService()
	_, err := svc.List(context.Background(), 1, &dto.ListFavoriteQuery{Status: "not_a_status"})
	if err == nil {
		t.Fatalf("expected validation error for bad status")
	}
}

func TestFavoriteStateBatch(t *testing.T) {
	svc, products, _ := setupFavoriteService()
	p1, _ := products.CreateSvcProduct(7)
	p2, _ := products.CreateSvcProduct(8)
	_, _ = svc.Add(context.Background(), 1, p1.ID)
	_, _ = svc.Add(context.Background(), 2, p1.ID)

	res, err := svc.State(context.Background(), 1, []uint{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if !res.Favorited[p1.ID] || res.Favorited[p2.ID] {
		t.Fatalf("unexpected favorited flags: %+v", res.Favorited)
	}
	if res.Counts[p1.ID] != 2 || res.Counts[p2.ID] != 0 {
		t.Fatalf("unexpected counts: %+v", res.Counts)
	}
}

// --- product-backed list/filter test -------------------------------------

// favoriteListRepo joins favorites with an in-memory product store so the
// status filter can be exercised without a database.
type favoriteListRepo struct {
	*fakeFavoriteRepo
	products map[uint]model.Product
}

func (r *favoriteListRepo) ListByUser(_ context.Context, userID uint, status string, _, _ int) ([]model.Product, int64, error) {
	ids := []uint{}
	for _, row := range r.fakeFavoriteRepo.rows {
		if row.UserID != userID {
			continue
		}
		p := r.products[row.ProductID]
		if status != "" && p.Status != status {
			continue
		}
		ids = append(ids, row.ProductID)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	out := make([]model.Product, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.products[id])
	}
	return out, int64(len(out)), nil
}

func TestFavoriteListStatusFilter(t *testing.T) {
	products := newFakeProductRepo()
	onSale, _ := products.CreateSvcProduct(7)
	sold, _ := products.CreateSvcProduct(8)
	removed, _ := products.CreateSvcProduct(9)
	products.products[sold.ID].Status = constants.ProductStatusSold
	products.products[removed.ID].Status = constants.ProductStatusRemoved

	repo := &favoriteListRepo{fakeFavoriteRepo: newFakeFavoriteRepo(), products: map[uint]model.Product{}}
	for _, p := range []*model.Product{onSale, sold, removed} {
		repo.products[p.ID] = *products.products[p.ID]
		_ = repo.Create(context.Background(), &model.Favorite{UserID: 1, ProductID: p.ID})
	}
	svc := NewFavoriteService(repo, products, slog.Default())

	cases := []struct {
		name   string
		status string
		want   int64
	}{
		{"all", "", 3},
		{"on sale", constants.ProductStatusOnSale, 1},
		{"sold", constants.ProductStatusSold, 1},
		{"removed", constants.ProductStatusRemoved, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.List(context.Background(), 1, &dto.ListFavoriteQuery{Status: tc.status})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if res.Total != tc.want {
				t.Fatalf("expected %d items for %s, got %d", tc.want, tc.name, res.Total)
			}
		})
	}
}
