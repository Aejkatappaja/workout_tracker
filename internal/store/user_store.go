package store

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type password struct {
	plainText *string
	hash      []byte
}

// dummyPasswordHash lets login spend a bcrypt comparison even when the user is
// not found, so response time does not reveal whether a username exists.
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("go-gym-timing-guard"), 12)

// FakePasswordCompare burns ~one bcrypt comparison; call it on the
// user-not-found branch of login to keep timing constant.
func FakePasswordCompare() {
	_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte("x"))
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plainText = &plaintextPassword
	p.hash = hash
	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash password  `json:"-"`
	Bio          string    `json:"bio"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var AnonymousUser = &User{}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{
		db: db,
	}
}

type UserStore interface {
	CreateUser(*User) error
	GetUserByUsername(username string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	UpdateUser(*User) error
	UpdateUserPassword(userID int, plaintextPassword string) error
	GetUserToken(scope, tokenPlainText string) (*User, error)
}

func (s *PostgresUserStore) CreateUser(user *User) error {
	query := `
	INSERT INTO users (username, email, password_hash, bio)
	VALUES ($1, $2, $3, $4)
	RETURNING id, created_at, updated_at
	`

	err := s.db.QueryRow(query, user.Username, user.Email, user.PasswordHash.hash, user.Bio).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (s *PostgresUserStore) GetUserByUsername(username string) (*User, error) {
	user := &User{PasswordHash: password{}}

	query := `
	SELECT id, username, email, password_hash, bio, created_at, updated_at
	FROM users
	WHERE username = $1
	`
	err := s.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash.hash,
		&user.Bio,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *PostgresUserStore) GetUserByEmail(email string) (*User, error) {
	user := &User{PasswordHash: password{}}

	query := `
	SELECT id, username, email, password_hash, bio, created_at, updated_at
	FROM users
	WHERE email = $1
	`
	err := s.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash.hash,
		&user.Bio,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *PostgresUserStore) UpdateUserPassword(userID int, plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	query := `
	UPDATE users
	SET password_hash = $1, updated_at = CURRENT_TIMESTAMP
	WHERE id = $2
	`
	result, err := s.db.Exec(query, hash, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *PostgresUserStore) UpdateUser(user *User) error {
	query := `
	UPDATE users
	SET username = $1, email = $2, bio = $3, updated_at = CURRENT_TIMESTAMP
	WHERE id = $4
	RETURNING updated_at
	`
	result, err := s.db.Exec(query, user.Username, user.Email, user.Bio, user.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *PostgresUserStore) GetUserToken(scope, plaintextPassword string) (*User, error) {
	tokenHash := sha256.Sum256([]byte(plaintextPassword))

	query := `
  SELECT u.id, u.username, u.email, u.password_hash, u.bio, u.created_at, u.updated_at
  FROM users u
  INNER JOIN tokens t ON t.user_id = u.id
  WHERE t.hash = $1 AND t.scope = $2 and t.expiry > $3
  `

	user := &User{
		PasswordHash: password{},
	}

	err := s.db.QueryRow(query, tokenHash[:], scope, time.Now()).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash.hash,
		&user.Bio,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}
