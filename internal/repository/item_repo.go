package repository

import (
	"context"
	"database/sql"
)

type ItemRepo struct {
	db *sql.DB
}

func NewItemRepo(db *sql.DB) *ItemRepo {
	return &ItemRepo{db: db}
}

func (r *ItemRepo) GetApplicableItems(ctx context.Context, couponID int) ([]string, error) {
	return []string{}, nil
}
