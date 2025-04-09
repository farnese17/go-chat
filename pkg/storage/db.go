package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type File struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Name      string `gorm:"type:varchar(100);not null;unique"`
	Path      string `gorm:"type:varchar(200);not null"`
	DeletedAt int64  `gorm:"default:null;column:deleted_at"`
}

var (
	ErrFileExisted       = errors.New("file already exist")
	ErrNotFound          = errors.New("not found")
	ErrTimeout           = errors.New("time out")
	ErrUnsupportDataBase = errors.New("unsupport database")
)

const (
	CodeNotFound = 404
)

type DB interface {
	IsExist(name, path string) error
	Get(id string) (*File, error)
	SaveFilePath(f *File) error
	DeleteFile(id string) error
	Close()
}

func SetupDB(option Option, logger *Logger) (DB, error) {
	var dialector gorm.Dialector
	switch option.(type) {
	case *MysqlOption:
		dialector = mysql.Open(option.GetDSN())
		db, sql, err := setupSQLDB(dialector, logger)
		if err != nil {
			return nil, err
		}
		return &sqlDB{db, sql}, nil
	case *SqliteOption:
		dialector = sqlite.Open(option.GetDSN())
		db, sql, err := setupSQLDB(dialector, logger)
		if err != nil {
			return nil, err
		}
		return &sqlDB{db, sql}, nil
	default:
		return nil, ErrUnsupportDataBase
	}
}

type sqlDB struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func setupSQLDB(dialector gorm.Dialector, logger *Logger) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(dialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		}})
	if err != nil {
		logger.logger.Printf("Failed to connect database: %v\n", err)
		return nil, nil, err
	}
	logger.logger.Println("Connected to database")
	err = db.AutoMigrate(&File{})
	if err != nil && !strings.Contains(err.Error(), "Duplicate key name") {
		return nil, nil, err
	}
	logger.logger.Println("Database tables migration completed successfully")

	sqlDB, err := db.DB()
	if err != nil {
		logger.logger.Printf("Failed to obtain database underlying instance: %v\n", err)
		return nil, nil, err
	}
	sqlDB.SetMaxIdleConns(50)
	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetConnMaxLifetime(time.Hour)
	logger.logger.Printf("Database connection pool configuration, MaxIdleConns:%d, MaxOpenConns:%d, ConnMaxLifeTime:%d", 50, 200, time.Hour)
	return db, sqlDB, nil
}

func (m *sqlDB) HandleError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	return err
}

// 检查文件是否已存在
func (m *sqlDB) IsExist(name, path string) error {
	var file File
	err := m.db.Where("name = ? AND deleted_at is null", name).First(&file).Error
	if err := m.HandleError(err); err != nil {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if file.Path == path {
		return ErrFileExisted
	}
	return nil
}

// 保存文件路径
func (m *sqlDB) SaveFilePath(f *File) error {
	err := m.db.Create(f).Error
	if err != nil {
		return err
	}
	return nil
}

// 获取文件
func (m *sqlDB) Get(id string) (*File, error) {
	var file *File
	err := m.db.Model(&File{}).Where("id = ? AND deleted_at is null", id).
		First(&file).Error
	fmt.Println(file)
	if err := m.HandleError(err); err != nil {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return file, nil
}

func (m *sqlDB) DeleteFile(id string) error {
	q := `UPDATE file SET deleted_at = ? WHERE id = ?`
	err := m.db.Exec(q, time.Now().Unix(), id).Error
	if err := m.HandleError(err); err != nil {
		return err
	}
	return nil
}

func (m *sqlDB) Close() {
	m.sqlDB.Close()
}
