package repository

import (
	"fmt"
	"time"

	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"gorm.io/gorm"
)

type UserRepository interface {
	CreateUser(data *m.User) error
	Get(value any, column string) (*m.User, error)
	Search(value any, column string) (*m.ResponseUserInfo, error)
	UpdateUserInfo(id uint, value any, column string) error
	UpdatePassword(id uint, password string) error
	Delete(id uint) error
	GetBanned(cursor *m.Cursor, lasttime int64) ([]*m.BanStatus, *m.Cursor, error)
}

type SQLUserRepository struct {
	db *gorm.DB
}

func NewSQLUserRepository(db *gorm.DB) UserRepository {
	return &SQLUserRepository{db}
}

func (s *SQLUserRepository) CreateUser(data *m.User) error {
	err := s.db.Create(data).Error
	return errorsx.HandleError(err)
}

func (s *SQLUserRepository) Get(value any, column string) (*m.User, error) {
	var user *m.User
	query := fmt.Sprintf("%s = ?", column)
	err := s.db.Where(query, value).First(&user).Error
	return user, errorsx.HandleError(err)
}

func (s *SQLUserRepository) Search(value any, column string) (*m.ResponseUserInfo, error) {
	var user *m.ResponseUserInfo
	query := fmt.Sprintf("%s = ?", column)
	err := s.db.Model(&m.User{}).Where(query, value).First(&user).Error
	return user, errorsx.HandleError(err)
}

func (s *SQLUserRepository) UpdateUserInfo(id uint, value any, column string) error {
	result := s.db.Model(&m.User{}).Where("id = ?", id).Update(column, value)
	if err := errorsx.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrRecordNotFound
	}
	return nil
}

func (s *SQLUserRepository) UpdatePassword(id uint, password string) error {
	err := s.db.Model(&m.User{}).Where("id = ?", id).Update("password", password).Error
	return errorsx.HandleError(err)
}

func (s *SQLUserRepository) Delete(id uint) error {
	err := s.db.Unscoped().Where("id = ?", id).Delete(&m.User{}).Error
	return errorsx.HandleError(err)
}

// updated_at > lasttime用于缓存增量预热
func (s *SQLUserRepository) GetBanned(cursor *m.Cursor, lasttime int64) ([]*m.BanStatus, *m.Cursor, error) {
	var users []*m.BanStatus
	now := time.Now().Unix()
	err := s.db.Model(&m.User{}).Limit(cursor.PageSize+1).
		Where("id > ? AND updated_at > ? AND ban_expire_at > ? AND ban_level != ?",
			cursor.LastID, lasttime, now, m.BanLevelNone).
		Find(&users).Error
	if err := errorsx.HandleError(err); err != nil {
		return nil, cursor, err
	}
	if len(users) > cursor.PageSize {
		users = users[:len(users)-1]
		cursor.LastID = users[len(users)-1].ID
	} else {
		cursor.HasMore = false
	}
	return users, cursor, nil
}

type TestableRepo interface {
	ExecSql(sql string, args ...any) error
}

func (t *SQLUserRepository) ExecSql(sql string, args ...any) error {
	return t.db.Exec(sql, args...).Error
}
