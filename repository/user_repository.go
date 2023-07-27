package repository

import (
	"context"
	"fmt"
	"login-example/entity"
	"time"

	"github.com/jmoiron/sqlx"
)

type IUserRepository interface {
	PreRegister(ctx context.Context, u *entity.User) error
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Delete(ctx context.Context, id entity.UserID) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) IUserRepository {
	return &userRepository{db: db}
}

// ユーザーをstate=inactiveで保存する
func (r *userRepository) PreRegister(ctx context.Context, u *entity.User) error {
	u.UpdatedAt = time.Now()
	u.CreatedAt = time.Now()
	u.State = entity.UserInactive

	query := `INSERT INTO user (
		email, password, salt, activate_token, state, updated_at, created_at
	) VALUES (:email, :password, :salt, :activate_token, :state, :updated_at, :created_at)`
	result, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		return fmt.Errorf("failed to Exec: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to LastInsertId: %w", err)
	}

	u.ID = entity.UserID(id)
	return nil
}

// emailからユーザーを取得する、対象のユーザーが存在しなかった場合、user=nilではないので注意
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `SELECT 
		id, email, password, salt, state, activate_token, updated_at, created_at
		FROM user WHERE email = ?`
	u := &entity.User{}
	// 対象のユーザーが存在しない場合、sql.ErrNoRowsがエラーで返ってくる
	if err := r.db.GetContext(ctx, u, query, email); err != nil {
		return nil, fmt.Errorf("failed to get: %w", err)
	}
	return u, nil
}

// ユーザーを削除する
func (r *userRepository) Delete(ctx context.Context, id entity.UserID) error {
	query := `DELETE FROM user WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
