package service

import (
	"strconv"
	"time"

	"github.com/farnese17/chat/registry"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/validator"
)

type Manager struct {
	service registry.Service
}

func NewManager(s registry.Service) *Manager {
	return &Manager{s}
}

func (mgr *Manager) CreateAdmin(data *m.Manager) (uint, error) {
	hashPW, err := utils.HashPassword(data.Password)
	if err != nil {
		return 0, err
	}
	data.Password = hashPW
	if err := mgr.service.Manager().Create(data); err != nil {
		return 0, err
	}
	return data.ID, nil
}

func (mgr *Manager) DeleteAdmin(id uint) error {
	err := mgr.service.Manager().Delete(id)
	if err == nil {
		key := m.CacheToken + strconv.Itoa(int(id))
		mgr.service.Cache().Remove(key)
	}
	return err
}

func (mgr *Manager) RestoreAdministrator(id uint) error {
	return mgr.service.Manager().RestoreAdministrator(id)
}

// 中间件验证权限
func (mgr *Manager) UpdatePassword(handler, id uint, new, confirm string) error {
	if handler != id {
		u, err := mgr.GetAdmin(handler)
		if err != nil {
			return err
		}
		if u.Permissions != m.MgrSuperAdministrator {
			return errorsx.ErrPermissiondenied
		}
	}

	pw := []string{new, confirm}
	for _, p := range pw {
		if err := validator.ValidatePassword(p); err != nil {
			return err
		}
	}

	if new != confirm {
		return errorsx.ErrDifferentPassword
	}
	hashPW, err := utils.HashPassword(new)
	if err != nil {
		return err
	}
	return mgr.service.Manager().UpdatePassword(id, hashPW)
}

func (mgr *Manager) Setpermission(id uint, permission uint) error {
	return mgr.service.Manager().SetPermission(id, permission)
}

func (mgr *Manager) Ban(id string, level int, days int) error {
	uid, _ := strconv.Atoi(id)
	if err := validator.ValidateUID(uint(uid)); err != nil {
		return err
	}

	expire := time.Duration(days) * 24 * time.Hour
	expireAt := time.Now().Add(expire).Unix()
	err := mgr.service.Manager().Ban(id, level, expireAt)
	if err != nil {
		return err
	}
	mgr.service.Cache().SetBanned(id, level, expire)
	if level == m.BanLevelTemporary || level == m.BanLevelPermanent {
		key := m.CacheToken + id
		mgr.service.Cache().Remove(key)
		mgr.service.Hub().Kick(uint(uid))
		if level == m.BanLevelPermanent {
			mgr.service.Cache().BFM().BanUser(uint(uid))
		}
	}
	if level == m.BanLevelMuted {
		mgr.service.Cache().BFM().AddMute(uint(uid), expireAt)
	}

	return nil
}

func (mgr *Manager) Unban(id string) error {
	uid, _ := strconv.Atoi(id)
	if err := validator.ValidateUID(uint(uid)); err != nil {
		return err
	}

	err := mgr.service.Manager().Unban(id)
	if err == nil {
		key := m.CacheBanned + id
		mgr.service.Cache().Remove(key)
		mgr.service.Cache().BFM().UnbanUser(uint(uid))
	}
	return err
}

func (mgr *Manager) BannedUserList(cursor *m.Cursor) (map[string]any, error) {
	data, c, err := mgr.service.User().GetBanned(cursor, 0)
	res := map[string]any{"cursor": c}
	if err != nil {
		return res, err
	}
	res["data"] = data
	return res, nil
}

func (mgr *Manager) CountBannedUser() (int64, error) {
	return mgr.service.Manager().CountBannedUser()
}

func (mgr *Manager) AdminList(cursor *m.Cursor) (map[string]any, error) {
	if !cursor.HasMore {
		return nil, nil
	}
	if err := validator.VerfityPageSize(cursor.PageSize); err != nil {
		return nil, err
	}
	c, admins, err := mgr.service.Manager().List(cursor)
	res := map[string]any{"cursor": c}
	if err != nil {
		return res, err
	}
	for _, admin := range admins {
		admin.Password = ""
	}
	res["data"] = admins
	return res, nil
}

func (mgr *Manager) GetAdmin(id uint) (*m.Manager, error) {
	admin, err := mgr.service.Manager().Get(id)
	if err != nil {
		return nil, err
	}
	admin.Password = ""
	return admin, nil
}

func (mgr *Manager) Login(id uint, password string) error {
	if err := validator.ValidatePassword(password); err != nil {
		return err
	}
	admin, err := mgr.service.Manager().Get(id)
	if err != nil {
		return err
	}
	if admin.Deleted_At != 0 {
		return errorsx.ErrUserNotExist
	}
	if !utils.ComparePassword(admin.Password, password) {
		return errorsx.ErrUsernameOrPasswordWrong
	}
	return nil
}
