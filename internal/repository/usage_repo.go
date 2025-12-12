package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type UsageRepo struct {
	db *sql.DB
}

func NewUsageRepo(db *sql.DB) *UsageRepo {
	return &UsageRepo{db: db}
}

// Get or create usage row AND lock it for update
func (r *UsageRepo) GetAndLockUsage(ctx context.Context, tx *sql.Tx, couponID int, userID string) (int, error) {
	var usageCount int

	query := `
		SELECT usage_count
		FROM coupon_usage
		WHERE coupon_id = $1 AND user_id = $2
		FOR UPDATE
	`

	err := tx.QueryRowContext(ctx, query, couponID, userID).Scan(&usageCount)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Create new row
			insert := `
				INSERT INTO coupon_usage (coupon_id, user_id, usage_count, last_used)
				VALUES ($1, $2, 0, NOW())
				RETURNING usage_count
			`

			err := tx.QueryRowContext(ctx, insert, couponID, userID).Scan(&usageCount)
			if err != nil {
				return 0, err
			}

			return usageCount, nil
		}
		return 0, err
	}

	return usageCount, nil
}

// Increment usage safely inside transaction
func (r *UsageRepo) IncrementUsage(ctx context.Context, tx *sql.Tx, couponID int, userID string) error {
	query := `
		UPDATE coupon_usage
		SET usage_count = usage_count + 1,
		    last_used = $3
		WHERE coupon_id = $1 AND user_id = $2
	`

	_, err := tx.ExecContext(ctx, query, couponID, userID, time.Now())
	return err
}
