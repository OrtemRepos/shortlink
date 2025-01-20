package app

import (
	"github.com/gin-gonic/gin"

	"github.com/OrtemRepos/shortlink/internal/gzip"
	log "github.com/OrtemRepos/shortlink/internal/logger"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/adapters"
	"github.com/OrtemRepos/shortlink/internal/ports"
)

func run(restAPI ports.RestAPIPort) {
	restAPI.Serve()
}

func Run(cfg *configs.Config) {
	logger, err := log.InitLogger()
	if err != nil {
		panic(err)
	}
	defer func() {
		if errSync := logger.Sync(); errSync != nil {
			logger.Error(errSync.Error())
		}
	}()
	var repository ports.URLRepositoryPort
	if cfg.UseDataBase() {
		repository = adapters.NewPostgreRepository(cfg)
	} else {
		repository, err = adapters.NewInMemoryURLRepository(cfg.Repository.SavePath)
		if err != nil {
			logger.Error(err.Error())
		}
	}

	restAPI := adapters.NewRestAPI(repository, gin.Default(), cfg)
	restAPI.Engine.Use(gzip.GzipMiddleware())
	restAPI.Engine.Use(log.LoggerMiddleware(logger))
	run(restAPI)
}
