package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rtu-api/internal/db"
	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
)

// UserRepository reads and writes rtu.users.
type UserRepository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var userSortable = httpx.Sortable{
	"employee_code": "u.employee_code",
	"first_name":    "u.first_name",
	"last_name":     "u.last_name",
	"email":         "u.email",
	"active":        "u.active",
	"created_at":    "u.created_at",
	"updated_at":    "u.updated_at",
}

// UserSortable lists the sort keys accepted by GET /users.
func UserSortable() httpx.Sortable { return userSortable }

// UserFilter narrows a user list query.
type UserFilter struct {
	Active *bool
}

const userListSelect = `
SELECT
    u.id, u.employee_code, u.title, u.first_name, u.last_name, u.email,
    u.position, u.active, u.last_login_at, u.created_at, u.updated_at,
    u.created_by, u.updated_by,
    count(*) OVER ()::bigint AS total_count
FROM rtu.users u
WHERE %s
ORDER BY %s %s, u.id %s
LIMIT %s OFFSET %s`

// UserView is a user row without the password hash.
type UserView struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	EmployeeCode string     `db:"employee_code" json:"employee_code"`
	Title        *string    `db:"title" json:"title"`
	FirstName    string     `db:"first_name" json:"first_name"`
	LastName     string     `db:"last_name" json:"last_name"`
	Email        string     `db:"email" json:"email"`
	Position     *string    `db:"position" json:"position"`
	Active       bool       `db:"active" json:"active"`
	LastLoginAt  *time.Time `db:"last_login_at" json:"last_login_at"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	CreatedBy    *uuid.UUID `db:"created_by" json:"created_by"`
	UpdatedBy    *uuid.UUID `db:"updated_by" json:"updated_by"`
	FullName     string     `db:"-" json:"full_name"`
}

// UserListItem is a UserView plus the window total.
type UserListItem struct {
	UserView
	TotalCount int64 `db:"total_count" json:"-"`
}

// PublicUser strips the password hash and fills full_name.
func PublicUser(u sqlc.User) UserView {
	v := UserView{
		ID:           u.ID,
		EmployeeCode: u.EmployeeCode,
		Title:        u.Title,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Email:        u.Email,
		Position:     u.Position,
		Active:       u.Active,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		CreatedBy:    u.CreatedBy,
		UpdatedBy:    u.UpdatedBy,
	}
	v.FullName = userFullName(v.Title, v.FirstName, v.LastName)
	return v
}

func userFullName(title *string, first, last string) string {
	parts := make([]string, 0, 3)
	if title != nil && strings.TrimSpace(*title) != "" {
		parts = append(parts, strings.TrimSpace(*title))
	}
	if first != "" {
		parts = append(parts, first)
	}
	if last != "" {
		parts = append(parts, last)
	}
	return strings.Join(parts, " ")
}

// List returns one page of users (never including password hashes).
func (r *UserRepository) List(ctx context.Context, page httpx.Page, filter UserFilter) ([]UserListItem, int64, error) {
	a := &args{}
	conds := conditions{}

	if filter.Active != nil {
		conds = append(conds, "u.active = "+a.add(*filter.Active))
	}
	if page.Search != nil {
		p := a.add(likePattern(*page.Search))
		conds = append(conds, fmt.Sprintf(
			`(u.employee_code ILIKE %s ESCAPE '\' OR u.email ILIKE %s ESCAPE '\' OR u.first_name ILIKE %s ESCAPE '\' OR u.last_name ILIKE %s ESCAPE '\')`,
			p, p, p, p,
		))
	}

	query := fmt.Sprintf(userListSelect,
		conds.where(), page.SortSQL, page.Order, page.Order,
		a.add(page.RowLimit()), a.add(page.Offset()),
	)

	rows, err := r.pool.Query(ctx, query, a.values...)
	if err != nil {
		return nil, 0, db.Translate(err)
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[UserListItem])
	if err != nil {
		return nil, 0, db.Translate(err)
	}
	for i := range items {
		items[i].FullName = userFullName(items[i].Title, items[i].FirstName, items[i].LastName)
	}

	var total int64
	if len(items) > 0 {
		total = items[0].TotalCount
	}
	return items, total, nil
}

// Get returns a public user by id.
func (r *UserRepository) Get(ctx context.Context, id uuid.UUID) (UserView, error) {
	user, err := r.GetAuth(ctx, id)
	if err != nil {
		return UserView{}, err
	}
	return PublicUser(user), nil
}

// GetAuth returns the full row including password_hash. Never serialise this.
func (r *UserRepository) GetAuth(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	user, err := r.q.GetUser(ctx, id)
	if err != nil {
		return sqlc.User{}, db.Translate(err, db.WithNotFound(httpx.ErrUserNotFound))
	}
	return user, nil
}

// GetByLogin finds a user by email (case-insensitive) or employee_code.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (sqlc.User, error) {
	user, err := r.q.GetUserByLogin(ctx, login)
	if err != nil {
		return sqlc.User{}, db.Translate(err, db.WithNotFound(httpx.ErrInvalidCredentials))
	}
	return user, nil
}

// Count returns the number of users. Used to gate bootstrap registration.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.q.CountUsers(ctx)
	if err != nil {
		return 0, db.Translate(err)
	}
	return n, nil
}

func userWriteConstraints() db.Options {
	return db.Options{Constraints: db.Constraints{
		"uk_users_email":         httpx.ErrUserEmailDup,
		"uk_users_email_lower":   httpx.ErrUserEmailDup,
		"uk_users_employee_code": httpx.ErrUserEmployeeDup,
	}}
}

// Create inserts a user.
func (r *UserRepository) Create(ctx context.Context, arg sqlc.CreateUserParams) (UserView, error) {
	arg.CreatedBy, arg.UpdatedBy = createAudit(ctx)
	user, err := r.q.CreateUser(ctx, arg)
	if err != nil {
		return UserView{}, db.Translate(err, userWriteConstraints())
	}
	return PublicUser(user), nil
}

// Update applies a partial update.
func (r *UserRepository) Update(ctx context.Context, arg sqlc.UpdateUserParams) (UserView, error) {
	arg.UpdatedBy = updateAudit(ctx)
	user, err := r.q.UpdateUser(ctx, arg)
	if err != nil {
		opt := userWriteConstraints()
		nf := httpx.ErrUserNotFound
		opt.NotFound = &nf
		return UserView{}, db.Translate(err, opt)
	}
	return PublicUser(user), nil
}

// SetActive soft-deletes or restores a user.
func (r *UserRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (UserView, error) {
	user, err := r.q.SetUserActive(ctx, sqlc.SetUserActiveParams{
		ID: id, Active: active, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return UserView{}, db.Translate(err, db.WithNotFound(httpx.ErrUserNotFound))
	}
	return PublicUser(user), nil
}

// SetPassword replaces the password hash.
func (r *UserRepository) SetPassword(ctx context.Context, id uuid.UUID, hash string) (UserView, error) {
	user, err := r.q.SetUserPassword(ctx, sqlc.SetUserPasswordParams{
		ID: id, PasswordHash: hash, UpdatedBy: updateAudit(ctx),
	})
	if err != nil {
		return UserView{}, db.Translate(err, db.WithNotFound(httpx.ErrUserNotFound))
	}
	return PublicUser(user), nil
}

// TouchLastLogin records a successful sign-in.
func (r *UserRepository) TouchLastLogin(ctx context.Context, id uuid.UUID) error {
	if err := r.q.TouchUserLastLogin(ctx, id); err != nil {
		return db.Translate(err)
	}
	return nil
}

// Delete removes a user permanently.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	affected, err := r.q.DeleteUser(ctx, id)
	if err != nil {
		return db.Translate(err)
	}
	if affected == 0 {
		return httpx.Err(httpx.ErrUserNotFound)
	}
	return nil
}
