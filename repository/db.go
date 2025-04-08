package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/logger"
	"github.com/go-redis/redis"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Service interface {
	Config() config.Config
	Logger() *zap.Logger
	User() UserRepository
	Group() GroupRepository
	Manager() Manager
	Cache() Cache
}

func SetupSQL(dsn string) (*gorm.DB, *sql.DB) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		}})
	if err != nil {
		logger.GetLogger().Fatal("Failed to connect database", zap.Error(err))
	}
	logger.GetLogger().Info("Connected to database")
	db.AutoMigrate(&model.User{}, &model.Manager{},
		&model.Friend{},
		&model.Group{}, &model.GroupPerson{}, &model.GroupAnnouncement{},
	)
	logger.GetLogger().Info("Database tables migration completed successfully")
	fixAutoIncrement(db)

	sqlDB, err := db.DB()
	if err != nil {
		logger.GetLogger().Fatal("Failed to obtain database underlying instance", zap.Error(err))
	}
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetConnMaxLifetime(time.Hour)
	logger.GetLogger().Info("Database connection pool configuration",
		zap.Int("MaxIdleConns", 50),
		zap.Int("MaxOpenConns", 200),
		zap.Duration("ConnMaxLifeTime", time.Hour),
	)
	return db, sqlDB
}

func fixAutoIncrement(db *gorm.DB) {
	tableName := []string{"user", "group", "manager"}
	idStartAt := map[string]int{"user": int(1e5 + 1), "group": int(1e9 + 1), "manager": 1001}
	currIDAt := []struct {
		Name           string `gorm:"column:name"`
		Auto_increment int    `gorm:"column:auto_increment"`
	}{}
	q := `SELECT TABLE_NAME AS name,AUTO_INCREMENT AS auto_increment 
			FROM information_schema.` + "`TABLES` " +
		`WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ?`
	if err := db.Debug().Raw(q, tableName).Scan(&currIDAt).Error; err != nil {
		logger.GetLogger().Panic("Failed to get auto_increment", zap.Error(err))
		panic(err)
	}
	if len(currIDAt) != len(tableName) {
		panic(fmt.Sprintf("Get %d auto_increment,but should has %d", len(currIDAt), len(tableName)))
	}
	for _, c := range currIDAt {
		if c.Auto_increment < idStartAt[c.Name] {
			if err := db.Exec(fmt.Sprintf("ALTER TABLE `%s` auto_increment = %d", c.Name, idStartAt[c.Name])).Error; err != nil {
				panic(err)
			}
		}
	}
}

func SetupRedis(cfg config.Config) *redis.Client {
	addr := cfg.Cache().Addr()
	db := cfg.Cache().DBNum()
	redisClient := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   db,
	})

	err := redisClient.Ping().Err()
	if err != nil {
		logger.GetLogger().Fatal("Failed to connect Redis", zap.String("Addr", addr), zap.Error(err))
	}
	logger.GetLogger().Info("Connected to Redis", zap.String("Addr", addr))
	return redisClient
}
