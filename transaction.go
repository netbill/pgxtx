package pgxtx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type poolDBTX struct {
	pool *pgxpool.Pool
}

func (p poolDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, args...)
}

func (p poolDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

func (p poolDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

func Exec(pool *pgxpool.Pool, ctx context.Context) DBTX {
	if tx, ok := From(ctx); ok {
		return tx
	}
	return poolDBTX{pool: pool}
}

func Transaction(pool *pgxpool.Pool, ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := From(ctx); ok {
		return fn(ctx)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
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
