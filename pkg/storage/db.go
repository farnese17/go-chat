package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type File struct {
	ID         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string `json:"name" gorm:"type:varchar(50);not null"`
	Path       string `json:"path" gorm:"type:varchar(200);not null"`
	Hash       string `json:"hash" gorm:"type:varchar(100);not null;column:hash;index:idx_hash"`
	UploadedBy uint   `json:"uploaded_by" gorm:"not null;column:uploaded_by"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	DeletedAt  int64  `json:"deleted_at" gorm:"default:null;column:deleted_at"`
}

type FileReference struct {
	ID         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	FileID     uint   `json:"file_id" gorm:"not null;column:file_id;index:idx_file_id"`
	Name       string `json:"name" gorm:"type:varchar(50);not null"`
	UploadedBy uint   `json:"uploaded_by" gorm:"not null;column:uploaded_by"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	DeletedAt  int64  `json:"deleted_at" gorm:"default:null;column:deleted_at"`
}

var (
	ErrFileExisted       = errors.New("file already exist")
	ErrNotFound          = errors.New("not found")
	ErrTimeout           = errors.New("time out")
	ErrUnsupportDataBase = errors.New("unsupport database")
	ErrPermissiondenied  = errors.New("permission denied")
	ErrDuplicateEntry    = errors.New("duplicate entry")
	ErrNotRunning        = errors.New("database not running")
	ErrConnectionReset   = errors.New("redis connection reset")
	ErrConnectionClosed  = errors.New("redis connection closed")
)

const (
	CodeNotFound = 404
)

type DB interface {
	FindFileByHash(hash string) ([]*File, bool, error)
	Get(id string) (*File, error)
	CreateReference(f *FileReference) (uint, error)
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
	case nil, *SqliteOption:
		dialector = sqlite.Open(option.GetDSN())
		db, sql, err := setupSQLDB(dialector, logger)
		if err != nil {
			return nil, err
		}
		return &sqlDB{db, sql}, nil
	case *RedisOption:
		client, err := setupRedis(option, logger)
		if err != nil {
			return nil, err
		}
		return &redisDB{client}, nil
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
func (m *sqlDB) FindFileByHash(hash string) ([]*File, bool, error) {
	var files []*File
	err := m.db.Where("hash = ?", hash).Find(&files).Error
	if err := m.HandleError(err); err != nil {
		return nil, false, err
	}
	if len(files) == 0 {
		return files, false, nil
	}
	return files, true, nil
}

// 创建文件引用
// 这里的引用是指文件的元数据，包含文件的ID、名称和上传者ID等信息
// 这个引用可以用于在数据库中查找文件的详细信息
// 例如，可以通过引用的ID来获取文件的存储路径等信息
// 这个引用可以用于在数据库中查找文件的详细信息
func (m *sqlDB) CreateReference(f *FileReference) (uint, error) {
	err := m.db.Create(f).Error
	return f.ID, m.HandleError(err)
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
		Where("file_reference.id = ? AND file_reference.deleted_at is null", id).
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

const (
	RedisPrefix        = "storage:"
	FileKey            = RedisPrefix + "file:"
	FileIDKey          = RedisPrefix + "file_id"
	FileReferenceKey   = RedisPrefix + "file_reference:"
	FileReferenceIDKey = RedisPrefix + "file_reference_id"
	FileHashKey        = RedisPrefix + "file_hash:"
)

type redisDB struct {
	client *redis.Client
}

func setupRedis(option Option, logger *Logger) (*redis.Client, error) {
	params, ok := option.(*RedisOption)
	if !ok {
		return nil, ErrUnsupportDataBase
	}
	client := redis.NewClient(&redis.Options{
		Addr:     params.Addr,
		Password: params.Password,
		DB:       params.DB,
	})

	err := client.Ping().Err()
	if err != nil {
		logger.logger.Printf("Failed to connect Redis: %v\n", err)
		return nil, err
	}

	return client, nil
}

func (r *redisDB) handleError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	// logger.logger.Printf("Redis error: %v\n", errStr)
	switch {
	case err == redis.Nil:
		return ErrNotFound
	case strings.Contains(errStr, "connect: connection refused"):
		return ErrNotRunning
	case strings.Contains(errStr, "i/o timeout"):
		return ErrTimeout
	case strings.Contains(errStr, "connection reset by peer"):
		return ErrConnectionReset
	case strings.Contains(errStr, "connection closed"):
		return ErrConnectionClosed
	default:
		return err
	}
}

func (r *redisDB) FindFileByHash(hash string) ([]*File, bool, error) {
	script := redis.NewScript(`
		local key = KEYS[1]
		local fileKey = KEYS[2]
		
		local fileIDs = redis.call("SMEMBERS",key)
		local files = {}
		for i = 1, #fileIDs do
			local f = redis.call("GET",fileKey .. fileIDs[i])
			if f then
				table.insert(files,f)
			end
		end
		return files
	`)
	result, err := script.Run(r.client, []string{FileHashKey + hash, FileKey}).Result()
	if err != nil {
		return nil, false, r.handleError(err)
	}
	data, ok := result.([]interface{})
	if !ok {
		return nil, false, errors.New("redis: unexpected result")
	}

	var files []*File
	for _, d := range data {
		var f *File
		if err := json.Unmarshal([]byte(d.(string)), &f); err != nil {
			continue
		}
		files = append(files, f)
	}
	return files, true, nil
}

func (r *redisDB) Get(id string) (*File, error) {
	script := redis.NewScript(`
		local fileKey = KEYS[1]
		local refKey = KEYS[2]
		
		local refData = redis.call("GET",refKey)
		if not refData then 
			return
		end 
		local ref = cjson.decode(refData)
		if ref.deleted_at ~= 0 then 
			return
		end

		local fileData = redis.call("GET",fileKey .. ref.file_id)
		if not fileData then 
			return
		end
		local file = cjson.decode(fileData)
		file.id = ref.id
		file.name = ref.name
		
		local res = cjson.encode(file)
		return res
	`)

	result, err := script.Run(r.client, []string{FileKey, FileReferenceKey + id}).Result()
	if err != nil {
		return nil, r.handleError(err)
	}
	data, ok := result.(string)
	if !ok {
		return nil, errors.New("redis: unexpected result")
	}
	var f *File
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		return nil, err
	}
	return f, nil
}

func (r *redisDB) CreateReference(f *FileReference) (uint, error) {
	script := redis.NewScript(`
		local refIDKey = KEYS[1]
		local refKey = KEYS[2]
		local ref = ARGV[1]
		
		local refID = redis.call("INCR",refIDKey)

		local refData = cjson.decode(ref)
		refData.id = refID
		ref = cjson.encode(refData)

		redis.call("SET",refKey .. refID,ref)
		return refID
	`)

	f.CreatedAt = time.Now().Unix()
	fjson, _ := json.Marshal(f)
	result, err := script.Run(r.client, []string{FileReferenceIDKey, FileReferenceKey}, fjson).Result()
	if err != nil {
		return 0, r.handleError(err)
	}
	id, ok := result.(int64)
	if !ok {
		return 0, errors.New("redis: unexpected result")
	}
	return uint(id), nil
}

func (r *redisDB) SaveFile(f *File) (*FileReference, error) {
	script := redis.NewScript(`
		local fileIDKey = KEYS[1]
		local refIDKey = KEYS[2]
		local fileKey = KEYS[3]
		local refKey = KEYS[4]
		local hashKey = KEYS[5]
		local file = ARGV[1]
		local ref = ARGV[2]

		local fileID = redis.call("INCR",fileIDKey)
		local refID = redis.call("INCR",refIDKey)

		local fileData = cjson.decode(file)
		local refData = cjson.decode(ref)
		local hash = fileData.hash

		fileData.id = fileID
		refData.file_id = fileID
		refData.id = refID

		file = cjson.encode(fileData)
		ref = cjson.encode(refData)

		redis.call("SET",fileKey .. fileID,file)
		redis.call("SET",refKey .. refID,ref)
		redis.call("SADD",hashKey .. hash,fileID)

		return ref
	`)

	f.CreatedAt = time.Now().Unix()
	fileRef := &FileReference{Name: f.Name, UploadedBy: f.UploadedBy, CreatedAt: f.CreatedAt}
	fJson, _ := json.Marshal(f)
	rJson, _ := json.Marshal(fileRef)

	result, err := script.Run(r.client,
		[]string{FileIDKey, FileReferenceIDKey, FileKey, FileReferenceKey, FileHashKey},
		fJson, rJson).Result()
	if err != nil {
		return nil, r.handleError(err)
	}
	res, ok := result.(string)
	if !ok {
		return nil, errors.New("redis: unexpected result")
	}
	if err := json.Unmarshal([]byte(res), fileRef); err != nil {
		return nil, err
	}
	return fileRef, nil
}

func (r *redisDB) Delete(uid uint, fileID string) error {
	script := redis.NewScript(`
		local fileKey = KEYS[1]
		local uploader = tonumber(ARGV[1])

		local file = redis.call("GET",fileKey)
		if not file then
			return 0
		end

		local f = cjson.decode(file)
		if f.uploaded_by ~= uploader then
			return 0
		end

		local deleted = redis.call("DEL",fileKey)
		return deleted
	`)

	key := FileReferenceKey + fileID
	result, err := script.Run(r.client, []string{key}, uid).Result()
	if err != nil {
		return r.handleError(err)
	}

	deleted, ok := result.(int64)
	if !ok {
		return errors.New("redis: unexpected result")
	}
	if deleted == 0 {
		return ErrPermissiondenied
	}
	return nil
}

func (r *redisDB) Close() {
	r.client.Close()
}
