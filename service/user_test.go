package service_test

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/service"
	"github.com/farnese17/chat/service/mock"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/logger"
	"github.com/farnese17/chat/utils/validator"
	ws "github.com/farnese17/chat/websocket"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var (
	uid   = uint(1e5 + 1)
	gid   = uint(1e9 + 1)
	log   *zap.Logger
	ctrl  *gomock.Controller
	hub   ws.HubInterface
	mocku *mock.MockUserRepository
	mockf *mock.MockFriendRepository
	mockg *mock.MockGroupRepository
	mockc *mock.MockCache
	u     *service.UserService
	f     *service.FriendService
	g     *service.GroupService
	s     *mock.MockService
	cfg   config.Config
)

func TestMain(m *testing.M) {
	cfgPath := os.Getenv("CHAT_CONFIG")
	cfg = config.LoadConfig(cfgPath)
	validator.SetupValidator()
	log = logger.SetupLogger()
	defer log.Sync()

	os.Exit(m.Run())
}

func clear(t *testing.T) {
	ctrl.Finish()
	for {
		select {
		case msg := <-mock.Message:
			t.Errorf("Unexpected data in channel %v", msg)
		default:
			hub.Stop()
			return
		}
	}
}

func setup(t *testing.T) {
	ctrl = gomock.NewController(t)
	mocku = mock.NewMockUserRepository(ctrl)
	mockf = mock.NewMockFriendRepository(ctrl)
	mockg = mock.NewMockGroupRepository(ctrl)
	mockc = mock.NewMockCache(ctrl)
	hub = mock.NewMockHub()
	go hub.Run()

	s = mock.NewMockService(ctrl)
	s.EXPECT().Config().Return(cfg).AnyTimes()
	s.EXPECT().Logger().Return(log).AnyTimes()
	s.EXPECT().User().Return(mocku).AnyTimes()
	s.EXPECT().Friend().Return(mockf).AnyTimes()
	s.EXPECT().Group().Return(mockg).AnyTimes()
	s.EXPECT().Cache().Return(mockc).AnyTimes()
	s.EXPECT().Hub().Return(hub).AnyTimes()

	u = service.NewUserService(s)
	f = service.NewFriendService(s)
	g = service.NewGroupService(s)
}

func TestRegister(t *testing.T) {
	setup(t)
	defer clear(t)

	users := []*model.User{
		{Username: "", Password: "123456", Email: "a@test.com"},
		{Username: "   ", Password: "123456", Email: "a@test.com"},
		{Username: "a", Password: "123456", Email: "a@test.com"},
		{Username: "abcdefghi", Password: "123456", Email: "abcdefg@test.com"},
		{Username: "ab", Password: "", Email: "ab@test.com"},
		{Username: "ac", Password: "12345", Email: "ab@test.com"},
		{Username: "ad", Password: "12345678901234567", Email: "ab@test.com"},
		{Username: "ae", Password: "123456", Email: "abtest.com"},
		{Username: "af", Password: "123456", Email: "ab@testcom"},
		{Username: "ag", Password: "123456", Phone: "11815815815"},
		{Username: "ah", Password: "123456", Phone: "1581581581"},
		{Username: "ai", Password: "123456", Phone: "158158158158"},
		{Username: "aj", Password: "1234  56", Email: "ab@test.com"},
		{Username: "ak", Password: " 123456", Email: "ab@test.com"},
		{Username: "al", Password: "1234  56 ", Email: "ab@test.com"},
		{Username: "am", Password: "123456", Email: "a b@test.com"},
		{Username: "an", Password: "123456", Email: "ab@t est.com"},
		{Username: "ao", Password: "123456", Email: "ab@test.c om"},
		{Username: "ap", Password: "123456", Email: " ab@t est.com"},
		{Username: "aq", Password: "123456", Email: "ab@t est.com "},
		{Username: "ar", Password: "123456", Phone: "158158  158", Email: "ab@test.com"},
		{Username: "as", Password: "123456", Phone: " 1581581581", Email: "ab@test.com"},
		{Username: "at", Password: "123456", Phone: "1581581581 ", Email: "ab@test.com"},
	}
	expected := []error{
		errors.New("用户名为必填字段"),
		errors.New("用户名不能为空"),
		errors.New("用户名长度必须至少为2个字符"),
		errors.New("用户名长度不能超过8个字符"),
		errors.New("密码为必填字段"),
		errors.New("密码长度应该在6到16个字符之间"),
		errors.New("密码长度应该在6到16个字符之间"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的手机号"),
		errors.New("请输入正确的手机号"),
		errors.New("请输入正确的手机号"),
		errors.New("密码不能包含空格"),
		errors.New("密码不能包含空格"),
		errors.New("密码不能包含空格"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的邮箱地址"),
		errors.New("请输入正确的手机号"),
		errors.New("请输入正确的手机号"),
		errors.New("请输入正确的手机号"),
	}

	for i, user := range users {
		t.Run(fmt.Sprintf("Register %s", user.Username), func(t *testing.T) {
			err := u.Register(user)
			assert.Equal(t, expected[i], err)
		})
	}

	users = []*model.User{
		{Username: "test1", Password: "123456", Email: "test1@test.com"},
		{Username: "test2", Password: "123456", Phone: "15815815815"},
		{Username: "test3", Password: "123456", Email: "test3@test.com"},
		{Username: "a c", Password: "123456", Email: "ac@test.com"},
		{Username: "ab", Password: "123456", Phone: "15815815815", Email: "ab@test.com"},
		{Username: "ab", Password: "123456", Phone: "15815815815"},
		{Username: "ab", Password: "123456", Email: "ab@test.com"},
		{Username: "abcd", Password: "123456", Email: "abcd@test.com"},
	}
	expected = []error{nil, nil, nil, nil, nil,
		errorsx.ErrPhoneRegistered,
		errorsx.ErrEmailRegistered,
		errorsx.ErrRegisterFailed,
	}
	mockErrors := []error{nil, nil, nil, nil, nil,
		errorsx.ErrDuplicateEntryPhone,
		errorsx.ErrDuplicateEntryEmail,
		errors.New("error"),
	}
	for i, user := range users {
		mocku.EXPECT().CreateUser(user).Return(mockErrors[i]).MinTimes(1)
		t.Run(fmt.Sprintf("Register %s", user.Username), func(t *testing.T) {
			err := u.Register(user)
			assert.Equal(t, expected[i], err)
		})
	}
}

func TestDeleteUser(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		mock     error
		expected error
	}{
		{errorsx.ErrForeignKeyViolatedGroup, errorsx.ErrHasGroupNeedHandOver},
		{errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{nil, nil},
	}
	for i, tt := range tests {
		mocku.EXPECT().Delete(uid).Return(tt.mock)
		t.Run(fmt.Sprintf("delete user %d", i), func(t *testing.T) {
			err := u.Delete(uid)
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestSearchUser(t *testing.T) {
	setup(t)
	defer clear(t)

	user := &model.ResponseUserInfo{ID: uid, Username: "test"}

	tests := []struct {
		account  string
		mock     error
		expected error
		mockAll  bool
	}{
		{"", nil, errorsx.ErrInputEmpty, false},
		{"100000", nil, errorsx.ErrInvalidParams, false},
		{"test", nil, errorsx.ErrInvalidParams, false},
		{"110000", errorsx.ErrRecordNotFound, errorsx.ErrUserNotExist, true},
		{"100001", errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{"100001", nil, nil, true},
	}

	for i, tt := range tests {
		if tt.mockAll {
			mocku.EXPECT().Search(gomock.Any(), gomock.Any()).Return(user, tt.mock)
		}
		t.Run(fmt.Sprintf("Search user %d", i), func(t *testing.T) {
			data, err := u.SreachUser(tt.account)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				assert.Equal(t, user, data)
			}
		})
	}

	tests = []struct {
		account  string
		mock     error
		expected error
		mockAll  bool
	}{
		{"", nil, errorsx.ErrInvalidParams, false},
		{"100000", nil, errorsx.ErrInvalidParams, false},
		{"test", nil, errorsx.ErrInvalidParams, false},
		{"110000", errorsx.ErrRecordNotFound, errorsx.ErrUserNotExist, true},
		{"100001", errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{"100001", nil, nil, true},
	}

	for i, tt := range tests {
		if tt.mockAll {
			mocku.EXPECT().Search(gomock.Any(), gomock.Any()).Return(user, tt.mock)
		}
		t.Run(fmt.Sprintf("get user %d", i), func(t *testing.T) {
			uid, _ := strconv.ParseUint(tt.account, 10, 64)
			data, err := u.Get(uint(uid))
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				assert.Equal(t, user, data)
			}
		})
	}
}

func TestUpdateUserInfo(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		val, column string
		mock        error
		expected    error
	}{
		{"test", "username", errorsx.ErrRecordNotFound, nil},
		{"test", "username", errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{"test", "username", nil, nil},
		{"15015015015", "phone", errorsx.ErrDuplicateEntryPhone, errorsx.ErrPhoneRegistered},
		{"15015015015", "phone", nil, nil},
		{"test@test.com", "email", errorsx.ErrDuplicateEntryEmail, errorsx.ErrEmailRegistered},
		{"test@test.com", "email", nil, nil},
		{"This is an avatar", "avatar", nil, nil},
	}

	for i, tt := range tests {
		mocku.EXPECT().UpdateUserInfo(uid, tt.val, tt.column).Return(tt.mock)
		t.Run(fmt.Sprintf("update information %d", i), func(t *testing.T) {
			err := u.UpdateInfo(uid, tt.val, tt.column)
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestUpdatePassword(t *testing.T) {
	setup(t)
	defer clear(t)

	hashPW, _ := utils.HashPassword("123456")
	user := &model.User{
		ID: 100001, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
		Username: "test",
		Password: hashPW,
		Phone:    "15815815815",
		Email:    "test@test.com",
		Avatar:   "This is an avatar"}

	tests := []struct {
		old, new, confirm string
		mockGet           *model.User
		mockGetErr        error
		mock              error
		expected          error
	}{
		{"", "123456", "123456", nil, nil, nil, errors.New("密码长度应该在6到16个字符之间")},
		{"1", "123456", "123456", nil, nil, nil, errors.New("密码长度应该在6到16个字符之间")},
		{strings.Repeat("a", 17), "123456", "123456", nil, nil, nil, errors.New("密码长度应该在6到16个字符之间")},
		{"123456", "123456", "123456", nil, nil, nil, errorsx.ErrSamePassword},
		{"1234567", "123456", "1234567", nil, nil, nil, errorsx.ErrDifferentPassword},
		{"123456", "aaaaaa", "aaaaaa", user, errorsx.ErrRecordNotFound, nil, errorsx.ErrUserNotExist},
		{"123456", "aaaaaa", "aaaaaa", user, errorsx.HandleError(errors.New("error")), nil, errorsx.ErrFailed},
		{"123456", "aaaaaa", "aaaaaa", user, nil, nil, nil},
	}

	for i, tt := range tests {
		if tt.mockGet != nil {
			mocku.EXPECT().Get(uid, "id").Return(tt.mockGet, tt.mockGetErr)
		}
		if tt.expected == nil {
			mocku.EXPECT().UpdatePassword(uid, gomock.Any()).Return(nil)
		}
		password := map[string]string{"old": tt.old, "new": tt.new, "confirm": tt.confirm}
		t.Run(fmt.Sprintf("update password %d", i), func(t *testing.T) {
			err := u.UpdatePassword(uid, password)
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestLogin(t *testing.T) {
	setup(t)
	defer clear(t)

	user := &model.User{
		ID: 100001, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
		Username: "test",
		Password: "$2a$10$/q2SN.0SsetWFYI8vVs1aONeQUdARvyw6FmMsxS6yAcvy.z0HbFXi",
		Phone:    "15815815815",
		Email:    "test@test.com",
		Avatar:   "This is an avatar"}

	now := time.Now()
	tests := []struct {
		account, password string
		banLevel          int
		banExpireAt       int64
		expected          error
	}{
		{"100001", "aaaaaa", 0, 0, errorsx.ErrUsernameOrPasswordWrong},
		{"100001", "aaaaaa", model.BanLevelPermanent, now.Add(24 * time.Hour).Unix(), fmt.Errorf("%s: %v", errorsx.ErrBanned.Error(), time.Unix(now.Add(24*time.Hour).Unix(), 0).Format("2006-01-02 15:04:05"))},
		{"100001", "aaaaaa", model.BanLevelPermanent, now.Add(48 * time.Hour).Unix(), fmt.Errorf("%s: %v", errorsx.ErrBanned.Error(), time.Unix(now.Add(48*time.Hour).Unix(), 0).Format("2006-01-02 15:04:05"))},
		{"100001", "123456", 0, 0, nil},
		{"13013013013", "123456", 0, 0, nil},
		{"test@mail.com", "123456", 0, 0, nil},
	}

	for i, tt := range tests {
		user.BanLevel = tt.banLevel
		user.BanExpireAt = tt.banExpireAt
		mocku.EXPECT().Get(gomock.Any(), gomock.Any()).Return(user, nil)
		t.Run(fmt.Sprintf("login %d", i), func(t *testing.T) {
			data, err := u.Login(tt.account, tt.password)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				assert.Equal(t, &model.ResponseUserInfo{
					ID:       100001,
					Username: "test",
					Phone:    "15815815815",
					Email:    "test@test.com",
					Avatar:   "This is an avatar",
				}, data)
			}
		})
	}
}

func TestGetAccountFeild(t *testing.T) {
	setup(t)
	defer clear(t)

	expected := errorsx.ErrInvalidParams
	tests := []struct {
		account string
		feild   string
		err     error
	}{
		{"13000000000", "phone", nil},
		{"13013013013", "phone", nil},
		{"14014014014", "phone", nil},
		{"15815815815", "phone", nil},
		{"16815815815", "phone", nil},
		{"17815815815", "phone", nil},
		{"18018018018", "phone", nil},
		{"19815815815", "phone", nil},
		{"00013013013", "", expected},
		{"10013013013", "", expected},
		{"11013013013", "", expected},
		{"12013013013", "", expected},
		{"1201301301", "", expected},
		{"120130130133", "", expected},
		{"1201301301a", "", expected},
		{"a2013013013", "", expected},
		{"1301a013013", "", expected},
		{"aaaaaaaaaaa", "", expected},
		{"test@test.com", "email", nil},
		{"a@mail.com", "email", nil},
		{"z@mail.com", "email", nil},
		{"A@mail.com", "email", nil},
		{"Z@mail.com", "email", nil},
		{"0@mail.com", "email", nil},
		{"9@mail.com", "email", nil},
		{"abc.abc@mail.com", "email", nil},
		{"abc.abc.abc@mail.com", "email", nil},
		{"abc_abc_abc@mail.com", "email", nil},
		{"abc-abc-abc@mail.com", "email", nil},
		{"abc-abc_abc@mail.com", "email", nil},
		{"abc-abc.abc@mail.com", "email", nil},
		{"abc.abc_abc@mail.com", "email", nil},
		{"abc-abc.abc-abc@mail.com", "email", nil},
		{"abc@abc-mail.com", "email", nil},
		{"abc.abc@abc-mail.com", "email", nil},
		{"abc_abc@abc-mail.com", "email", nil},
		{"abc-abc@abc-mail.com", "email", nil},
		{"abc@abc-mail.com.cn", "email", nil},
		{"abc@abc-mail.com.cn.aa.bb", "email", nil},
		{"abc@abc-mail.com.cn.aa.bb.cc", "email", nil},
		{"_@mail.com", "", expected},
		{"-@mail.com", "", expected},
		{".@mail.com", "", expected},
		{"a-@mail.com", "", expected},
		{"a_@mail.com", "", expected},
		{"a.@mail.com", "", expected},
		{"a+b@mail.com", "", expected},
		{"a#b@mail.com", "", expected},
		{"a*b@mail.com", "", expected},
		{"a/b@mail.com", "", expected},
		{"a!b@mail.com", "", expected},
		{"a b@mail.com", "", expected},
		{"a__a@mail.com", "", expected},
		{"a..a@mail.com", "", expected},
		{"a--a@mail.com", "", expected},
		{"a_-a@mail.com", "", expected},
		{"a.-a@mail.com", "", expected},
		{"a_.a@mail.com", "", expected},
		{"abc@-mail.com", "", expected},
		{"abc@_mail.com", "", expected},
		{"abc@.mail.com", "", expected},
		{"abc@.mail-.com", "", expected},
		{"abc@.mail_.com", "", expected},
		{"abc@.mail..com", "", expected},
		{"abc@mail.com-", "", expected},
		{"abc@mail.com_", "", expected},
		{"abc@mail.com.", "", expected},
		{"abc@mail.com?", "", expected},
		{"abc@mail?.com", "", expected},
		{"abc@?mail.com", "", expected},
		{"abc@mail.com.com.com..com", "", expected},
		{"100001", "id", nil},
		{strings.Repeat("9", 9), "id", nil},
		{"test", "", expected},
		{"100000", "", expected},
		{strings.Repeat("9", 10), "", expected},
		{"1", "", expected},
		{"0", "", expected},
		{"99999", "", expected},
		{"-1", "", expected},
		{"-99999", "", expected},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("test get account feild %s", tt.account), func(t *testing.T) {
			val, err := u.GetAccountField(tt.account)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.feild, val)
		})
	}
}
