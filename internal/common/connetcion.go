package common

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/OrtemRepos/shortlink/configs"
)

var db *sqlx.DB

func GetConnection(cfg *configs.Config) *sqlx.DB {
	if db != nil {
		return db
	}
	credential := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname)

	var err error
	db, err = sqlx.Open("pgx", credential)
	if err != nil {
		panic(err)
	}
	return db
}
