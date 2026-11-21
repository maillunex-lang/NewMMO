-- +goose Up
CREATE TABLE public.accounts (
    account_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login text NOT NULL UNIQUE CHECK (login ~ '^[a-z0-9_]{3,24}$'),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE public.auth_tokens (
    token_hash char(64) PRIMARY KEY CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    account_id uuid NOT NULL REFERENCES public.accounts(account_id),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE public.login_attempts (
    attempt_id bigserial PRIMARY KEY,
    -- Empty login denotes registration traffic and cannot be a login.
    login text NOT NULL CHECK (login = '' OR login ~ '^[a-z0-9_]{3,24}$'),
    ip inet NOT NULL,
    ok boolean NOT NULL,
    at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX login_attempts_ip_time ON public.login_attempts (ip, at);
CREATE INDEX login_attempts_time ON public.login_attempts (at);
CREATE INDEX auth_tokens_account ON public.auth_tokens (account_id);

GRANT SELECT ON public.accounts, public.auth_tokens, public.login_attempts TO newmmo_app;
GRANT INSERT (login, password_hash), UPDATE (password_hash) ON public.accounts TO newmmo_app;
GRANT INSERT (token_hash, account_id, expires_at), UPDATE (revoked_at) ON public.auth_tokens TO newmmo_app;
GRANT INSERT (login, ip, ok), DELETE ON public.login_attempts TO newmmo_app;
GRANT USAGE ON SEQUENCE public.login_attempts_attempt_id_seq TO newmmo_app;

-- +goose Down
DROP TABLE public.login_attempts;
DROP TABLE public.auth_tokens;
DROP TABLE public.accounts;
