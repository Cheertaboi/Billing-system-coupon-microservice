package models

type ValidationRequest struct {
	UserID     string
	CouponCode string
	CartItems  []CartItem
	OrderTotal float64
}

type ValidationResponse struct {
	IsValid  bool    `json:"is_valid"`
	Discount float64 `json:"discount,omitempty"`
	Message  string  `json:"message"`
}
