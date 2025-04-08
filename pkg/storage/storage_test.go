package storage_test

import (
	"os"
	"path"
	"strings"
	"testing"

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
			m.EXPECT().SaveFilePath(gomock.Any(), gomock.Any()).Return(nil)
			file := createTestFile(t, fileDir, tt.filename)
			defer file.Close()
			writeTestFile(t, file, tt.content)
			want, err := ls.(*storage.LocalStorage).HashFile(file)
			assert.NoError(t, err)
			want += path.Ext(tt.filename)
			_, err = file.Seek(0, 0)
			assert.NoError(t, err)

			got, err := ls.Upload(file, tt.filename)
			assert.NoError(t, err)
			if tt.IsExist == nil {
				assert.Equal(t, want, got)
			} else {
				assert.True(t, true, len(got) > len(want)+1)
				idx := strings.LastIndex(want, ".")
				assert.Contains(t, got, want[:idx])
			}
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
