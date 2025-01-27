package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/auth"
	"github.com/OrtemRepos/shortlink/internal/common"
	"github.com/OrtemRepos/shortlink/internal/domain"
	"github.com/OrtemRepos/shortlink/internal/logger"
	"github.com/OrtemRepos/shortlink/internal/middleware"
	"github.com/OrtemRepos/shortlink/internal/ports"
	"github.com/OrtemRepos/shortlink/internal/task"
	"github.com/OrtemRepos/shortlink/internal/worker"

	"github.com/gin-gonic/gin"
)

type RestAPI struct {
	cfg           *configs.Config
	workerPool    worker.WorkerPool
	tokenProvider ports.PortJWT
	repo          ports.URLRepositoryPort
	deleteChan    chan map[string][]string
	log           *zap.Logger
	*gin.Engine
}

func NewRestAPI(repo ports.URLRepositoryPort,
	engine *gin.Engine, cfg *configs.Config,
) *RestAPI {
	log := logger.GetLogger()
	tokenProvider := NewProviderJWT(cfg)
	workerPool := worker.NewWorkerPool(
		"deleteWorker",
		cfg.Worker.WorkersCount,
		cfg.Worker.BufferSize,
		cfg.Worker.ErrMaximumAmount,
		worker.NewPoolMetrics(),
		worker.NewWorkerMetrics,
	)
	return &RestAPI{
		repo:          repo,
		tokenProvider: tokenProvider,
		workerPool:    workerPool,
		Engine:        engine,
		log:           log,
		cfg:           cfg,
	}
}

const cookieExpTime = 3 * time.Hour

func (r *RestAPI) Serve() {
	r.workerPool.Start(context.TODO())

	timeout := time.Second

	deleteTask := task.NewBatcherDeleteTask(
		r.deleteChan,
		r.repo,
		r.cfg.Worker.BufferSize,
		timeout,
	)

	for i := 0; i < r.cfg.Worker.WorkersCount; i++ {
		_ = r.workerPool.Submit(context.TODO(), deleteTask)
	}

	saveMiddleware := middleware.SaveUserLinkMiddleware(
		common.GetConnection(r.cfg),
	)

	protectedRouters := r.Group("/api")
	protectedRouters.Use(auth.AuthMiddleware(r.tokenProvider))
	protectedRouters.POST("/shorten",
		r.JSONShortURL,
		saveMiddleware,
	)
	protectedRouters.POST("/batch_shorten",
		r.BatchShortURL,
		saveMiddleware,
	)
	protectedRouters.GET("/:shortURL", r.GetLongURL)
	protectedRouters.GET("/user/urls", r.GetAllUserLinks)

	authRouter := r.Group("/")
	authRouter.POST("login", r.Auth)
	r.GET("/ping", r.Ping)
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "404 Not Found",
			"message": "The requested resource was not found on this server.",
		})
	})
	if err := r.Run(r.cfg.Server.Address); err != nil {
		log.Fatal(err)
	}
}

func (r *RestAPI) GetLongURL(c *gin.Context) {
	shortURL := c.Param("shortURL")
	url, err := r.repo.Find(context.TODO(), shortURL)
	if err == domain.ErrURLNotFound {
		c.String(http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Redirect(http.StatusMovedPermanently, url.LongURL)
}

func (r *RestAPI) Ping(c *gin.Context) {
	err := r.repo.Ping(context.TODO())
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
	c.String(http.StatusOK, "OK")
}

func (r *RestAPI) JSONShortURL(c *gin.Context) {
	result := c.GetStringMap("result")
	if result == nil {
		result = make(map[string]interface{})
	}
	status := http.StatusCreated
	c.Header("Content-Type", "application/json")
	var url domain.URL
	if err := json.NewDecoder(c.Request.Body).Decode(&url); err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if url.LongURL == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest,
			gin.H{
				"error":   "400 Bad Request",
				"message": "The request body is empty or malformed.",
			},
		)
		return
	}
	if err := r.repo.Save(context.TODO(), &url); errors.Is(err, domain.ErrURLAlreadyExists) {
		status = http.StatusConflict
	} else if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set("urls", []*domain.URL{&url})
	result["result"] = fmt.Sprintf("%s/%s", r.cfg.Server.BaseAddress, url.ShortURL)
	c.Set("result", result)
	c.Next()
	c.JSON(status, result)
}

func (r *RestAPI) BatchShortURL(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	result := c.GetStringMap("result")
	if result == nil {
		result = make(map[string]any)
	}
	var urlsToShorten map[string]string
	if err := c.ShouldBindJSON(&urlsToShorten); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	if len(urlsToShorten) < 1 {
		c.String(http.StatusBadRequest, "Urls not found")
		return
	}

	urlsToSave := make([]*domain.URL, 0, len(urlsToShorten))
	for _, longURL := range urlsToShorten {
		url := domain.NewURL(longURL)
		urlsToSave = append(urlsToSave, url)
	}
	c.Set("urls", urlsToSave)
	if err := r.repo.BatchSave(context.TODO(), urlsToSave); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	i := 0
	for key := range urlsToShorten {
		result[key] = *urlsToSave[i]
		i++
	}
	c.Next()
	c.Set("result", result)
	c.JSON(http.StatusCreated, result)
}

func (r *RestAPI) Auth(c *gin.Context) {
	tokenString, err := c.Cookie("auth")
	if err == nil && tokenString != "" {
		claims, errCheck := auth.CheckToken(tokenString, r.tokenProvider)
		if errCheck == nil {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"UserID": claims.UserID, "msg": "You alredy login!"})
			return
		}
		r.log.Info("Token err")
	}
	userID := uuid.NewString()
	tokenString, err = r.tokenProvider.BuildJWTString(userID)
	if err != nil {
		r.log.Info("LoginMeddleware error", zap.Error(err))
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Set("UserID", userID)
	c.SetCookie("auth", tokenString, int(cookieExpTime), "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"UserID": userID})
}

func (r *RestAPI) GetAllUserLinks(c *gin.Context) {
	userID := c.GetString("UserID")
	result := c.GetStringMap("result")
	if result == nil {
		result = make(map[string]interface{})
	}

	db := common.GetConnection(r.cfg)
	query := `
    SELECT urls.id, urls.long_url, urls.short_url
    FROM user_links
    JOIN urls ON user_links.url_id = urls.id
    WHERE user_links.user_id = $1;
    `
	rows, err := db.Queryx(query, userID)
	if err != nil {
		r.log.Error("GetAllUserLinks error", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user links"})
		return
	}
	defer rows.Close()
	var urls []domain.URL
	for rows.Next() {
		var url domain.URL
		err := rows.StructScan(&url)
		url.ShortURL = r.cfg.Server.BaseAddress + url.ShortURL
		if err != nil {
			r.log.Error("GetAllUserLinks error", zap.Error(err))
			continue
		}
		urls = append(urls, url)
	}
	if len(urls) == 0 {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	result["urls"] = urls
	c.Set("result", result)
	c.JSON(http.StatusOK, result)
}

func (r *RestAPI) DeleteLink(c *gin.Context) {
	userID := c.GetString("UserID")
	linkIDs := c.QueryArray("linkIDs")
	request := map[string][]string{
		userID: linkIDs,
	}

	select {
	case r.deleteChan <- request:
		c.JSON(http.StatusAccepted, gin.H{"message": "Link deletion initiated"})
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests, please try again later"})
	}
}
