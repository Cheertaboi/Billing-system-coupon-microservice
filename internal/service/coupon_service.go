package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yourusername/coupon-system/internal/models"
)

// Repos required by service (use interfaces to allow mocking)
type CouponRepo interface {
	GetCouponMeta(ctx context.Context, code string) (*models.CouponMeta, error)
}

type UsageRepo interface {
	GetAndLockUsage(ctx context.Context, tx *sql.Tx, couponID int, userID string) (int, error)
	IncrementUsage(ctx context.Context, tx *sql.Tx, couponID int, userID string) error
}

type CouponService struct {
	db         *sql.DB // used for transactions
	couponRepo CouponRepo
	usageRepo  UsageRepo
	// small in-memory cache (optional): map[coupon_code]*models.CouponMeta
	cache map[string]*models.CouponMeta
}

func NewCouponService(db *sql.DB, cRepo CouponRepo, uRepo UsageRepo) *CouponService {
	return &CouponService{
		db:         db,
		couponRepo: cRepo,
		usageRepo:  uRepo,
		cache:      make(map[string]*models.CouponMeta),
	}
}

// ValidateRequest and Response types -- reuse models.ValidationRequest/Response
type ValidateRequest = models.ValidationRequest
type ValidateResponse = models.ValidationResponse

// ValidateCoupon performs full validation and (if valid) consumes usage atomically.
func (s *CouponService) ValidateCoupon(ctx context.Context, req ValidateRequest) (ValidateResponse, error) {
	// short request-scoped deadline to avoid long-running ops
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	// 1) Load coupon meta (try cache first)
	var couponMeta *models.CouponMeta
	if cm, ok := s.cache[req.CouponCode]; ok {
		couponMeta = cm
	} else {
		m, err := s.couponRepo.GetCouponMeta(ctx, req.CouponCode)
		if err != nil {
			return ValidateResponse{IsValid: false, Message: "internal_error"}, err
		}
		if m == nil {
			return ValidateResponse{IsValid: false, Message: "coupon_not_found"}, nil
		}
		// store in cache (simple; you can add TTL/invalidation later)
		s.cache[req.CouponCode] = m
		couponMeta = m
	}

	now := time.Now().UTC()
	// 2) Basic validations
	if couponMeta.ExpiryDate.Before(now) {
		return ValidateResponse{IsValid: false, Message: "coupon_expired"}, nil
	}
	if couponMeta.MinOrderValue > req.OrderTotal {
		return ValidateResponse{IsValid: false, Message: "min_order_value_not_met"}, nil
	}
	if couponMeta.ValidFrom != nil && couponMeta.ValidTo != nil {
		if now.Before(*couponMeta.ValidFrom) || now.After(*couponMeta.ValidTo) {
			return ValidateResponse{IsValid: false, Message: "not_in_valid_window"}, nil
		}
	}

	// 3) Parallel item applicability checks using worker pool
	// Build a helper "isApplicable" that checks if an item matches coupon rules
	applicableMap := make(map[string]bool)
	for _, id := range couponMeta.ApplicableItems {
		applicableMap[id] = true
	}
	categoryMap := make(map[string]bool)
	for _, c := range couponMeta.ApplicableCategories {
		categoryMap[c] = true
	}

	// worker input: CartItem, output: discount contribution (float64)
	type itemIn struct {
		it models.CartItem
	}
	type itemOut struct {
		discount float64
	}

	// determine workerCount relative to cart size (but at least 2)
	workerCount := 4
	if len(req.CartItems) > 0 && len(req.CartItems) < workerCount {
		workerCount = len(req.CartItems)
		if workerCount == 0 {
			workerCount = 1
		}
	}

	inCh := make(chan itemIn)
	outCh := make(chan itemOut)

	// spawn workers
	for i := 0; i < workerCount; i++ {
		go func() {
			for in := range inCh {
				it := in.it
				// check applicability
				applies := false
				if len(applicableMap) == 0 && len(categoryMap) == 0 {
					// no restrictions -> applies to all items
					applies = true
				}
				if applicableMap[it.ID] {
					applies = true
				}
				if categoryMap[it.Category] {
					applies = true
				}
				// compute discount contribution for items (inventory target)
				discount := 0.0
				if applies && couponMeta.TargetType == "inventory" {
					if couponMeta.DiscountType == "percentage" {
						discount = float64(it.Qty) * it.Price * (couponMeta.DiscountValue / 100.0)
					} else { // flat
						// flat discount: treat as per-order flat; to avoid double counting, let worker send zero
						discount = 0.0
					}
				}
				select {
				case outCh <- itemOut{discount: discount}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// send items
	go func() {
		for _, it := range req.CartItems {
			select {
			case inCh <- itemIn{it: it}:
			case <-ctx.Done():
				break
			}
		}
		close(inCh)
	}()

	// collect results
	totalItemsDiscount := 0.0
	collectDone := make(chan struct{})
	go func() {
		for range req.CartItems {
			select {
			case o := <-outCh:
				totalItemsDiscount += o.discount
			case <-ctx.Done():
				// exit early
				break
			}
		}
		// ensure any remaining outputs drained
		close(collectDone)
	}()

	// Wait until collectors finished or context done
	select {
	case <-collectDone:
	case <-ctx.Done():
		return ValidateResponse{IsValid: false, Message: "timeout_during_item_checks"}, ctx.Err()
	}

	// compute charges discount if target_type == "charges"
	chargesDiscount := 0.0
	if couponMeta.TargetType == "charges" {
		if couponMeta.DiscountType == "percentage" {
			chargesDiscount = req.OrderTotal * (couponMeta.DiscountValue / 100.0)
		} else {
			// flat on charges
			chargesDiscount = couponMeta.DiscountValue
		}
	}

	// handle flat per-order inventory discounts:
	flatInventoryDiscount := 0.0
	if couponMeta.TargetType == "inventory" && couponMeta.DiscountType == "flat" {
		flatInventoryDiscount = couponMeta.DiscountValue
	}

	// choose final discount (simple strategy):
	totalDiscount := totalItemsDiscount
	if couponMeta.TargetType == "charges" {
		totalDiscount = chargesDiscount
	} else if couponMeta.TargetType == "inventory" {
		// if flat discount, apply flat once; if percentage, totalItemsDiscount already set
		if couponMeta.DiscountType == "flat" {
			totalDiscount = flatInventoryDiscount
		}
	}

	// 4) Concurrency-safe usage increment using DB transaction + SELECT FOR UPDATE
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ValidateResponse{IsValid: false, Message: "internal_error"}, fmt.Errorf("begin tx: %w", err)
	}
	// ensure rollback on any exit
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Get and lock usage row
	usageCount, err := s.usageRepo.GetAndLockUsage(ctx, tx, couponMeta.ID, req.UserID)
	if err != nil {
		return ValidateResponse{IsValid: false, Message: "internal_error"}, fmt.Errorf("get lock: %w", err)
	}

	// Check user-based usage constraints
	if couponMeta.UsageType == "one_time" && usageCount >= 1 {
		return ValidateResponse{IsValid: false, Message: "coupon_already_used"}, nil
	}
	if couponMeta.MaxUsagePerUser > 0 && usageCount >= couponMeta.MaxUsagePerUser {
		return ValidateResponse{IsValid: false, Message: "usage_limit_reached"}, nil
	}

	// At this point, we can increment usage (consume)
	if err := s.usageRepo.IncrementUsage(ctx, tx, couponMeta.ID, req.UserID); err != nil {
		return ValidateResponse{IsValid: false, Message: "internal_error"}, fmt.Errorf("increment usage: %w", err)
	}

	// commit
	if err := tx.Commit(); err != nil {
		return ValidateResponse{IsValid: false, Message: "internal_error"}, fmt.Errorf("tx commit: %w", err)
	}
	committed = true

	// Final response
	resp := ValidateResponse{
		IsValid:  true,
		Discount: totalDiscount,
		Message:  "coupon_applied",
	}
	return resp, nil
}
