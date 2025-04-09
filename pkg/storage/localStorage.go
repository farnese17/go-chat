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
	"strconv"
	"time"
)

var (
	ErrUploadFailed = errors.New("upload failed")
)

type Storage interface {
	Upload(file multipart.File, filename string) (uint, error)
	Download(id string) (*File, error)
	Delete(id string) error
}

type LocalStorage struct {
	Path   string
	DB     DB
	Logger *Logger
}

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
	}
	log.Println("Success connection to DB")
	return ls, nil
}

func (ls *LocalStorage) Upload(file multipart.File, filename string) (uint, error) {
	defer file.Close()
	t := time.Now().Format("20060102")
	dirPath := filepath.Join(ls.Path, t)
	name, err := ls.HashFile(file)
	if err != nil {
		return 0, ErrUploadFailed
	}
	file.Seek(0, io.SeekStart)

	ext := filepath.Ext(filename)
	fullname := name
	if len(ext) > 1 {
		dirPath = filepath.Join(dirPath, ext[1:])
		fullname += ext
	}
	if err := ls.DB.IsExist(fullname, dirPath); err != nil && !errors.Is(err, ErrNotFound) {
		if errors.Is(err, ErrFileExisted) {
			time := time.Now().UnixMilli()
			fullname = name + "_" + strconv.FormatInt(time, 10)
			if len(ext) > 1 {
				fullname += ext
			}
			fmt.Println(fullname)
		} else {
			return 0, err
		}
	}

	if err := os.MkdirAll(dirPath, 0740); err != nil {
		return 0, err
	}

	filePath := filepath.Join(dirPath, fullname)
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0740)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		os.Remove(filePath)
		return 0, ErrUploadFailed
	}

	saveFile := &File{Name: fullname, Path: dirPath}
	if err := ls.DB.SaveFilePath(saveFile); err != nil {
		os.Remove(filePath)
		return 0, ErrUploadFailed
	}

	return saveFile.ID, nil
}

func (ls *LocalStorage) Download(id string) (*File, error) {
	return ls.DB.Get(id)
}

func (ls *LocalStorage) Delete(id string) error {
	return ls.DB.DeleteFile(id)
}

// hash.sha256作为文件名
func (ls *LocalStorage) HashFile(file io.Reader) (string, error) {
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
