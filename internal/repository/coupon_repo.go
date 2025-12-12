package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yourusername/coupon-system/internal/models"
)

type CouponRepo struct {
	db *sql.DB
}

func NewCouponRepo(db *sql.DB) *CouponRepo {
	return &CouponRepo{db: db}
}

func (r *CouponRepo) GetCouponMeta(ctx context.Context, code string) (*models.CouponMeta, error) {
	var c models.Coupon

	query := `
		SELECT id, coupon_code, expiry_date, usage_type, min_order_value,
		       valid_from, valid_to, discount_type, discount_value,
		       max_usage_per_user, target_type, terms_and_conditions,
		       created_at, updated_at
		FROM coupons
		WHERE coupon_code = $1;
	`

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&c.ID,
		&c.CouponCode,
		&c.ExpiryDate,
		&c.UsageType,
		&c.MinOrderValue,
		&c.ValidFrom,
		&c.ValidTo,
		&c.DiscountType,
		&c.DiscountValue,
		&c.MaxUsagePerUser,
		&c.TargetType,
		&c.Terms,
		&c.CreatedAt,
		&c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	items, err := r.getApplicableItems(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	categories, err := r.getApplicableCategories(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	return &models.CouponMeta{
		Coupon:               c,
		ApplicableItems:      items,
		ApplicableCategories: categories,
	}, nil
}

func (r *CouponRepo) getApplicableItems(ctx context.Context, couponID int) ([]string, error) {
	query := `SELECT medicine_id FROM coupon_applicable_items WHERE coupon_id = $1`
	rows, err := r.db.QueryContext(ctx, query, couponID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, nil
}

func (r *CouponRepo) getApplicableCategories(ctx context.Context, couponID int) ([]string, error) {
	query := `SELECT category_name FROM coupon_applicable_categories WHERE coupon_id = $1`
	rows, err := r.db.QueryContext(ctx, query, couponID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		categories = append(categories, name)
	}
	return categories, nil
}
