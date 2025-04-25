package repository

import (
	"time"

	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"gorm.io/gorm"
)

type Manager interface {
	Create(data *m.Manager) error
	Delete(id uint) error
	RestoreAdministrator(id uint) error
	UpdatePassword(id uint, pw string) error
	Ban(id string, level int, expireAt int64) error
	Unban(id string) error
	CountBannedUser() (int64, error)
	SetPermission(id uint, permission uint) error
	Get(id uint) (*m.Manager, error)
	List(cursor *m.Cursor) (*m.Cursor, []*m.Manager, error)
}

type SQLManagerRepository struct {
	db *gorm.DB
}

func NewSQLManagerRepository(db *gorm.DB) Manager {
	return &SQLManagerRepository{db}
}

func (s *SQLManagerRepository) Create(data *m.Manager) error {
	err := s.db.Create(data).Error
	return errorsx.HandleError(err)
}

func (s *SQLManagerRepository) Delete(id uint) error {
	err := s.db.Model(&m.Manager{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now().Unix()).Error
	return errorsx.HandleError(err)
}

func (s *SQLManagerRepository) RestoreAdministrator(id uint) error {
	err := s.db.Model(&m.Manager{}).
		Where("id = ?", id).
		Update("deleted_at", nil).Error
	return errorsx.HandleError(err)
}

func (s *SQLManagerRepository) UpdatePassword(id uint, pw string) error {
	err := s.db.Model(&m.Manager{}).Where("id = ?", id).Update("password", pw).Error
	return errorsx.HandleError(err)
}

func (s *SQLManagerRepository) Ban(id string, level int, expire int64) error {
	result := s.db.Model(&m.User{}).
		Where("id = ? AND ban_level != ?", id, m.BanLevelPermanent).
		Updates(m.User{BanLevel: level, BanExpireAt: expire})
	if err := errorsx.HandleError(result.Error); err != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrRecordNotFound
	}
	return nil
}

func (s *SQLManagerRepository) Unban(id string) error {
	updates := map[string]any{
		"ban_level":     m.BanLevelNone,
		"ban_expire_at": 0,
	}
	result := s.db.Model(&m.User{}).
		Where("id = ?", id).
		Updates(updates)
	if err := errorsx.HandleError(result.Error); err != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrRecordNotFound
	}
	return nil
}

func (s *SQLManagerRepository) CountBannedUser() (int64, error) {
	var count int64
	err := s.db.Model(&m.User{}).Where("ban_level != ?", m.BanLevelNone).Count(&count).Error
	return count, errorsx.HandleError(err)
}

func (s *SQLManagerRepository) SetPermission(id uint, permission uint) error {
	q := `UPDATE manager SET permissions = ? WHERE id = ?`
	err := s.db.Exec(q, permission, id).Error
	return errorsx.HandleError(err)
}

func (s *SQLManagerRepository) List(cursor *m.Cursor) (*m.Cursor, []*m.Manager, error) {
	var data []*m.Manager
	err := s.db.Limit(cursor.PageSize+1).
		Where("id > ?", cursor.LastID).
		Find(&data).Error
	if err := errorsx.HandleError(err); err != nil {
		return cursor, data, err
	}
	if len(data) > cursor.PageSize {
		data = data[:len(data)-1]
		cursor.LastID = data[len(data)-1].ID
	} else {
		cursor.HasMore = false
	}
	return cursor, data, nil
}

func (s *SQLManagerRepository) Get(id uint) (*m.Manager, error) {
	var admin *m.Manager
	err := s.db.Where("id = ?", id).First(&admin).Error
	return admin, errorsx.HandleError(err)
}
