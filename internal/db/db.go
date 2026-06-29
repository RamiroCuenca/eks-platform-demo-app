// Package db wraps a pgx connection pool to Aurora PostgreSQL. All connections require
// TLS (sslmode=require) to satisfy the cluster's rds.force_ssl=1 parameter.
package db

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/RamiroCuenca/eks-platform-demo-app/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

// New builds a lazy connection pool. It does not dial the database; the first query
// establishes a connection, so a data-tier outage at boot does not crash the process.
func New(ctx context.Context, cfg config.DBConfig) (*DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(cfg.User), url.QueryEscape(cfg.Password),
		cfg.Host, cfg.Port, cfg.Name, cfg.SSLMode)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	poolCfg.MaxConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Ping verifies connectivity for readiness checks.
func (d *DB) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return d.pool.Ping(ctx)
}

// Version returns the server version string — a trivial read that proves a working
// TLS connection end to end.
func (d *DB) Version(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var v string
	if err := d.pool.QueryRow(ctx, "select version()").Scan(&v); err != nil {
		return "", err
	}
	return v, nil
}

func (d *DB) Close() { d.pool.Close() }
