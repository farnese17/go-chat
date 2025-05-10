package errorsx

import (
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// 错误消息
var (
	ErrNil                           = errors.New("OK")
	ErrHandleSuccessed               = errors.New("处理成功但可能会延迟生效")
	ErrMessagePushServiceUnavailabel = errors.New("消息已发送但可能会延迟到达")
	ErrUnkonwnMessageType            = errors.New("未知消息类型")
	ErrFailed                        = errors.New("FAIL")
	ErrOperactionTimeout             = errors.New("操作超时")
	ErrNotFound                      = errors.New("not found")
	ErrInvalidToken                  = errors.New("无效token")
	ErrCantParseToken                = errors.New("无法解析token,请稍后再试")
	ErrServerClosed                  = errors.New("服务已停止")
	ErrServerStarted                 = errors.New("服务已启动")
	ErrSystemUnavailable             = errors.New("系统内部错误,请稍后再试")
	ErrNoSettingOption               = errors.New("没有找到配置项")
	ErrSystemBusy                    = errors.New("系统繁忙,请稍后再试")
	ErrUnknownError                  = errors.New("未知错误")
	ErrOperactionFailed              = errors.New("操作失败,请重试")
	ErrOperactionSuccess             = errors.New("操作成功")
	ErrUploadFailed                  = errors.New("上传失败，请重试")
	ErrFileExisted                   = errors.New("文件已存在")
	ErrUserExisted                   = errors.New("用户已存在")
	ErrUserNotExist                  = errors.New("用户不存在")
	ErrUserNotLogin                  = errors.New("用户未登录")
	ErrLoginFailed                   = errors.New("登录失败,请重试")
	ErrLoginExpired                  = errors.New("登录已过期,请重新登录")
	ErrGetUserInfoFailed             = errors.New("获取用户信息失败,请重试")
	ErrRegisterFailed                = errors.New("注册失败,请重试")
	ErrPhoneRegistered               = errors.New("该手机号已注册")
	ErrEmailRegistered               = errors.New("该邮箱已注册")
	ErrInputEmpty                    = errors.New("输入不能为空")
	ErrPermissiondenied              = errors.New("权限不足")
	ErrUsernameOrPasswordWrong       = errors.New("用户名或密码错误")
	ErrWrongPassword                 = errors.New("密码错误")
	ErrDifferentPassword             = errors.New("两次输入的密码不一致")
	ErrSamePassword                  = errors.New("新密码不能和旧密码一致")
	ErrNoLogin                       = errors.New("请先登录后再进行操作")
	ErrHasGroupNeedHandOver          = errors.New("注销账号前,请先移交群聊")
	ErrBanned                        = errors.New("你已被禁止")
	ErrUserBanned                    = errors.New("该用户已被禁止")
	ErrUserMuted                     = errors.New("该用户已被禁言")
	//
	ErrAlreadyFriend  = errors.New("对方已经是你的好友")
	ErrBlocked        = errors.New("你在对方的黑名单中")
	ErrAlreadyRequest = errors.New("已经发送好友请求")
	ErrAlreadyBlock   = errors.New("对方在黑名单中")
	ErrNoBlocked      = errors.New("对方不在黑名单中")
	ErrNoRequest      = errors.New("对方没有发送好友请求")
	//
	ErrAlreadyInGroup       = errors.New("已经在群组中")
	ErrJoinGroup            = errors.New("%s加入群组")
	ErrGroupNotFound        = errors.New("群组不存在")
	ErrNoGroup              = errors.New("找不到群组")
	ErrInvalidRole          = errors.New("无效角色")
	ErrInvalidParams        = errors.New("无效参数")
	ErrCantBanAdmin         = errors.New("不能Ban管理员")
	ErrAlreadyAdmin         = errors.New("该用户已经是管理员")
	ErrAlreadyMember        = errors.New("该用户已经是普通成员")
	ErrNotExpectedRole      = errors.New("不是预期角色")
	ErrNotInGroup           = errors.New("用户未加入群组")
	ErrPageSizeTooSmall     = errors.New("页面条目太小")
	ErrPageSizeTooBig       = errors.New("页面条目太大")
	ErrAlreadyApply         = errors.New("已经发送申请")
	ErrNoApplied            = errors.New("用户未申请")
	ErrJoinGroupFailed      = errors.New("加入群组失败")
	ErrNoInvite             = errors.New("你未被邀请加入群组")
	ErrOwnerCantLeave       = errors.New("群主不能退出群聊,请先移交群主")
	ErrCantSetMyselfAdmin   = errors.New("不能将自己设置为管理员")
	ErrHandOverOwnerFirst   = errors.New("请先移交群主")
	ErrNotAdmin             = errors.New("你不是管理员")
	ErrInvitationHasExpired = errors.New("邀请已过期")
	ErrCantKickAdmin        = errors.New("不能踢出管理员")
	ErrNotInApplyList       = errors.New("对方不在申请列表中")
	ErrCantSearchNull       = errors.New("搜索值不能为空")
)

var StatusCode = map[error]int{
	ErrNil:                           200,
	ErrHandleSuccessed:               200,
	ErrMessagePushServiceUnavailabel: 200,
	ErrFailed:                        500,
	ErrOperactionTimeout:             408,
	ErrNotFound:                      404,
	ErrInvalidToken:                  401,
	ErrCantParseToken:                401,
	ErrServerClosed:                  503,
	ErrServerStarted:                 503,
	ErrSystemUnavailable:             503,
	ErrNoSettingOption:               800,
	ErrSystemBusy:                    801,
	ErrUnknownError:                  1000,
	ErrUserExisted:                   1001,
	ErrUserNotExist:                  1002,
	ErrUserNotLogin:                  1003,
	ErrLoginFailed:                   1004,
	ErrLoginExpired:                  1005,
	ErrGetUserInfoFailed:             1006,
	ErrRegisterFailed:                1007,
	ErrPhoneRegistered:               1008,
	ErrEmailRegistered:               1009,
	ErrInputEmpty:                    1010,
	ErrPermissiondenied:              1011,
	ErrUploadFailed:                  1501,
	ErrFileExisted:                   1502,
	ErrOperactionFailed:              1601,
	ErrOperactionSuccess:             1602,
	ErrNoLogin:                       1603,
	ErrHasGroupNeedHandOver:          1604,
	ErrBanned:                        1605,
	ErrUserBanned:                    1606,
	ErrUserMuted:                     1607,
	//
	ErrWrongPassword:           2001,
	ErrUsernameOrPasswordWrong: 2002,
	ErrDifferentPassword:       2003,
	ErrSamePassword:            2004,
	ErrNoBlocked:               2005,
	//
	ErrAlreadyFriend:  3001,
	ErrBlocked:        3002,
	ErrAlreadyRequest: 3003,
	ErrAlreadyBlock:   3004,
	ErrNoRequest:      3005,
	//
	ErrAlreadyInGroup:       4001,
	ErrJoinGroup:            4002,
	ErrGroupNotFound:        4003,
	ErrNoGroup:              4004,
	ErrInvalidRole:          4005,
	ErrInvalidParams:        4006,
	ErrCantBanAdmin:         4007,
	ErrNotInGroup:           4008,
	ErrPageSizeTooSmall:     4009,
	ErrAlreadyApply:         4010,
	ErrNoApplied:            4011,
	ErrJoinGroupFailed:      4012,
	ErrNoInvite:             4013,
	ErrAlreadyAdmin:         4014,
	ErrAlreadyMember:        4015,
	ErrNotExpectedRole:      4016,
	ErrOwnerCantLeave:       4017,
	ErrCantSetMyselfAdmin:   4018,
	ErrHandOverOwnerFirst:   4019,
	ErrNotAdmin:             4020,
	ErrInvitationHasExpired: 4021,
	ErrCantKickAdmin:        4022,
	ErrNotInApplyList:       4023,
	ErrCantSearchNull:       4024,
	ErrPageSizeTooBig:       4026,

	ErrUnkonwnMessageType: 5000,
}

func GetStatusCode(err error) int {
	if code, ok := StatusCode[err]; ok {
		return code
	}
	return 500
}

// 系统消息
const (
	ILLEGAL_REQUEST = "非法请求"

	GET_WAITTING_SEND_MSG_FAILED  = "获取待发送消息失败: %v"
	GET_GROUP_FAILED              = "获取群聊列表失败: %v"
	GET_GROUP_PERSONS_FAIL        = "获取群聊成员失败: %v"
	JOIN_GROUP_FAILED             = "加入群聊失败: %v"
	LEAVE_GROUP_FAILED            = "退出群聊失败: %v"
	GROUPID_INIT_FAILED           = "群聊ID初始化失败: %v"
	GET_GROUPID_FAILED            = "获取群聊ID失败: %v"
	CREATE_GROUP_FAILED           = "创建群聊失败: %v"
	DESTROY_GROUP_FAILED          = "解散群聊失败: %v"
	CHECK_USER_JOINED_FAILED      = "检查用户是否已加入群聊失败: %v"
	CHECK_USER_EXIST_FAILED       = "查找用户失败: %v"
	GET_GROUP_NAME_FAIL           = "获取群聊名称失败: %v"
	SEND_FRIEND_REQUEST_FAILED    = "发送好友请求失败: %v"
	ADD_FRIEND_FAILED             = "添加好友失败: %v"
	GET_MAX_UID_FAILED            = "获取UID失败: %v"
	SET_MAX_UID_FAILED            = "设置UID失败: %v"
	GET_FRIENDS_FAILED            = "获取好友列表失败: %v"
	GET_CHAT_NOW_USER_INFO_FAILED = "获取当前聊天用户信息失败: %v"
	DELETE_FRIEND_FAILED_SYSTEM   = "删除好友失败： %v"
	BLACKLIST_FAILED              = "添加黑名单失败： %v"

	// 好友
	SEND_FRIEND_REQUEST          = "已发送好友请求"
	ALREADY_IS_FRIEND            = "已经是好友"
	FRIEND_REQUEST_FAILED        = "添加%s为好友失败,请重试"
	ADD_FRIEND                   = "添加了%s为好友"
	ACCEPT_FRIEND_REQUEST_A      = "接受了%s的好友申请"
	ACCEPT_FRIEND_REQUEST_B      = "%s接受了你的好友申请"
	REJECT_FRIEND_REQUEST_FAILED = "拒绝%s的好友请求失败,系统正在重试"
	REJECT_FRIEND_REQUEST        = "拒绝了%s的好友请求"
	DELETE_FRIEND_FAILED         = "删除好友%s失败,请重试"
	DELETE_FRIEND_SUCCESS        = "删除了好友%s"
	ADD_BLACKLIST_FAILED         = "添加%s到黑名单失败,请重试"
	ADD_BLACKLIST_SUCCESS        = "添加%s到黑名单"
)

var (
	ErrFail                     = errors.New("fail")
	ErrUnexpected               = errors.New("unexpected")
	ErrDuplicateEntry           = errors.New("duplicate entry")
	ErrDuplicateEntryPhone      = errors.New("duplicate entry: phone")
	ErrDuplicateEntryEmail      = errors.New("duplicate entry: email")
	ErrForeignKeyViolated       = errors.New("foreign key violated")
	ErrForeignKeyViolatedMember = errors.New("foreign key violated: memberid")
	ErrForeignKeyViolatedGroup  = errors.New("foreign key violated: groupid")
	ErrRecordNotFound           = errors.New("record not found")
	ErrNoAffectedRows           = errors.New("no affected rows")
	ErrMemberNotFound           = errors.New("member not found")
	ErrWrongTokenType           = errors.New("wrong token type")
	ErrPermissionDenied         = errors.New("permission denied")
	ErrNilTransaction           = errors.New("nil transaction")
	ErrGetFromContext           = errors.New("get from context")
	ErrNotRunning               = errors.New("server not running")
	ErrTimeout                  = errors.New("timeout")
	ErrConnectionReset          = errors.New("conntection reset")
	ErrConnectionClosed         = errors.New("connection closed")
)

func HandleError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRecordNotFound
	}
	errStr := err.Error()
	if strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "invalid connection") ||
		strings.Contains(errStr, "timeout") {
		return ErrOperactionTimeout
	}
	if e, ok := err.(*mysql.MySQLError); ok {
		switch e.Number {
		case 1062:
			if strings.Contains(e.Message, "uni_user_phone") {
				return ErrDuplicateEntryPhone
			}
			if strings.Contains(e.Message, "uni_user_email") {
				return ErrDuplicateEntryEmail
			}
			return ErrDuplicateEntry
		case 1451: // delete failed
			if strings.Contains(e.Message, "owner") {
				return ErrForeignKeyViolatedGroup
			}
		case 1452: // insert failed
			if strings.Contains(e.Message, "owner") {
				return ErrForeignKeyViolated
			}
			if strings.Contains(e.Message, "memberid") {
				return ErrForeignKeyViolatedMember
			}
			if strings.Contains(e.Message, "groupid") {
				return ErrForeignKeyViolatedGroup
			}
		case 1040:
			time.Sleep(3 * time.Second)
			return ErrSystemBusy
		}
	}
	return ErrFailed
}
