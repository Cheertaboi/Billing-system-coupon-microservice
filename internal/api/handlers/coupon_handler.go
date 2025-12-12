package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Cheertaboi/Billing-system-coupon-microservice/internal/models"
	"github.com/Cheertaboi/Billing-system-coupon-microservice/internal/repository"
	"github.com/Cheertaboi/Billing-system-coupon-microservice/internal/service"
)

// --- Request / Response DTOs ---

type CreateCouponRequest struct {
	CouponCode      string   `json:"coupon_code"`
	ExpiryDate      string   `json:"expiry_date"` // RFC3339 string
	UsageType       string   `json:"usage_type"`
	MinOrderValue   float64  `json:"min_order_value"`
	ValidFrom       string   `json:"valid_from,omitempty"`
	ValidTo         string   `json:"valid_to,omitempty"`
	DiscountType    string   `json:"discount_type"`
	DiscountValue   float64  `json:"discount_value"`
	MaxUsagePerUser int      `json:"max_usage_per_user"`
	TargetType      string   `json:"target_type"`
	Terms           string   `json:"terms_and_conditions,omitempty"`
	Items           []string `json:"applicable_medicine_ids,omitempty"`
	Categories      []string `json:"applicable_categories,omitempty"`
}

type ValidateRequestBody struct {
	UserID     string            `json:"user_id"`
	Coupon     string            `json:"coupon_code"`
	CartItems  []models.CartItem `json:"cart_items"`
	OrderTotal float64           `json:"order_total"`
	Timestamp  string            `json:"timestamp"` // optional, RFC3339
}

type ApplicableRequestBody struct {
	UserID     string            `json:"user_id"`
	CartItems  []models.CartItem `json:"cart_items"`
	OrderTotal float64           `json:"order_total"`
	Timestamp  string            `json:"timestamp"` // optional, RFC3339
}

type ApplicableResponse struct {
	ApplicableCoupons []string `json:"applicable_coupons"`
}

// --- Handler struct & constructor ---

type CouponHandler struct {
	db         *sql.DB
	couponRepo *repository.CouponRepo
	usageRepo  *repository.UsageRepo
	service    *service.CouponService
}

func NewCouponHandler(db *sql.DB) *CouponHandler {
	cRepo := repository.NewCouponRepo(db)
	uRepo := repository.NewUsageRepo(db)

	// service expects interfaces; pass repository implementations
	svc := service.NewCouponService(db, cRepo, uRepo)

	return &CouponHandler{
		db:         db,
		couponRepo: cRepo,
		usageRepo:  uRepo,
		service:    svc,
	}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func parseTimeOrEmpty(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// --- Handlers ---

// CreateCoupon handles POST /admin/coupons
// creates coupon record + items + categories in a transaction
func (h *CouponHandler) CreateCoupon(w http.ResponseWriter, r *http.Request) {
	var req CreateCouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	// basic validation
	if req.CouponCode == "" || req.DiscountValue <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "coupon_code and discount_value required"})
		return
	}

	// parse dates
	expiry, err := time.Parse(time.RFC3339, req.ExpiryDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expiry_date; use RFC3339"})
		return
	}
	validFrom, err := parseTimeOrEmpty(req.ValidFrom)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid valid_from; use RFC3339"})
		return
	}
	validTo, err := parseTimeOrEmpty(req.ValidTo)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid valid_to; use RFC3339"})
		return
	}

	// start tx
	ctx := r.Context()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could_not_start_tx"})
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// insert coupon
	insertCoupon := `
		INSERT INTO coupons
		(coupon_code, expiry_date, usage_type, min_order_value, valid_from, valid_to,
		 discount_type, discount_value, max_usage_per_user, target_type, terms_and_conditions, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())
		RETURNING id
	`
	var couponID int
	err = tx.QueryRowContext(ctx, insertCoupon,
		req.CouponCode,
		expiry,
		req.UsageType,
		req.MinOrderValue,
		validFrom,
		validTo,
		req.DiscountType,
		req.DiscountValue,
		req.MaxUsagePerUser,
		req.TargetType,
		req.Terms,
	).Scan(&couponID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed_create_coupon"})
		return
	}

	// insert items
	if len(req.Items) > 0 {
		stmt := `INSERT INTO coupon_applicable_items (coupon_id, medicine_id) VALUES ($1, $2)`
		for _, mid := range req.Items {
			if _, err := tx.ExecContext(ctx, stmt, couponID, mid); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed_create_items"})
				return
			}
		}
	}

	// insert categories
	if len(req.Categories) > 0 {
		stmt := `INSERT INTO coupon_applicable_categories (coupon_id, category_name) VALUES ($1, $2)`
		for _, cat := range req.Categories {
			if _, err := tx.ExecContext(ctx, stmt, couponID, cat); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed_create_categories"})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "commit_failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":   "coupon_created",
		"coupon_id": couponID,
	})
}

// ValidateCoupon handles POST /coupons/validate
func (h *CouponHandler) ValidateCoupon(w http.ResponseWriter, r *http.Request) {
	var req ValidateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	// build service request
	vr := models.ValidationRequest{
		UserID:     req.UserID,
		CouponCode: req.Coupon,
		CartItems:  req.CartItems,
		OrderTotal: req.OrderTotal,
	}

	// if timestamp provided parse it (override)
	if strings.TrimSpace(req.Timestamp) != "" {
		if t, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			// service currently doesn't use timestamp in request struct, but we keep for future
			_ = t
		}
	}

	ctx := r.Context()
	resp, err := h.service.ValidateCoupon(ctx, vr)
	if err != nil {
		// internal error
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "detail": err.Error()})
		return
	}

	if !resp.IsValid {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"is_valid": false,
			"message":  resp.Message,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"is_valid": true,
		"discount": resp.Discount,
		"message":  resp.Message,
	})
}

// GetApplicableCoupons handles GET /coupons/applicable
// Accepts cart (via query params or JSON). We'll accept JSON body (POST would be okay; assignment wanted GET — we'll support GET with query 'user' and JSON body fallback)
func (h *CouponHandler) GetApplicableCoupons(w http.ResponseWriter, r *http.Request) {
	// allow GET with body (some clients don't allow GET body; if empty, return bad request)
	var req ApplicableRequestBody
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return
		}
	} else {
		// try query params: user, order_total, timestamp, items (items as id:qty:price:category comma-separated)
		user := r.URL.Query().Get("user")
		orderTotalStr := r.URL.Query().Get("order_total")
		ts := r.URL.Query().Get("timestamp")
		itemsRaw := r.URL.Query().Get("items") // format: id|category|price|qty, id|category|price|qty
		if user == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user required"})
			return
		}
		req.UserID = user
		if orderTotalStr != "" {
			if f, err := strconv.ParseFloat(orderTotalStr, 64); err == nil {
				req.OrderTotal = f
			}
		}
		req.Timestamp = ts
		if itemsRaw != "" {
			parts := strings.Split(itemsRaw, ",")
			for _, p := range parts {
				fields := strings.Split(p, "|")
				if len(fields) < 4 {
					continue
				}
				price, _ := strconv.ParseFloat(fields[2], 64)
				qty, _ := strconv.Atoi(fields[3])
				req.CartItems = append(req.CartItems, models.CartItem{
					ID:       fields[0],
					Category: fields[1],
					Price:    price,
					Qty:      qty,
				})
			}
		}
	}

	// timestamp parse
	var now time.Time
	if strings.TrimSpace(req.Timestamp) != "" {
		if t, err := time.Parse(time.RFC3339, req.Timestamp); err == nil {
			now = t.UTC()
		} else {
			now = time.Now().UTC()
		}
	} else {
		now = time.Now().UTC()
	}

	// get all coupon codes (simple approach)
	const allCouponsQ = `SELECT id, coupon_code, expiry_date, min_order_value, valid_from, valid_to, usage_type, max_usage_per_user FROM coupons`
	rows, err := h.db.QueryContext(r.Context(), allCouponsQ)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed_list_coupons"})
		return
	}
	defer rows.Close()

	applicable := []string{}

	for rows.Next() {
		var id int
		var code string
		var expiry time.Time
		var minOrder float64
		var validFrom sql.NullTime
		var validTo sql.NullTime
		var usageType string
		var maxUsage sql.NullInt64

		if err := rows.Scan(&id, &code, &expiry, &minOrder, &validFrom, &validTo, &usageType, &maxUsage); err != nil {
			continue
		}

		// basic checks
		if expiry.Before(now) {
			continue
		}
		if minOrder > req.OrderTotal {
			continue
		}
		if validFrom.Valid && validTo.Valid {
			if now.Before(validFrom.Time) || now.After(validTo.Time) {
				continue
			}
		}

		// check user usage count (non-locking read)
		var usageCount int
		ucQuery := `SELECT usage_count FROM coupon_usage WHERE coupon_id=$1 AND user_id=$2`
		row := h.db.QueryRowContext(r.Context(), ucQuery, id, req.UserID)
		switch err := row.Scan(&usageCount); err {
		case nil:
			// ok
		case sql.ErrNoRows:
			usageCount = 0
		default:
			// on error, be conservative and skip this coupon
			continue
		}
		if maxUsage.Valid && int64(usageCount) >= maxUsage.Int64 {
			continue
		}
		if usageType == "one_time" && usageCount >= 1 {
			continue
		}

		// fetch meta to test item/category applicability
		meta, err := h.couponRepo.GetCouponMeta(r.Context(), code)
		if err != nil || meta == nil {
			continue
		}

		// evaluate if any cart item matches rules (if coupon has restrictions)
		applies := false
		if len(meta.ApplicableItems) == 0 && len(meta.ApplicableCategories) == 0 {
			// no restrictions => applies to whole cart
			applies = true
		} else {
			itemSet := make(map[string]bool)
			for _, iid := range meta.ApplicableItems {
				itemSet[iid] = true
			}
			catSet := make(map[string]bool)
			for _, c := range meta.ApplicableCategories {
				catSet[c] = true
			}
			for _, it := range req.CartItems {
				if itemSet[it.ID] || catSet[it.Category] {
					applies = true
					break
				}
			}
		}

		if applies {
			applicable = append(applicable, code)
		}
	}

	writeJSON(w, http.StatusOK, ApplicableResponse{ApplicableCoupons: applicable})
}
