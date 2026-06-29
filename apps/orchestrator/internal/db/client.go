// Authentication uses IAM database auth. The orchestrator generates a
// short-lived (15 min) auth token via the AWS SDK on every new connection
// instead of using a static password. No secret ever lives in an env var.
// Authorization now happens entirely in the orchestrator's Go code (it
// already checks state.UserID != claims.Sub on every request).
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds Aurora connection parameters.
type Config struct {
	Host     string
	Port     string
	Database string
	DBUser   string
	Region   string
}

// Client wraps a pgx connection pool. Connections are re-authenticated
// periodically since IAM tokens expire after 15 minutes
// MaxConnLifetime handles this by recycling connections before expiry.
type Client struct {
	pool *pgxpool.Pool
	cfg  Config
}

// NewClient builds a connection pool authenticated via AWS IAM.
// awsCfg should be loaded via config.LoadDefaultConfig(ctx) in main.go,
// which picks up credentials from the environment, IAM role (when running
// on ECS), or local AWS CLI profile (when running locally).
func NewClient(ctx context.Context, cfg Config, awsCfg aws.Config) (*Client, error) {
	poolCfg, err := pgxpool.ParseConfig(buildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	// Set MaxConnLifetime below the 15-minute IAM token expiry so pgx
	// always recycles connections before their auth token goes stale.
	poolCfg.MaxConnLifetime = 10 * time.Minute
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1

	// BeforeConnect regenerates the IAM auth token for every new physical
	// connection the pool opens. This is what makes password-less auth work
	// transparently with pgx's connection pooling.
	poolCfg.BeforeConnect = func(ctx context.Context, connCfg *pgx.ConnConfig) error {
		token, err := auth.BuildAuthToken(ctx, fmt.Sprintf("%s:%s", cfg.Host, cfg.Port), cfg.Region, cfg.DBUser, awsCfg.Credentials)
		if err != nil {
			return fmt.Errorf("build IAM auth token: %w", err)
		}
		connCfg.Password = token
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connectivity immediately so startup fails fast if Aurora
	// or IAM permissions are misconfigured, rather than failing on first request.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping aurora: %w", err)
	}

	return &Client{pool: pool, cfg: cfg}, nil
}

// Close releases all pooled connections. Call on graceful shutdown.
func (c *Client) Close() {
	c.pool.Close()
}

// buildDSN constructs the connection string. sslmode=require is mandatory —
// Aurora IAM auth will not work over an unencrypted connection.
func buildDSN(cfg Config) string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s sslmode=require",
		cfg.Host, cfg.Port, cfg.Database, cfg.DBUser,
	)
}
