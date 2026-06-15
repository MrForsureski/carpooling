package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"office_trip/internal/model"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	query := `
		INSERT INTO users (id, company_id, email, password_hash, full_name, phone, role, email_verified, verify_token, verify_token_at, is_active, created_at, updated_at)
		VALUES (:id, :company_id, :email, :password_hash, :full_name, :phone, :role, :email_verified, :verify_token, :verify_token_at, :is_active, NOW(), NOW())
	`
	_, err := getDbRunner(ctx, r.db).NamedExecContext(ctx, query, u)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u := &model.User{}
	err := getDbRunner(ctx, r.db).GetContext(ctx, u, `SELECT * FROM users WHERE id = $1 AND is_active = true`, id)
	if err == sql.ErrNoRows {
		return nil, model.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := getDbRunner(ctx, r.db).GetContext(ctx, u, `SELECT * FROM users WHERE email = $1`, email)
	if err == sql.ErrNoRows {
		return nil, model.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetByVerifyToken(ctx context.Context, token string) (*model.User, error) {
	u := &model.User{}
	err := getDbRunner(ctx, r.db).GetContext(ctx, u, `SELECT * FROM users WHERE verify_token = $1`, token)
	if err == sql.ErrNoRows {
		return nil, model.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role model.Role) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`,
		role, id,
	)
	return err
}

func (r *UserRepo) UpdateEmailVerified(ctx context.Context, id uuid.UUID) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE users SET email_verified = true, role = 'passenger', verify_token = NULL, verify_token_at = NULL, updated_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

func (r *UserRepo) UpdateRefreshToken(ctx context.Context, id uuid.UUID, tokenHash *string, at *time.Time) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE users SET refresh_token = $1, refresh_token_at = $2, updated_at = NOW() WHERE id = $3`,
		tokenHash, at, id,
	)
	return err
}

func (r *UserRepo) UpdateVerifyToken(ctx context.Context, id uuid.UUID, token string, at time.Time) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE users SET verify_token = $1, verify_token_at = $2, updated_at = NOW() WHERE id = $3`,
		token, at, id,
	)
	return err
}

func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]*model.User, int, error) {
	var users []*model.User
	err := getDbRunner(ctx, r.db).SelectContext(ctx, &users,
		`SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}

	var total int
	err = getDbRunner(ctx, r.db).GetContext(ctx, &total, `SELECT COUNT(*) FROM users`)
	return users, total, err
}

func (r *UserRepo) GetCompanyByEmailDomain(ctx context.Context, domain string) (*model.Company, error) {
	c := &model.Company{}
	err := getDbRunner(ctx, r.db).GetContext(ctx, c,
		`SELECT * FROM companies WHERE email_domain = $1 AND is_active = true`,
		domain,
	)
	if err == sql.ErrNoRows {
		return nil, model.ErrCompanyNotFound
	}
	return c, err
}
