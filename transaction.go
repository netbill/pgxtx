package pgdbx

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type txKey struct{}

var key txKey

func Inject(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, key, tx)
}

func From(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(key).(pgx.Tx)
	return tx, ok
}

func (db *DB) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := From(ctx); ok {
		return fn(ctx)
	}

	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	ctx = Inject(ctx, tx)

	if err = fn(ctx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	committed = true
	return nil
}
