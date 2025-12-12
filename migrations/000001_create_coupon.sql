-- +goose Up
CREATE TABLE coupons (
    id SERIAL PRIMARY KEY,
    coupon_code VARCHAR(100) UNIQUE NOT NULL,
    expiry_date TIMESTAMP WITH TIME ZONE NOT NULL,
    usage_type VARCHAR(20) NOT NULL CHECK (usage_type IN ('one_time','multi_use','time_based')),
    min_order_value NUMERIC(12,2) DEFAULT 0,
    valid_from TIMESTAMP WITH TIME ZONE,
    valid_to TIMESTAMP WITH TIME ZONE,
    discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('flat','percentage')),
    discount_value NUMERIC(12,2) NOT NULL,
    max_usage_per_user INT DEFAULT 1,
    terms_and_conditions TEXT,
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('inventory','charges')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE coupon_applicable_items (
    id SERIAL PRIMARY KEY,
    coupon_id INT NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    medicine_id VARCHAR(100) NOT NULL
);

CREATE TABLE coupon_applicable_categories (
    id SERIAL PRIMARY KEY,
    coupon_id INT NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    category_name VARCHAR(100) NOT NULL
);

CREATE TABLE coupon_usage (
    id SERIAL PRIMARY KEY,
    coupon_id INT NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    user_id VARCHAR(100) NOT NULL,
    usage_count INT DEFAULT 0,
    last_used TIMESTAMP WITH TIME ZONE,
    UNIQUE (coupon_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS coupon_usage;
DROP TABLE IF EXISTS coupon_applicable_categories;
DROP TABLE IF EXISTS coupon_applicable_items;
DROP TABLE IF EXISTS coupons;
