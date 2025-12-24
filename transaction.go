package pgx

import (
	"context"
	"database/sql"
)

type txKey struct{}

var key txKey

func Inject(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, key, tx)
}

func From(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(key).(*sql.Tx)
	return tx, ok
}

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Exec(db *sql.DB, ctx context.Context) DBTX {
	if tx, ok := From(ctx); ok {
		return tx
	}
	return db
}

func Transaction(db *sql.DB, ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := From(ctx); ok {
		return fn(ctx) // уже внутри tx
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	ctx = Inject(ctx, tx)

	if err = fn(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
