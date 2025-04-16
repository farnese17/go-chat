package storage

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUploadFailed = errors.New("upload failed")
)

type Storage interface {
	Upload(uploader uint, file multipart.File, filename string) (uint, error)
	Download(id string) (*File, error)
	Delete(uid uint, fileID string) error
}

type IDGenerator interface {
	NewID() string
}

type DefaultIDGenerator struct{}

func (*DefaultIDGenerator) NewID() string {
	return uuid.NewString()
}

type LocalStorage struct {
	Path   string
	DB     DB
	Logger *Logger
	IDGen  IDGenerator
}

// option为nil,默认使用sqlite
func NewLocalStorage(fileDir, logDir string, option Option) (Storage, error) {
	fileDir = filepath.Clean(fileDir)
	logger, err := SetupLogger(logDir)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	db, err := SetupDB(option, logger)
	if err != nil {
		log.Printf("Failed to connection to DB: %v\n", err)
		return nil, err
	}
	ls := &LocalStorage{
		Path:   fileDir,
		DB:     db,
		Logger: logger,
		IDGen:  &DefaultIDGenerator{},
	}
	log.Println("Success connection to DB")
	return ls, nil
}

func (ls *LocalStorage) Upload(uploader uint, file multipart.File, filename string) (uint, error) {
	defer file.Close()
	// 创建目录
	t := time.Now().Format("20060102")
	ext := filepath.Ext(filename)
	dir := filepath.Join(ls.Path, t)
	if len(ext) > 1 {
		dir = filepath.Join(dir, ext[1:])
	} else {
		dir = filepath.Join(dir, "unknown")
	}
	// ..../files/date/type/file
	if err := os.MkdirAll(dir, 0740); err != nil {
		return 0, err
	}

	// 创建文件,计算hash值
	diskName := ls.addExt(ls.IDGen.NewID(), ext)
	filePath := filepath.Join(dir, diskName)
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), file); err != nil {
		os.Remove(filePath)
		return 0, err
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	// 检查文件是否存在,存在则创建引用直接返回
	id, created, err := ls.checkAndCreateReference(uploader, hash, filename, f)
	if err != nil {
		os.Remove(filePath)
		return 0, err
	}
	if created {
		os.Remove(filePath)
		return id, nil
	}

	// 保存文件路径
	saveFile := &File{Name: filename, Path: filePath, Hash: hash, UploadedBy: uploader}
	var fileID uint
	if fileRef, err := ls.DB.SaveFile(saveFile); err != nil {
		os.Remove(filePath)
		return 0, ErrUploadFailed
	} else {
		fileID = fileRef.ID
	}

	return fileID, nil
}

// view or download
func (ls *LocalStorage) Download(id string) (*File, error) {
	return ls.DB.Get(id)
}

func (ls *LocalStorage) Delete(uid uint, fileID string) error {
	return ls.DB.Delete(uid, fileID)
}

func (ls *LocalStorage) addExt(name, ext string) string {
	if len(ext) > 1 {
		return name + ext
	}
	return name
}

// 检查文件是否存在
// 如果不存在，返回false和nil,表示没有插入引用表
// 如果存在，插入引用表
// 返回true和引用ID，表示插入引用表成功
func (ls *LocalStorage) checkAndCreateReference(uploader uint, hash string, filename string, newFile *os.File) (uint, bool, error) {
	files, exist, err := ls.DB.FindFileByHash(hash)
	if err != nil {
		return 0, false, err
	}
	if !exist {
		return 0, false, nil
	}

	// hash值相同，对比文件
	f, same := ls.compareFile(newFile, files...)
	if !same {
		return 0, false, nil
	}

	fileRef := &FileReference{
		FileID:     f.ID,
		Name:       filename,
		UploadedBy: uploader,
	}
	id, err := ls.DB.CreateReference(fileRef)
	if err != nil {
		return 0, false, err
	}

	return id, true, nil
}

func (ls *LocalStorage) compareFile(newFile *os.File, oldFilesPath ...*File) (*File, bool) {
	newFileInfo, _ := newFile.Stat()
	newFileSize := newFileInfo.Size()

	for _, f := range oldFilesPath {
		oldFile, err := os.Open(f.Path)
		if err != nil {
			continue
		}
		defer oldFile.Close()

		oldFileInfo, _ := oldFile.Stat()
		oldFileSize := oldFileInfo.Size()
		if newFileSize != oldFileSize {
			continue
		}

		newFile.Seek(0, 0)
		oldFile.Seek(0, 0)
		buffer1 := make([]byte, 8196)
		buffer2 := make([]byte, 8196)
		for {
			n1, err1 := newFile.Read(buffer1)
			n2, err2 := oldFile.Read(buffer2)
			if n1 != n2 || string(buffer1[:n1]) != string(buffer2[:n2]) {
				break
			}
			if err1 == io.EOF && err2 == io.EOF {
				return f, true
			}
			if err1 != nil || err2 != nil {
				break
			}
		}
	}
	return nil, false
}

// hash.sha256作为文件名
func (ls *LocalStorage) HashFile(file io.Reader) (string, error) {
	io.MultiWriter()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%x", h.Sum(nil))
	return name, nil
}

func (ls *LocalStorage) Stop() {
	ls.DB.Close()
	ls.Logger.Close()
}

// import (
//  "context"
//  "mime/multipart"

//  "github.com/farnese17/chat/utils"
//  "github.com/farnese17/chat/utils/errors"
//  "github.com/qiniu/go-sdk/v7/auth"
//  "github.com/qiniu/go-sdk/v7/storage"
// )

// var (
//  bucket    = utils.Bucket
//  accessKey = utils.AccessKey
//  secretKey = utils.SecretKey
//  url       = utils.QiNiuAddress
// )

// func Upload(file multipart.File, fileSize int64) (string, int) {
//  putPolicy := storage.PutPolicy{
//      Scope: bucket,
//  }
//  mac := auth.New(accessKey, secretKey)
//  upToken := putPolicy.UploadToken(mac)

//  cfg := storage.Config{}
//  cfg.Region = &storage.ZoneHuanan
//  cfg.UseHTTPS = false
//  cfg.UseCdnDomains = false

//  formUploader := storage.NewFormUploader(&cfg)
//  ret := storage.PutRet{}
//  putExtra := storage.PutExtra{}

//  err := formUploader.PutWithoutKey(context.Background(), &ret, upToken, file, fileSize, &putExtra)
//  if err != nil {
//      return "", errors.ERROR
//  }
//  return url + ret.Key, errors.SUCCESS
// }
