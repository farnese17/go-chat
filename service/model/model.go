package model

import (
	"gorm.io/gorm"
)

// user model
type User struct {
	// gorm.Model
	ID          uint           `json:"id" gorm:"primarykey"`
	CreatedAt   int64          `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index;column:deleted_at"`
	Username    string         `json:"username" gorm:"type:varchar(8);not null" validate:"required,min=2,max=8,username" label:"用户名"`
	Password    string         `json:"password" gorm:"type:varchar(64);not null" validate:"required,pwlength,nospace" label:"密码" `
	Phone       string         `json:"phone" gorm:"type:varchar(11);unique;default:null;index" validate:"omitempty,mobile" label:"手机号"`
	Email       string         `json:"email" gorm:"type:varchar(30);unique;default:null;index" validate:"omitempty,email" label:"邮箱"`
	Avatar      string         `json:"avatar"`
	BanLevel    int            `json:"ban_level" gorm:"type:int;column:ban_level"`
	BanExpireAt int64          `json:"ban_expire_at" gorm:"default:null;column:ban_expire_at"`

	Friend1 []Friend `json:"-" gorm:"foreignKey:User1;references:ID;constraint:OnDelete:CASCADE"`
	Friend2 []Friend `json:"-" gorm:"foreignKey:User2;references:ID;constraint:OnDelete:CASCADE"`

	HasGroups []Group       `json:"-" gorm:"foreignKey:Owner;references:ID"`
	Members   []GroupPerson `json:"-" gorm:"foreignKey:MemberID;references:ID;constraint:OnDelete:CASCADE"`
}

type ResponseUserInfo struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	BanLevel    int    `json:"ban_level"`
	BanExpireAt int64  `json:"ban_expire_at"`
}
type BanStatus struct {
	ID          uint  `json:"id"`
	BanLevel    int   `json:"ben_level"`
	BanExpireAt int64 `json:"ban_expire_at"`
}

const (
	// Ban levels
	BanLevelPermanent = -1 + iota
	BanLevelNone
	BanLevelMuted
	BanLevelNoPost
	BanLevelTemporary
)

const (
	// Group roles
	GroupRoleApplied = -3
	GroupRoleInvited = -2
	GroupRoleBan     = -1
	GroupRoleOwner   = 1
	GroupRoleAdmin   = 2
	GroupRoleMember  = 3
)

type Group struct {
	GID       uint   `json:"gid" gorm:"primarykey;autoincrement;column:gid"`
	Name      string `json:"name" gorm:"type:varchar(20);index" validate:"max=20" label:"群组名称"`
	Owner     uint   `json:"owner" gorm:"not null" validate:"required,uid" label:"id"`
	Founder   uint   `json:"founder" gorm:"not null"`
	Desc      string `json:"desc" gorm:"type:varchar(255)" validate:"max=255"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	LastTime  int64  `json:"last_time" gorm:"autoUpdateTime;column:last_time"`

	Members      []GroupPerson       `json:"-" gorm:"foreignKey:GroupID;references:GID;constraint:OnDelete:CASCADE"`
	Announcement []GroupAnnouncement `json:"-" gorm:"foreignKey:GroupID;references:GID;constraint:OnDelete:CASCADE"`
}
type GroupPerson struct {
	ID        uint  `json:"id" gorm:"primarykey;autoincrement;column:id"`
	MemberID  uint  `json:"member_id" gorm:"not null;column:member_id;uniqueIndex:idx_member" validate:"required,uid"`
	GroupID   uint  `json:"group_id" gorm:"not null;column:group_id;uniqueIndex:idx_member"`
	Role      int   `json:"role" gorm:"type:int;default:3"`
	InviterID uint  `json:"inviter_id" gorm:"column:inviter_id" validate:"omitempty,uid"`
	CreatedAt int64 `json:"created_at" gorm:"autoCreatTime"`
	Version   int   `gorm:"type:int;default:0"`
}
type GroupAnnouncement struct {
	// gorm.Model
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt int64          `json:"created_at" gorm:"autoCreatTime"`
	UpdatedAt int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	GroupID   uint           `json:"group_id" gorm:"index;column:group_id"`
	Content   string         `json:"content" gorm:"type:text"`
	CreatedBy uint           `json:"created_by" gorm:"column:created_by"`
}

// 成员信息
type MemberInfo struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	Role        int    `json:"role"`
	Created_at  int64  `json:"created_at"`
	BanLevel    int    `json:"ban_level"`
	BanExpireAt int64  `json:"ban_expire_at"`
}

type SummaryGroupInfo struct {
	GID       uint   `json:"gid" gorm:"column:gid"`
	GroupName string `json:"groupname" gorm:"column:groupname"`
}

type GroupLastActiveTime struct {
	GID      uint  `json:"gid" gorm:"column:gid"`
	LastTime int64 `json:"last_time"`
}

type GroupMemberRole struct {
	ID        uint
	MemberID  uint `gorm:"column:member_id"`
	Role      int
	Username  string
	Groupname string `gorm:"column:groupname"`
	Version   int    `gorm:"column:version"`
}

type GroupAnnounceInfo struct {
	ID        uint   `json:"id"`
	Content   string `json:"content"`
	CreatedBy string `json:"created_by"`
	UpdatedAt int64  `json:"updated_at"`
}

const (
	// friend status
	RejectFriend  = -2 + iota // only flag
	UnblockFriend             // only flag
	FSNull
	FSAdded
	FSReq1To2
	FSReq2To1
	FSBlock1To2
	FSBlock2To1
	FSBothBlocked
)

type Friend struct {
	ID          uint   `json:"id" gorm:"primarykey;autoincrement;column:id"`
	User1       uint   `json:"user1" gorm:"column:user1;not null;uniqueindex:idx_user1_user2"`
	User2       uint   `json:"user2" gorm:"column:user2;not null;uniqueindex:idx_user1_user2"`
	User1Remark string `json:"user1_remark" gorm:"type:varchar(10)"`
	User2Remark string `json:"user2_remark" gorm:"type:varchar(10)"`
	User1Group  string `json:"user1_group" gorm:"type:varchar(10)"`
	User2Group  string `json:"user2_group" gorm:"type:varchar(10)"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreatTime"`
	Status      int    `json:"status" gorm:"type:int;not null"`
	Version     int    `json:"version" gorm:"default:0"`
}

// 好友列表结构
type Friendinfo struct {
	ID          uint   `json:"id"`
	UID         uint   `json:"uid"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Remark      string `json:"remark"`
	Group       string `json:"group"`
	Status      int    `json:"status"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Created_at  int64  `json:"created_at"`
	BanLevel    int    `json:"ban_level"`
	BanExpireAt int64  `json:"ban_expire_at"`
}

// 好友列表结构
type SummaryFriendInfo struct {
	ID          uint   `json:"id"`
	UID         uint   `json:"uid"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Remark      string `json:"remark"`
	Group       string `json:"group"`
	Status      int    `json:"status"`
	BanLevel    int    `json:"ban_level"`
	BanExpireAt int64  `json:"ban_expire_at"`
}

// type contextKey int

// const (
// 	CancelKey contextKey = iota
// 	TxKey
// 	RollbackKey
// 	DataKey
// 	ExpectedRoleKey
// 	CurrentRoleKey
// )

type Cursor struct {
	PageSize int  `json:"page_size"`
	LastID   uint `json:"last_id"`
	HasMore  bool `json:"has_more"`
}

const (
	CacheMessage    = "chat:cache:message:"
	CacheBlockUnack = "chat:block:unack"
	CacheGroup      = "chat:cache:group:"
	CacheGroups     = "chat:cache:groups"

	CacheToken  = "chat:token:"
	CacheBanned = "chat:banned:"

	CacheLatestWarmTime = "chat:cache:latest_warm"
)

type MemberStatusContext struct {
	GID       uint
	From      uint
	To        uint
	NewStatus int
	NoStatus  bool
	Data      map[uint]*GroupMemberRole
}

type Manager struct {
	ID          uint   `json:"id" gorm:"primarykey;autoincrement"`
	Created_At  int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	Updated_At  int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	Deleted_At  int64  `json:"deleted_at" gorm:"default:null;column:deleted_at"`
	Permissions uint   `json:"permissions" gorm:"default:4;column:permissions"`
	Username    string `json:"username" gorm:"not null;column:username"`
	Password    string `json:"password" gorm:"not null;column:password"`
	Email       string `json:"email" gorm:"not null;column:email"`
}

// manager permissions
const (
	MgrSuperAdministrator uint = 7
	MgrWriteAndRead       uint = 6
	MgrOnlyRead           uint = 4
)
