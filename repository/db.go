package repository

import (
	"context"
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

func SetupSQL(dsn string) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		}})
	if err != nil {
		logger.GetLogger().Error("Failed to connect database", zap.Error(err))
		return nil, nil, err
	}
	logger.GetLogger().Info("Connected to database")
	db.AutoMigrate(&model.User{}, &model.Manager{},
		&model.Friend{},
		&model.Group{}, &model.GroupPerson{}, &model.GroupAnnouncement{},
	)
	logger.GetLogger().Info("Database tables migration completed successfully")
	if err := fixAutoIncrement(db); err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.GetLogger().Error("Failed to obtain database underlying instance", zap.Error(err))
		return nil, nil, err
	}
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetConnMaxLifetime(time.Minute * 5)
	logger.GetLogger().Info("Database connection pool configuration",
		zap.Int("MaxIdleConns", 50),
		zap.Int("MaxOpenConns", 200),
		zap.Duration("ConnMaxLifeTime", time.Hour),
	)
	timeoutMiddleware(time.Second * 5)(db)
	return db, sqlDB, nil
}

func fixAutoIncrement(db *gorm.DB) error {
	tableName := []string{"user", "group", "manager"}
	idStartAt := map[string]int{"user": int(1e5 + 1), "group": int(1e9 + 1), "manager": 1001}
	currIDAt := []struct {
		Name           string `gorm:"column:name"`
		Auto_increment int    `gorm:"column:auto_increment"`
	}{}
	q := `SELECT TABLE_NAME AS name,AUTO_INCREMENT AS auto_increment 
			FROM information_schema.` + "`TABLES` " +
		`WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ?`
	if err := db.Raw(q, tableName).Scan(&currIDAt).Error; err != nil {
		logger.GetLogger().Error("Failed to get auto_increment", zap.Error(err))
		return err
	}
	if len(currIDAt) != len(tableName) {
		return fmt.Errorf("Get %d auto_increment,but should has %d", len(currIDAt), len(tableName))
	}
	for _, c := range currIDAt {
		if c.Auto_increment < idStartAt[c.Name] {
			if err := db.Exec(fmt.Sprintf("ALTER TABLE `%s` auto_increment = %d", c.Name, idStartAt[c.Name])).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func timeoutMiddleware(timeout time.Duration) func(*gorm.DB) {
	return func(db *gorm.DB) {
		operations := []string{"create", "query", "update", "delete"}
		for _, op := range operations {
			t := timeout * 2
			processor := db.Callback().Create()
			switch op {
			case "query":
				processor = db.Callback().Query()
				t = timeout
			case "update":
				processor = db.Callback().Update()
			case "delete":
				processor = db.Callback().Delete()
			}
			processor.Before("gorm:"+op).Register("timeout:before_"+op, func(d *gorm.DB) {
				ctx, cancel := context.WithTimeout(context.Background(), t)
				d.Statement.Context = ctx
				d.InstanceSet("cancel", cancel)
			})
			processor.After("gorm:"+op).Register("timeout:after_"+op, func(d *gorm.DB) {
				if cancel, ok := d.InstanceGet("cancel"); ok {
					if cancelFunc, ok := cancel.(context.CancelFunc); ok {
						cancelFunc()
					}
				}
			})
		}
	}
}

func SetupRedis(cfg config.Config) (*redis.Client, error) {
	addr := cfg.Cache().Addr()
	db := cfg.Cache().DBNum()
	redisClient := redis.NewClient(&redis.Options{
		Addr:       addr,
		DB:         db,
		MaxRetries: 5,
	})

	err := redisClient.Ping().Err()
	if err != nil {
		logger.GetLogger().Error("Failed to connect Redis", zap.String("Addr", addr), zap.Error(err))
		return nil, err
	}
	logger.GetLogger().Info("Connected to Redis", zap.String("Addr", addr))
	return redisClient, nil
}
