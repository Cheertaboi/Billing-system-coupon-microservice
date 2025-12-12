package models

type CartItem struct {
	ID       string
	Category string
	Price    float64
	Qty      int
}

type CartRequest struct {
	CartItems  []CartItem `json:"cart_items"`
	OrderTotal float64    `json:"order_total"`
	Timestamp  string     `json:"timestamp"`
	CouponCode string     `json:"coupon_code"`
	UserID     string     `json:"user_id"`
}
