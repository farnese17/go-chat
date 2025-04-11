package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type File struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	Name       string `gorm:"type:varchar(50);not null"`
	Path       string `gorm:"type:varchar(200);not null"`
	Hash       string `gorm:"type:varchar(100);not null;column:hash;uniqueIndex:idx_hash"`
	UploadedBy uint   `gorm:"not null;column:uploaded_by;uniqueIndex:idx_hash"`
	CreatedAt  int64  `gorm:"autoCreateTime;column:created_at"`
	// DeletedAt  int64  `gorm:"default:null;column:deleted_at;uniqueIndex:idx_hash"`
}

type FileReference struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	FileID     uint   `gorm:"not null;column:file_id;index:idx_file_id"`
	Name       string `gorm:"type:varchar(50);not null"`
	UploadedBy uint   `gorm:"not null;column:uploaded_by"`
	CreatedAt  int64  `gorm:"autoCreateTime;column:created_at"`
	DeletedAt  int64  `gorm:"default:null;column:deleted_at"`
}

var (
	ErrFileExisted       = errors.New("file already exist")
	ErrNotFound          = errors.New("not found")
	ErrTimeout           = errors.New("time out")
	ErrUnsupportDataBase = errors.New("unsupport database")
	ErrPermissiondenied  = errors.New("permission denied")
	ErrDuplicateEntry    = errors.New("duplicate entry")
)

const (
	CodeNotFound = 404
)

type DB interface {
	FindFileByHash(uploader uint, hash string) (uint, bool, error)
	Get(id string) (*File, error)
	CreateReference(f *FileReference) error
	SaveFile(f *File) (*FileReference, error)
	Delete(uid uint, fileID string) error
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
	err = db.AutoMigrate(&File{}, &FileReference{})
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
	switch err {
	case gorm.ErrRecordNotFound:
		return ErrNotFound
	case gorm.ErrDuplicatedKey:
		return ErrDuplicateEntry
	case context.DeadlineExceeded:
		return ErrTimeout
	}
	return err
}

// 检查文件是否已存在
func (m *sqlDB) FindFileByHash(uploader uint, hash string) (uint, bool, error) {
	var file File
	err := m.db.Where("hash = ? AND uploaded_by = ?", hash, uploader).First(&file).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err := m.HandleError(err); err != nil {
		return 0, false, err
	}
	return file.ID, true, nil
}

// 创建文件引用
// 这里的引用是指文件的元数据，包含文件的ID、名称和上传者ID等信息
// 这个引用可以用于在数据库中查找文件的详细信息
// 例如，可以通过引用的ID来获取文件的存储路径等信息
// 这个引用可以用于在数据库中查找文件的详细信息
func (m *sqlDB) CreateReference(f *FileReference) error {
	err := m.db.Create(f).Error
	return m.HandleError(err)
}

// 保存文件路径
func (m *sqlDB) SaveFile(f *File) (*FileReference, error) {
	fileRef := &FileReference{FileID: f.ID, Name: f.Name, UploadedBy: f.UploadedBy}
	err := m.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(f).Error; err != nil {
			return err
		}

		fileRef.FileID = f.ID
		if err := tx.Create(fileRef).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil && strings.Contains(err.Error(), "Duplicate entry") {
		return nil, ErrDuplicateEntry
	}
	return fileRef, m.HandleError(err)
}

// 获取文件
func (m *sqlDB) Get(id string) (*File, error) {
	var file *File
	err := m.db.Model(&FileReference{}).
		Select("file_reference.file_id AS id,file_reference.name,f.path,f.hash").
		Joins("LEFT JOIN file AS f ON f.id = file_reference.file_id").
		Where("file_reference.id = ? AND deleted_at is null", id).
		First(&file).Error
	if err := m.HandleError(err); err != nil {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return file, nil
}

func (m *sqlDB) Delete(uid uint, fileID string) error {
	q := `UPDATE file_reference SET deleted_at = ? WHERE id = ? AND uploaded_by = ?`
	result := m.db.Exec(q, time.Now().Unix(), fileID, uid)
	if err := m.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return ErrPermissiondenied
	}
	return nil
}

func (m *sqlDB) Close() {
	m.sqlDB.Close()
}
