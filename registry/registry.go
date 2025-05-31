package registry

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/pkg/storage"
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
	Storage() storage.Storage

	SetHub(hub websocket.HubInterface)
	Shutdown()

	Uptime() time.Duration
}

func SetupService(configPath string) Service {
	var err error

	service, err = initRegistry(configPath)
	if err != nil {
		panic(err)
	}
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
	storage    storage.Storage

	config    config.Config
	runningAt time.Time
}

func getDSN(cfg config.Config) string {
	dbcfg := cfg.Database()
	// Addr = root:123456@tcp(127.0.0.1:33060)/chat?charset=utf8mb4&parseTime=True&loc=Local
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User(), dbcfg.Password(), dbcfg.Host(), dbcfg.Port(), dbcfg.DBname())
	return dsn
}

func handleDBConnectError(err error) {
	if err != nil {
		fmt.Println("Initial db failed: " + err.Error())
		fmt.Println("Starting retry...")
		time.Sleep(time.Second * 2)
	}
}

func initRegistry(configPath string) (*registry, error) {
	cfgPath := findConfigFile(configPath)
	cfg := config.LoadConfig(cfgPath)
	logger := logger.SetupLogger()
	validator.SetupValidator()

	dsn := getDSN(cfg)
	var db *gorm.DB
	var sqlDB *sql.DB
	var err error
	for {
		db, sqlDB, err = repo.SetupSQL(dsn)
		if err != nil {
			handleDBConnectError(err)
			continue
		}
		break
	}

	var redisClient *redis.Client
	for {
		redisClient, err = repo.SetupRedis(cfg)
		if err != nil {
			handleDBConnectError(err)
			continue
		}
		break
	}

	var fs storage.Storage
	for {
		fs, err = storage.NewLocalStorage(cfg.FileServer().Path(), cfg.FileServer().LogPath(), &storage.MysqlOption{
			User:     cfg.Database().User(),
			Password: cfg.Database().Password(),
			Addr:     cfg.Database().Host(),
			Port:     cfg.Database().Port(),
			DBName:   cfg.Database().DBname(),
		})
		if err != nil {
			handleDBConnectError(err)
			continue
		}
		break
	}

	reg := &registry{
		mu:        sync.RWMutex{},
		db:        db,
		sqlDB:     sqlDB,
		rc:        redisClient,
		logger:    logger,
		config:    cfg,
		storage:   fs,
		runningAt: time.Now(),
	}
	reg.initRepository()
	cache := repo.NewRedisCache(redisClient, reg)
	reg.cache = cache

	wsHub := websocket.NewHubInterface(reg)
	reg.hub = wsHub

	return reg, nil
}

func (r *registry) initRepository() {
	r.userRepo = repo.NewSQLUserRepository(r.db)
	r.friendRepo = repo.NewSQLFriendRepository(r.db)
	r.groupRepo = repo.NewSQLGroupRepository(r.db)
	r.mgrRepo = repo.NewSQLManagerRepository(r.db)
}

func (r *registry) Uptime() time.Duration {
	return time.Since(r.runningAt)
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

func (r *registry) Storage() storage.Storage {
	return r.storage
}

func (r *registry) Shutdown() {
	r.storage.Close()
	r.sqlDB.Close()
	r.cache.Stop()
	r.rc.Close()
	r.logger.Info("Server stop")
	r.logger.Sync()
}

func findConfigFile(path string) string {
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
