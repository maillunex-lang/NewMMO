package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/sha256"
	"errors"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	tokenTTL  = 12 * time.Hour
	ticketTTL = 30 * time.Second
)

type Failure struct {
	Code         string
	RetryAfterMS int64
}

func (e *Failure) Error() string { return e.Code }

var (
	ErrInternal           = &Failure{Code: "INTERNAL"}
	ErrUnauthenticated    = &Failure{Code: "UNAUTHENTICATED"}
	ErrInvalidCredentials = &Failure{Code: "INVALID_CREDENTIALS"}
	ErrLoginTaken         = &Failure{Code: "LOGIN_TAKEN"}
	ErrNameTaken          = &Failure{Code: "NAME_TAKEN"}
	ErrLimitReached       = &Failure{Code: "LIMIT_REACHED"}
	ErrForbidden          = &Failure{Code: "FORBIDDEN"}
	ErrNotFound           = &Failure{Code: "NOT_FOUND"}
	ErrInvalidTicket      = errors.New("invalid or expired session ticket")
)

type Store struct {
	pool      *pgxpool.Pool
	limits    Limits
	dummyHash string
}

func Open(ctx context.Context, cfg Config) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid NEWMMO_DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("cannot initialize application database pool")
	}
	var role string
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.QueryRow(probe, `SELECT current_user`).Scan(&role); err != nil {
		pool.Close()
		return nil, errors.New("application database unavailable")
	}
	if role != "newmmo_app" {
		pool.Close()
		return nil, errors.New("NEWMMO_DATABASE_URL must connect as newmmo_app")
	}
	dummy, err := hashPassword("dummy password verification", passwordIterations)
	if err != nil {
		pool.Close()
		return nil, errors.New("cannot initialize password verification")
	}
	return &Store{pool: pool, limits: cfg.Limits, dummyHash: dummy}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Pool exposes the application DB pool for gameplay packages (quest engine).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) Ready(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Prune removes login history beyond the rate-limit retention window.
func (s *Store) Prune(ctx context.Context) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM public.login_attempts WHERE at < now() - interval '24 hours'`)
	_, _ = s.pool.Exec(ctx, `DELETE FROM public.ws_tickets WHERE created_at < now() - interval '1 hour'`)
}

func (s *Store) Register(ctx context.Context, login, password string, ip netip.Addr) (string, error) {
	var accountID string
	var rejection error
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockSource(ctx, tx, ip); err != nil {
			return err
		}
		registration := ""
		delay, err := retryDelay(ctx, tx, ip, &registration, false, time.Hour, s.limits.RegistrationsPerIP)
		if err != nil {
			return err
		}
		if delay > 0 {
			rejection = &Failure{Code: "RATE_LIMITED", RetryAfterMS: delay}
			return nil
		}
		if err := recordAttempt(ctx, tx, ip, "", true); err != nil {
			return err
		}
		hashed, err := hashPassword(password, passwordIterations)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `INSERT INTO public.accounts (login, password_hash) VALUES ($1, $2)
			ON CONFLICT (login) DO NOTHING RETURNING account_id::text`, login, hashed).Scan(&accountID)
		if errors.Is(err, pgx.ErrNoRows) {
			rejection = ErrLoginTaken
			return nil
		}
		return err
	})
	if err != nil {
		return "", ErrInternal
	}
	return accountID, rejection
}

type Session struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	AccountID   string    `json:"account_id"`
}

func (s *Store) Login(ctx context.Context, login, password string, ip netip.Addr) (Session, error) {
	var result Session
	var rejection error
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockSource(ctx, tx, ip); err != nil {
			return err
		}
		const window = 15 * time.Minute
		pairDelay, err := retryDelay(ctx, tx, ip, &login, true, window, s.limits.LoginPerPair)
		if err != nil {
			return err
		}
		ipDelay, err := retryDelay(ctx, tx, ip, nil, true, window, s.limits.LoginPerIP)
		if err != nil {
			return err
		}
		if delay := max(pairDelay, ipDelay); delay > 0 {
			rejection = &Failure{Code: "RATE_LIMITED", RetryAfterMS: delay}
			return nil
		}

		var oldHash string
		err = tx.QueryRow(ctx, `SELECT account_id::text, password_hash FROM public.accounts WHERE login = $1`,
			login).Scan(&result.AccountID, &oldHash)
		known := err == nil
		if errors.Is(err, pgx.ErrNoRows) {
			oldHash = s.dummyHash
		} else if err != nil {
			return err
		}
		matches, iterations, err := verifyPassword(password, oldHash)
		if err != nil {
			return err
		}
		success := known && matches
		if err := recordAttempt(ctx, tx, ip, login, success); err != nil {
			return err
		}
		if !success {
			if iterations < passwordIterations {
				// Pad legacy failures to the current work factor.
				if _, err := pbkdf2.Key(sha256.New, password, []byte("legacy-auth-padding"), passwordIterations-iterations, 32); err != nil {
					return err
				}
			}
			rejection = ErrInvalidCredentials
			return nil
		}
		if iterations < passwordIterations {
			updated, err := hashPassword(password, passwordIterations)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.accounts SET password_hash = $1
				WHERE account_id = $2 AND password_hash = $3`, updated, result.AccountID, oldHash); err != nil {
				return err
			}
		}

		result.AccessToken, err = newSecret()
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO public.auth_tokens (token_hash, account_id, expires_at)
			VALUES ($1, $2, clock_timestamp() + make_interval(secs => $3))
			RETURNING expires_at`, hashSecret(result.AccessToken), result.AccountID, tokenTTL.Seconds()).Scan(&result.ExpiresAt)
	})
	if err != nil {
		return Session{}, ErrInternal
	}
	if rejection != nil {
		return Session{}, rejection
	}
	result.ExpiresAt = result.ExpiresAt.UTC()
	return result, nil
}

func lockSource(ctx context.Context, tx pgx.Tx, ip netip.Addr) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 18))`, ip.String())
	return err
}

func retryDelay(ctx context.Context, tx pgx.Tx, ip netip.Addr, login *string, failuresOnly bool, window time.Duration, limit int) (int64, error) {
	var delay int64
	err := tx.QueryRow(ctx, `SELECT coalesce((
		SELECT greatest(1, ceil(extract(epoch FROM (at + make_interval(secs => $2) - now())) * 1000))::bigint
		FROM public.login_attempts WHERE ip = $1 AND at > now() - make_interval(secs => $2)
			AND (NOT $3::boolean OR NOT ok) AND ($4::text IS NULL OR login = $4)
		ORDER BY at DESC OFFSET $5 LIMIT 1
	),0)`, ip, window.Seconds(), failuresOnly, login, limit-1).Scan(&delay)
	return delay, err
}

func recordAttempt(ctx context.Context, tx pgx.Tx, ip netip.Addr, login string, success bool) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.login_attempts (login, ip, ok) VALUES ($1, $2, $3)`, login, ip, success)
	return err
}

func (s *Store) Authenticate(ctx context.Context, token string) (string, error) {
	if !ValidSecret(token) {
		return "", ErrUnauthenticated
	}
	var id string
	err := s.pool.QueryRow(ctx, `SELECT account_id::text FROM public.auth_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, hashSecret(token)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	if err != nil {
		return "", ErrInternal
	}
	return id, nil
}

func (s *Store) Logout(ctx context.Context, token string) error {
	if !ValidSecret(token) {
		return ErrUnauthenticated
	}
	tag, err := s.pool.Exec(ctx, `UPDATE public.auth_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, hashSecret(token))
	if err != nil {
		return ErrInternal
	}
	if tag.RowsAffected() != 1 {
		return ErrUnauthenticated
	}
	return nil
}

type Character struct {
	ID      string `json:"character_id"`
	Name    string `json:"name"`
	ClassID string `json:"class_id"`
	Level   int    `json:"level"`
}

func (s *Store) Characters(ctx context.Context, accountID string) ([]Character, error) {
	rows, err := s.pool.Query(ctx, `SELECT character_id, name, class_id, xp FROM public.characters
		WHERE account_id = $1 ORDER BY character_id`, accountID)
	if err != nil {
		return nil, ErrInternal
	}
	characters, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Character, error) {
		var id, xp int64
		var c Character
		err := row.Scan(&id, &c.Name, &c.ClassID, &xp)
		c.ID, c.Level = strconv.FormatInt(id, 10), Level(xp)
		return c, err
	})
	if err != nil {
		return nil, ErrInternal
	}
	return characters, nil
}

func Level(xp int64) int {
	for level := 1; level < 60; level++ {
		next := int64(math.Round(100 * math.Pow(float64(level), 1.6)))
		if xp < next {
			return level
		}
		xp -= next
	}
	return 60
}

func (s *Store) CreateCharacter(ctx context.Context, accountID, name, classID string) (string, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO public.characters
		(account_id, name, name_ci, class_id, flags, content_build_id)
		SELECT $1, $2, $3, $4, '{"TUTORIAL-COMPLETE":true}', build_id FROM public.content_active WHERE id = 1
		RETURNING character_id`, accountID, name, strings.ToLower(name), classID).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "characters_name_ci_key":
			return "", ErrNameTaken
		case "characters_account_limit":
			return "", ErrLimitReached
		}
	}
	if err != nil {
		return "", ErrInternal
	}
	return strconv.FormatInt(id, 10), nil
}

type Ticket struct {
	Value       string `json:"ticket"`
	ExpiresInMS int64  `json:"expires_in_ms"`
}

func (s *Store) IssueTicket(ctx context.Context, accountID string, characterID int64) (Ticket, error) {
	var owner string
	err := s.pool.QueryRow(ctx, `SELECT account_id::text FROM public.characters WHERE character_id = $1`, characterID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrNotFound
	}
	if err != nil {
		return Ticket{}, ErrInternal
	}
	if owner != accountID {
		return Ticket{}, ErrForbidden
	}
	secret, err := newSecret()
	if err != nil {
		return Ticket{}, ErrInternal
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO public.ws_tickets (ticket_hash, account_id, character_id, expires_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))`,
		hashSecret(secret), accountID, characterID, ticketTTL.Seconds())
	if err != nil {
		return Ticket{}, ErrInternal
	}
	return Ticket{Value: secret, ExpiresInMS: ticketTTL.Milliseconds()}, nil
}

// ConsumeTicket claims the ticket once and returns its authoritative binding.
func (s *Store) ConsumeTicket(ctx context.Context, ticket string) (string, int64, error) {
	if !ValidSecret(ticket) {
		return "", 0, ErrInvalidTicket
	}
	var accountID string
	var characterID int64
	err := s.pool.QueryRow(ctx, `UPDATE public.ws_tickets SET used_at = now()
		WHERE ticket_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING account_id::text, character_id`, hashSecret(ticket)).Scan(&accountID, &characterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrInvalidTicket
	}
	if err != nil {
		return "", 0, ErrInternal
	}
	return accountID, characterID, nil
}
