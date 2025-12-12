package models

import "time"

type Coupon struct {
	ID              int
	CouponCode      string
	ExpiryDate      time.Time
	UsageType       string
	MinOrderValue   float64
	ValidFrom       *time.Time
	ValidTo         *time.Time
	DiscountType    string
	DiscountValue   float64
	MaxUsagePerUser int
	TargetType      string
	Terms           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Optimized read model for validation

type CouponMeta struct {
	Coupon
	ApplicableItems      []string
	ApplicableCategories []string
}
