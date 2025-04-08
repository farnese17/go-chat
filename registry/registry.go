package registry

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/farnese17/chat/config"
	repo "github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/utils/logger"
	"github.com/farnese17/chat/utils/validator"
	"github.com/farnese17/chat/websocket"
	"github.com/go-redis/redis"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var service Service

type Service interface {
	Config() config.Config
	Logger() *zap.Logger
	User() repo.UserRepository
	Friend() repo.FriendRepository
	Group() repo.GroupRepository
	Manager() repo.Manager
	Cache() repo.Cache
	Hub() websocket.HubInterface

	SetHub(hub websocket.HubInterface)
	Shutdown()
}

func SetupService() Service {
	service = initRegistry()
	return service
}

func GetService() Service {
	return service
}

type registry struct {
	mu     sync.RWMutex
	db     *gorm.DB
	sqlDB  *sql.DB
	rc     *redis.Client
	logger *zap.Logger

	userRepo   repo.UserRepository
	friendRepo repo.FriendRepository
	groupRepo  repo.GroupRepository
	mgrRepo    repo.Manager
	cache      repo.Cache
	hub        websocket.HubInterface

	config config.Config
}

func initRegistry() *registry {
	cfgPath := findConfigFile()
	cfg := config.LoadConfig(cfgPath)
	logger := logger.SetupLogger()
	validator.SetupValidator()

	// Addr = root:123456@tcp(127.0.0.1:33060)/chat?charset=utf8mb4&parseTime=True&loc=Local
	dbcfg := cfg.Database()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User(), dbcfg.Password(), dbcfg.Host(), dbcfg.Port(), dbcfg.DBname())
	db, sqlDB := repo.SetupSQL(dsn)
	redisClient := repo.SetupRedis(cfg)

	userRepo := repo.NewSQLUserRepository(db)
	friendRepo := repo.NewSQLFriendRepository(db)
	groupRepo := repo.NewSQLGroupRepository(db)
	mgrRepo := repo.NewSQLManagerRepository(db)

	reg := &registry{
		mu:         sync.RWMutex{},
		db:         db,
		sqlDB:      sqlDB,
		rc:         redisClient,
		logger:     logger,
		userRepo:   userRepo,
		friendRepo: friendRepo,
		groupRepo:  groupRepo,
		mgrRepo:    mgrRepo,
		config:     cfg,
	}
	cache := repo.NewRedisCache(redisClient, reg)
	reg.cache = cache

	wsHub := websocket.NewHubInterface(reg)
	reg.hub = wsHub
	return reg
}
func (r *registry) Config() config.Config {
	return r.config
}
func (r *registry) Logger() *zap.Logger {
	return r.logger
}
func (r *registry) User() repo.UserRepository {
	return r.userRepo
}

func (r *registry) Friend() repo.FriendRepository {
	return r.friendRepo
}
func (r *registry) Group() repo.GroupRepository {
	return r.groupRepo
}
func (r *registry) Manager() repo.Manager {
	return r.mgrRepo
}
func (r *registry) Cache() repo.Cache {
	return r.cache
}

func (r *registry) Hub() websocket.HubInterface {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hub
}
func (r *registry) SetHub(hub websocket.HubInterface) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hub = hub
}

func (r *registry) Shutdown() {
	r.sqlDB.Close()
	r.cache.Stop()
	r.rc.Close()
	r.logger.Info("Server stop")
	r.logger.Sync()
}

func findConfigFile() string {
	var path string
	flag.StringVar(&path, "config", "", "configuration file path")
	flag.Parse()
	if path != "" {
		return path
	}

	path = os.Getenv("CHAT_CONFIG")
	if path != "" {
		return path
	}

	searchPaths := []string{
		"./config/config.yaml",
		"/etc/chat/config.yaml",
		fmt.Sprintf("%s/chat/config/config.yaml", os.Getenv("HOME")),
		"/usr/local/chat/config/config.yaml",
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return searchPaths[0]
}
