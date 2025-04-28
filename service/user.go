package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/farnese17/chat/registry"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/validator"
	"go.uber.org/zap"
)

type UserService struct {
	service registry.Service
}

func NewUserService(s registry.Service) *UserService {
	return &UserService{s}
}

func (u *UserService) Register(data *m.User) error {
	if err := validator.Validate(data); err != nil {
		u.service.Logger().Error("Failed to register user", zap.Any("user", data))
		return err
	}

	pw, err := utils.HashPassword(data.Password)
	if err != nil {
		return errorsx.ErrFailed
	}
	data.Password = pw
	maxRetries := u.service.Config().Common().MaxRetries()
	for try := 0; try < maxRetries; try++ {
		err := u.service.User().CreateUser(data)
		if err == nil {
			u.service.Logger().Info("Finished register user",
				zap.Uint("id", data.ID),
				zap.String("username", data.Username),
				zap.String("phone", data.Phone),
				zap.String("email", data.Email),
			)
			return nil
		}
		if errors.Is(err, errorsx.ErrDuplicateEntryPhone) {
			u.service.Logger().Info("Failed to register: phone already registered", zap.String("phone", data.Phone))
			return errorsx.ErrPhoneRegistered
		} else if errors.Is(err, errorsx.ErrDuplicateEntryEmail) {
			u.service.Logger().Info("Failed to register: email already registered", zap.String("email", data.Email))
			return errorsx.ErrEmailRegistered
		} else {
			u.service.Logger().Warn("Failed to create user, retrying...", zap.Error(err))
			delay := u.service.Config().Common().RetryDelay(try)
			time.Sleep(delay)
		}
	}
	u.service.Logger().Error("Failed to register user after 3 retries")
	return errorsx.ErrRegisterFailed
}

func (u *UserService) Delete(id uint) error {
	if err := u.service.User().Delete(id); err != nil {
		if errors.Is(err, errorsx.ErrForeignKeyViolatedGroup) {
			u.service.Logger().Info("Failed to delete user: has groups", zap.Uint("id", id))
			return errorsx.ErrHasGroupNeedHandOver
		}
		u.service.Logger().Error("Filed to delete user", zap.Error(err))
		return err
	}
	u.service.Logger().Info("Finished delete user", zap.Uint("id", id))
	return nil
}

// 接受uid/手机号/邮箱
func (u *UserService) SreachUser(account string) (*m.ResponseUserInfo, error) {
	if account == "" {
		return nil, errorsx.ErrInputEmpty
	}
	field, err := u.GetAccountField(account)
	if err != nil {
		return nil, err
	}
	user, err := u.service.User().Search(account, field)
	if err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			return nil, errorsx.ErrUserNotExist
		}
		u.service.Logger().Error("Failed to search", zap.Error(err))
		return nil, err
	}
	return user, nil
}

func (u *UserService) Get(uid uint) (*m.ResponseUserInfo, error) {
	if err := validator.ValidateUID(uid); err != nil {
		return nil, err
	}
	user, err := u.service.User().Search(uid, "id")
	if err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			return nil, errorsx.ErrUserNotExist
		}
		u.service.Logger().Error("Failed to search", zap.Error(err))
		return nil, err
	}
	return user, nil
}

func (u *UserService) UpdateInfo(id uint, value string, field string) error {
	switch field {
	case "avatar":
	case "username":
		if err := validator.ValidateUsername(value); err != nil {
			return err
		}
	case "phone":
		if err := validator.ValidateMobile(value); err != nil {
			return err
		}
	case "email":
		if err := validator.ValidateEmail(value); err != nil {
			return err
		}
	default:
		u.service.Logger().Warn("Failed to update information: invalid field", zap.String("field", field))
		return errorsx.ErrInvalidParams
	}
	if err := u.service.User().UpdateUserInfo(id, value, field); err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			u.service.Logger().Error("Failed to update user information: user not found", zap.Uint("id", id))
			return nil
		}
		if errors.Is(err, errorsx.ErrDuplicateEntryPhone) {
			u.service.Logger().Error("Failed to update user information: phone is used", zap.Uint("id", id), zap.String("phone", value))
			return errorsx.ErrPhoneRegistered
		}
		if errors.Is(err, errorsx.ErrDuplicateEntryEmail) {
			u.service.Logger().Error("Failed to update user information: email is used", zap.Uint("id", id), zap.String("phone", value))
			return errorsx.ErrEmailRegistered
		}
		u.service.Logger().Error("Failed to update userinfo", zap.Error(err))
		return err
	}
	return nil
}

func (u *UserService) UpdatePassword(id uint, password map[string]string) error {
	old, new, confirm := password["old"], password["new"], password["confirm"]
	for _, pw := range []string{old, new, confirm} {
		if err := validator.ValidatePassword(pw); err != nil {
			u.service.Logger().Warn("Failed to update password: invalid password", zap.Error(err))
			return err
		}
	}
	if old == new {
		return errorsx.ErrSamePassword
	}
	if new != confirm {
		return errorsx.ErrDifferentPassword
	}
	user, err := u.service.User().Get(id, "id")
	if err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			u.service.Logger().Error("Failed to update password: user does not exist", zap.Uint("id", id))
			return errorsx.ErrUserNotExist
		}
		u.service.Logger().Error("Failed to update password: retrieving user", zap.Error(err))
		return err
	}
	if !utils.ComparePassword(user.Password, old) {
		return errorsx.ErrWrongPassword
	}
	hashedPW, err := utils.HashPassword(new)
	if err != nil {
		return err
	}
	err = u.service.User().UpdatePassword(id, hashedPW)
	if err != nil {
		u.service.Logger().Error("Failed to update password", zap.Error(err))
		return err
	}
	u.service.Logger().Info("Update password successful", zap.Uint("id", id))
	return nil
}

func (u *UserService) Login(account string, password string) (*m.ResponseUserInfo, error) {
	if err := validator.ValidatePassword(password); err != nil {
		return nil, errorsx.ErrUsernameOrPasswordWrong
	}
	column, err := u.GetAccountField(account)
	if err != nil {
		return nil, errorsx.ErrUsernameOrPasswordWrong
	}
	user, err := u.service.User().Get(account, column)
	if err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			u.service.Logger().Info("Failed to login, User not existed", zap.String("account", account))
			return nil, errorsx.ErrUserNotExist
		}
		u.service.Logger().Error("Failed to login: retrieving user", zap.Error(err))
		return nil, err
	}
	if err := u.ifBannedThenReturn(user.ID, user.BanLevel, user.BanExpireAt); err != nil {
		return nil, err
	}

	if !utils.ComparePassword(user.Password, password) {
		return nil, errorsx.ErrUsernameOrPasswordWrong
	}
	u.service.Logger().Info("Login successful", zap.String("account", account))
	userinfo := &m.ResponseUserInfo{
		ID:       user.ID,
		Username: user.Username,
		Avatar:   user.Avatar,
		Phone:    user.Phone,
		Email:    user.Email,
	}
	return userinfo, nil
}

func (u *UserService) ifBannedThenReturn(id uint, banlevel int, expireT int64) error {
	s := errorsx.ErrBanned.Error()
	t := time.Unix(expireT, 0).Format("2006-01-02 15:04:05")
	if banlevel == m.BanLevelPermanent {
		return fmt.Errorf("%s: %v", s, t)
	}
	if banlevel == m.BanLevelTemporary {
		if expireT > time.Now().Unix() {
			return fmt.Errorf("%s: %v", s, t)
		}
		// 封禁已过期，更新
		u.service.Manager().Unban(strconv.Itoa(int(id)))
	}
	return nil
}

func (u *UserService) GetAccountField(value string) (string, error) {
	if err := validator.ValidateMobile(value); err == nil {
		return "phone", nil
	}
	if err := validator.ValidateEmail(value); err == nil {
		return "email", nil
	}
	if len(value) < 10 {
		id, _ := strconv.ParseUint(value, 10, 64)
		if err := validator.ValidateUID(uint(id)); err == nil {
			return "id", nil
		}
	}
	return "", errorsx.ErrInvalidParams
}

func (u *UserService) Logout(id uint) error {
	key := m.CacheToken + strconv.Itoa(int(id))
	u.service.Cache().Remove(key)
	return nil
}
