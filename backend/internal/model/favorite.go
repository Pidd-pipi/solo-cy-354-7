package model

import "time"

// Favorite is a user's bookmark of a second-hand product. A user may only
// favorite a given product once; the composite unique index enforces this at
// the storage layer so duplicate clicks never create a second row.
type Favorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null;uniqueIndex:uniq_user_product,priority:1" json:"user_id"`
	ProductID uint      `gorm:"index;not null;uniqueIndex:uniq_user_product,priority:2" json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
}
