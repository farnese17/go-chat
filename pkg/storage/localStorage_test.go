package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/farnese17/chat/pkg/storage"
	"github.com/farnese17/chat/pkg/storage/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

var logger *storage.Logger

const (
	fileDir = "./files/"
	logPath = "./log"
)

func clear() {
	logger.Close()
	os.RemoveAll(fileDir)
	os.RemoveAll("./log.log")
}

func setup(t *testing.T) (storage.Storage, *mock.MockDB) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockDB(ctrl)

	logger, _ = storage.SetupLogger("./")
	ls := &storage.LocalStorage{
		Path:   fileDir,
		DB:     m,
		Logger: logger,
	}
	return ls, m
}

func TestUpload(t *testing.T) {
	ls, m := setup(t)
	defer clear()

	tests := []struct {
		name     string
		content  []byte
		filename string
		IsExist  error
	}{
		{name: "upload jpg file", content: []byte("fake jpg content"), filename: "test.jpg", IsExist: nil},
		{name: "upload txt file", content: []byte("fake txt content"), filename: "test.txt", IsExist: nil},
		{name: "upload mp4 file", content: []byte("fake mp4 content"), filename: "test.mp4", IsExist: nil},
		{name: "upload jpg file,but it's existed", content: []byte("fake jpg content"), filename: "test.jpg", IsExist: storage.ErrFileExisted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.EXPECT().IsExist(gomock.Any(), gomock.Any()).Return(tt.IsExist)
			m.EXPECT().SaveFilePath(gomock.Any()).Return(nil)
			// 创建测试文件
			file := createTestFile(t, fileDir, tt.filename)
			defer file.Close()
			writeTestFile(t, file, tt.content)

			// 计算新建文件的哈希值
			want, err := ls.(*storage.LocalStorage).HashFile(file)
			assert.NoError(t, err)
			_, err = file.Seek(0, 0)
			assert.NoError(t, err)

			// 执行
			_, err = ls.Upload(1, file, tt.filename)
			assert.NoError(t, err)

			// 预期保存的目录
			ext := filepath.Ext(tt.filename)
			dir := filepath.Join(fileDir, time.Now().Format("20060102"), ext[1:])
			// 模糊搜索
			var pattern string
			if tt.IsExist == nil {
				pattern = filepath.Join(dir, want+ext)
			} else {
				pattern = filepath.Join(dir, want+"_*"+ext)
			}

			// 搜索匹配的文件
			matches, err := filepath.Glob(pattern)
			assert.NoError(t, err)

			f, err := os.Open(matches[0])
			assert.NoError(t, err)
			defer f.Close()
			// 对比保存文件的哈希值
			got, err := ls.(*storage.LocalStorage).HashFile(f)
			assert.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func createTestFile(t *testing.T, dir, name string) *os.File {
	os.Mkdir(fileDir, 0744)
	file, err := os.OpenFile(dir+name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0744)
	assert.NoError(t, err)
	return file
}

func writeTestFile(t *testing.T, file *os.File, content []byte) {
	_, err := file.Write(content)
	assert.NoError(t, err)
	_, err = file.Seek(0, 0)
	assert.NoError(t, err)
}
