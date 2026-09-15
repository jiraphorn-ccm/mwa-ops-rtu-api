package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rtu-api/internal/db/sqlc"
	"github.com/rtu-api/internal/httpx"
	"github.com/rtu-api/internal/repository"
)

const bcryptCost = 10

// UserService applies the business rules of rtu.users. No roles or permissions.
type UserService struct {
	repo *repository.UserRepository
}

// UserCreateInput is the POST /users (and bootstrap register) body.
type UserCreateInput struct {
	EmployeeCode string  `json:"employee_code" validate:"required,max=20"`
	Title        *string `json:"title" validate:"omitempty,max=10"`
	FirstName    string  `json:"first_name" validate:"required,max=100"`
	LastName     string  `json:"last_name" validate:"required,max=100"`
	Email        string  `json:"email" validate:"required,email,max=100"`
	Password     string  `json:"password" validate:"required,min=8,max=72"`
	Position     *string `json:"position" validate:"omitempty,max=150"`
	Active       *bool   `json:"active"`
}

// UserUpdateInput is the PATCH /users/{id} body.
type UserUpdateInput struct {
	EmployeeCode *string `json:"employee_code" validate:"omitempty,max=20"`
	Title        *string `json:"title" validate:"omitempty,max=10"`
	FirstName    *string `json:"first_name" validate:"omitempty,max=100"`
	LastName     *string `json:"last_name" validate:"omitempty,max=100"`
	Email        *string `json:"email" validate:"omitempty,email,max=100"`
	Password     *string `json:"password" validate:"omitempty,min=8,max=72"`
	Position     *string `json:"position" validate:"omitempty,max=150"`
	Active       *bool   `json:"active"`
}

// List returns one page of users.
func (s *UserService) List(ctx context.Context, page httpx.Page, filter repository.UserFilter) ([]repository.UserListItem, int64, error) {
	return s.repo.List(ctx, page, filter)
}

// Get returns a single user without the password hash.
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (repository.UserView, error) {
	return s.repo.Get(ctx, id)
}

// Create registers a new user.
func (s *UserService) Create(ctx context.Context, in UserCreateInput) (repository.UserView, error) {
	hash, err := hashPassword(in.Password)
	if err != nil {
		return repository.UserView{}, err
	}
	return s.repo.Create(ctx, sqlc.CreateUserParams{
		EmployeeCode: strings.TrimSpace(in.EmployeeCode),
		Title:        trimPtr(in.Title),
		FirstName:    strings.TrimSpace(in.FirstName),
		LastName:     strings.TrimSpace(in.LastName),
		Email:        strings.ToLower(strings.TrimSpace(in.Email)),
		PasswordHash: hash,
		Position:     trimPtr(in.Position),
		Active:       in.Active,
	})
}

// Update applies a partial update. A new password revokes nothing here —
// callers that need to drop sessions should use AuthService.ChangePassword.
func (s *UserService) Update(ctx context.Context, id uuid.UUID, fields httpx.FieldSet, in UserUpdateInput) (repository.UserView, error) {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return repository.UserView{}, err
	}

	params := sqlc.UpdateUserParams{ID: id}

	code, setCode, err := patchRequired(fields, "employee_code", in.EmployeeCode)
	if err != nil {
		return repository.UserView{}, err
	}
	if setCode {
		code = strings.TrimSpace(code)
	}
	params.EmployeeCode, params.EmployeeCodeDoUpdate = code, setCode

	first, setFirst, err := patchRequired(fields, "first_name", in.FirstName)
	if err != nil {
		return repository.UserView{}, err
	}
	if setFirst {
		first = strings.TrimSpace(first)
	}
	params.FirstName, params.FirstNameDoUpdate = first, setFirst

	last, setLast, err := patchRequired(fields, "last_name", in.LastName)
	if err != nil {
		return repository.UserView{}, err
	}
	if setLast {
		last = strings.TrimSpace(last)
	}
	params.LastName, params.LastNameDoUpdate = last, setLast

	email, setEmail, err := patchRequired(fields, "email", in.Email)
	if err != nil {
		return repository.UserView{}, err
	}
	if setEmail {
		email = strings.ToLower(strings.TrimSpace(email))
	}
	params.Email, params.EmailDoUpdate = email, setEmail

	active, setActive, err := patchRequired(fields, "active", in.Active)
	if err != nil {
		return repository.UserView{}, err
	}
	params.Active, params.ActiveDoUpdate = active, setActive

	params.Title, params.TitleDoUpdate = patchNullable(fields, "title", trimPtr(in.Title))
	params.Position, params.PositionDoUpdate = patchNullable(fields, "position", trimPtr(in.Position))

	if fields.Has("password") {
		if in.Password == nil || strings.TrimSpace(*in.Password) == "" {
			return repository.UserView{}, httpx.Err(httpx.ErrValidationFailed).
				WithField("password", httpx.IssueRequired, "This field cannot be null.")
		}
		hash, err := hashPassword(*in.Password)
		if err != nil {
			return repository.UserView{}, err
		}
		params.PasswordHash = hash
		params.PasswordHashDoUpdate = true
	}

	return s.repo.Update(ctx, params)
}

// SoftDelete deactivates a user.
func (s *UserService) SoftDelete(ctx context.Context, id uuid.UUID) (repository.UserView, error) {
	return s.repo.SetActive(ctx, id, false)
}

// Restore reactivates a user.
func (s *UserService) Restore(ctx context.Context, id uuid.UUID) (repository.UserView, error) {
	return s.repo.SetActive(ctx, id, true)
}

// Purge removes a user permanently.
func (s *UserService) Purge(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func hashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", httpx.Err(httpx.ErrInternal).WithCause(err)
	}
	return string(hash), nil
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
