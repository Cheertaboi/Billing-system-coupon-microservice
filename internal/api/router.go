package api

import (
	"database/sql"
	"net/http"

	"github.com/Cheertaboi/Billing-system-coupon-microservice/internal/api/handlers"
	"github.com/go-chi/chi/v5"
)

// NewRouter builds the HTTP router for the coupon-service
func NewRouter(db *sql.DB) http.Handler {
	r := chi.NewRouter()

	couponHandler := handlers.NewCouponHandler(db)

	// Public coupon endpoints
	r.Route("/coupons", func(r chi.Router) {
		r.Get("/applicable", couponHandler.GetApplicableCoupons)
		r.Post("/validate", couponHandler.ValidateCoupon)
	})

	// Admin endpoints
	r.Route("/admin", func(r chi.Router) {
		r.Post("/coupons", couponHandler.CreateCoupon)
	})

	// health
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return r
}
