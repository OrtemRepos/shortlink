package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/common"
	"github.com/OrtemRepos/shortlink/internal/domain"
	"github.com/OrtemRepos/shortlink/internal/logger"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var Log = logger.GetLogger()

type PostgreRepository struct {
	Database *sqlx.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS urls (
	id SERIAL PRIMARY KEY,
	long_url TEXT NOT NULL UNIQUE,
	short_url TEXT NOT NULL UNIQUE
);`

func NewPostgreRepository(cfg *configs.Config) *PostgreRepository {
	db := common.GetConnection(cfg)

	ctx, cancel := createContextWithTimeout()
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		Log.Panic("PostgreRepository: failed to ping database", zap.Error(err))
	}
	checkExistsTable(db)
	return &PostgreRepository{
		Database: db,
	}
}

func (p *PostgreRepository) Close() error {
	return p.Database.Close()
}

func (p *PostgreRepository) Ping() error {
	ctx, cancel := createContextWithTimeout()
	defer cancel()
	return p.Database.PingContext(ctx)
}

func checkExistsTable(db *sqlx.DB) {
	ctx, cancel := createContextWithTimeout()
	defer cancel()
	db.MustExecContext(ctx, schema)

	db.MustExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_short_url ON urls (short_url);")
}

func (p *PostgreRepository) Find(shortURL string) (*domain.URL, error) {
	var url domain.URL
	ctx, cancel := createContextWithTimeout()
	defer cancel()
	err := p.Database.GetContext(ctx, &url, "SELECT id, long_url, short_url FROM urls WHERE short_url = $1", shortURL)
	if err != nil {
		Log.Error("Error in find url", zap.Any("URL", url), zap.Error(err))
		return nil, err
	}
	Log.Info("Find in storage", zap.Any("url", url))
	return &url, nil
}

func (p *PostgreRepository) Save(url *domain.URL) error {
	ctx, cancel := createContextWithTimeout()
	defer cancel()

	tx := p.Database.MustBeginTx(ctx, nil)

	defer func() { _ = tx.Rollback() }()

	err := p.save(ctx, tx, url)
	if errors.Is(err, domain.ErrURLAlreadyExists) {
		return err
	} else if err != nil {
		return fmt.Errorf("unable to save URL: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("unable to commit transaction: %w", err)
	}

	return nil
}
func (p *PostgreRepository) save(ctx context.Context, tx *sqlx.Tx, url *domain.URL) error {
	url.GenerateShortURL()
	stmt, err := tx.PreparexContext(
		ctx,
		"INSERT INTO urls (long_url, short_url) VALUES ($1, $2) ON CONFLICT (long_url) DO NOTHING RETURNING id, short_url;",
	)
	if err != nil {
		return fmt.Errorf("unable to prepare statement: %w", err)
	}
	defer stmt.Close()
	var id int
	var shortURL string
	err = stmt.QueryRowxContext(ctx, url.LongURL, url.ShortURL).Scan(&id, &shortURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.GetContext(ctx, url, "SELECT id, long_url, short_url FROM urls WHERE long_url = $1", url.LongURL)
			if err != nil {
				return fmt.Errorf("failed to get existing URL: %w", err)
			}
			return domain.ErrURLAlreadyExists
		}
		return fmt.Errorf("query row error: %w", err)
	}
	url.ID = id
	url.ShortURL = shortURL
	if err != nil {
		return fmt.Errorf("failed to save user link: %w", err)
	}
	return nil
}

func (p *PostgreRepository) BatchSave(urls []*domain.URL) error {
	ctx, cancel := createContextWithTimeout()
	defer cancel()
	tx := p.Database.MustBeginTx(ctx, nil)

	defer func() { _ = tx.Rollback() }()

	for _, url := range urls {
		err := p.save(ctx, tx, url)

		if !errors.Is(err, domain.ErrURLAlreadyExists) && err != nil {
			return fmt.Errorf("unable to save URL: %w", err)
		}
	}
	return tx.Commit()
}

func createContextWithTimeout() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	return ctx, cancel
}
