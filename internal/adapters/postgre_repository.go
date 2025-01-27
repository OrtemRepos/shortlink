package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/common"
	"github.com/OrtemRepos/shortlink/internal/domain"
	"github.com/OrtemRepos/shortlink/internal/logger"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgreRepository struct {
	Database *sqlx.DB
	log      *zap.Logger
}

const schema = `
CREATE TABLE IF NOT EXISTS urls (
	id SERIAL PRIMARY KEY,
	link_id  UUID NOT NULL UNIQUE,
	long_url TEXT NOT NULL UNIQUE,
	short_url TEXT NOT NULL UNIQUE
	is_deleted BOOLEAN DEFAULT FALSE
);`

func NewPostgreRepository(ctx context.Context, cfg *configs.Config) *PostgreRepository {
	db := common.GetConnection(cfg)
	log := logger.GetLogger()
	if err := db.PingContext(ctx); err != nil {
		log.Panic("PostgreRepository: failed to ping database", zap.Error(err))
	}
	checkExistsTable(ctx, db)
	return &PostgreRepository{
		Database: db,
		log:      log,
	}
}

func (p *PostgreRepository) Close() error {
	return p.Database.Close()
}

func (p *PostgreRepository) Ping(ctx context.Context) error {
	return p.Database.PingContext(ctx)
}

func checkExistsTable(ctx context.Context, db *sqlx.DB) {
	db.MustExecContext(ctx, schema)

	db.MustExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_short_url ON urls (short_url);")
}

func (p *PostgreRepository) Find(ctx context.Context, shortURL string) (*domain.URL, error) {
	var url domain.URL
	err := p.Database.GetContext(ctx, &url, "SELECT id, long_url, short_url FROM urls WHERE short_url = $1", shortURL)
	if err != nil {
		p.log.Error("Error in find url", zap.Any("URL", url), zap.Error(err))
		return nil, err
	}
	p.log.Info("Find in storage", zap.Any("url", url))
	return &url, nil
}

func (p *PostgreRepository) Save(ctx context.Context, url *domain.URL) error {
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
	return nil
}

func (p *PostgreRepository) BatchSave(ctx context.Context, urls []*domain.URL) error {
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

func (p *PostgreRepository) delete(ctx context.Context, tx *sqlx.Tx, userID, linkID string) error {
	stmt, err := tx.PrepareContext(ctx, "UPDATE urls SET is_deleted = true WHERE id = $1 AND link_id = $2;")
	if err != nil {
		p.log.Error("failed to prepare delete statement", zap.Error(err))
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(ctx, userID, linkID)
	if err != nil {
		p.log.Error("failed to delete URL", zap.Error(err))
		return fmt.Errorf("failed to delete URL: %w", err)
	}
	return nil
}

func (p *PostgreRepository) BatchDelete(ctx context.Context, ids map[string][]string) error {
	tx := p.Database.MustBeginTx(ctx, nil)
	errs := make([]error, 0, len(ids))
	defer func() { _ = tx.Rollback() }()
	for userID, linkIDs := range ids {
		for _, linkID := range linkIDs {
			err := p.delete(ctx, tx, userID, linkID)
			if err != nil {
				p.log.Error("failed to delete URL", zap.Error(err), zap.String("user_id", userID), zap.String("link_id", linkID))
				errs = append(errs, fmt.Errorf("unable to delete URL: %w", err))
			}
		}
	}
	errs = append(errs, tx.Commit())
	err := errors.Join(errs...)
	return err
}
