package service_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/farnese17/chat/service/mock"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	ws "github.com/farnese17/chat/websocket"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGroupCreate(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		group    *model.Group
		mock     error
		expected error
		mockAll  bool
	}{
		{&model.Group{GID: gid, Name: strings.Repeat("a", 25), Owner: uid, Founder: uid},
			nil,
			errors.New("群组名称长度不能超过20个字符"), false},
		{&model.Group{GID: gid, Name: "test", Owner: uid, Founder: uid},
			errorsx.ErrForeignKeyViolated,
			errorsx.ErrUserNotExist, true},
		{&model.Group{GID: gid, Name: "test", Owner: uid, Founder: uid},
			errorsx.HandleError(errors.New("error")),
			errorsx.ErrFailed, true},
		{&model.Group{GID: gid, Name: "test", Owner: uid, Founder: uid}, nil, nil, true},
		{&model.Group{GID: gid, Name: strings.Repeat("a", 20), Owner: uid, Founder: uid}, nil, nil, true},
	}
	for i, tt := range tests {
		if tt.mockAll {
			mockg.EXPECT().Create(tt.group).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().Remove(gomock.Any())
		}
		t.Run(fmt.Sprintf("create group: %d", i), func(t *testing.T) {
			result, err := g.Create(tt.group)
			assert.Equal(t, tt.expected, err)
			if tt.expected != nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.group, result)
			}
		})
	}
}

func TestGroupSearchByID(t *testing.T) {
	setup(t)
	defer clear(t)

	group := &model.Group{GID: gid, Name: "test", Owner: uid, Founder: uid}
	tests := []struct {
		gid      uint
		mockData *model.Group
		mockErr  error
		expected error
	}{
		{gid, nil, errorsx.ErrRecordNotFound, errorsx.ErrGroupNotFound},
		{gid, nil, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{gid, group, nil, nil},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("search group by id: %d", i), func(t *testing.T) {
			mockg.EXPECT().SearchByID(tt.gid).Return(tt.mockData, tt.mockErr)
			group, err := g.SearchByID(gid)
			assert.Equal(t, tt.expected, err)
			assert.Equal(t, tt.mockData, group)
		})
	}
}

func TestGroupSearchByName(t *testing.T) {
	setup(t)
	defer clear(t)

	groups := []*model.Group{{GID: gid, Name: "test", Owner: uid, Founder: uid, Desc: "", CreatedAt: time.Now().Unix()}}
	tests := []struct {
		name     string
		cursor   *model.Cursor
		mock     error
		expected error
		mockAll  bool
	}{
		{"test", &model.Cursor{PageSize: 0}, nil, errorsx.ErrPageSizeTooSmall, false},
		{"test", &model.Cursor{PageSize: 31}, nil, errorsx.ErrPageSizeTooBig, false},
		{"test", &model.Cursor{PageSize: 15}, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{"test", &model.Cursor{PageSize: 15}, nil, nil, true},
	}
	for i, tt := range tests {
		if tt.mockAll {
			mockg.EXPECT().SearchByName(tt.name, tt.cursor).Return(groups, tt.cursor, tt.mock)
		}
		t.Run(fmt.Sprintf("search group by name %d", i), func(t *testing.T) {
			result, err := g.SearchByName(tt.name, tt.cursor)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				assert.Equal(t, groups, result["groups"])
				assert.Equal(t, tt.cursor, result["cursor"])
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		gid      uint
		from     uint
		column   string
		value    string
		mock     error
		expected error
	}{
		{gid, uid, "wrong column", "", nil, errorsx.ErrInvalidParams},
		{gid, uid, "name", "", errorsx.ErrNoAffectedRows, errorsx.ErrPermissiondenied},
		{gid, uid, "desc", "", errorsx.ErrNoAffectedRows, errorsx.ErrPermissiondenied},
		{gid, uid, "name", "", errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{gid, uid, "desc", "", errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{gid, uid, "name", "", nil, nil},
		{gid, uid, "desc", "", nil, nil},
	}

	for i, tt := range tests {
		if i > 0 {
			mockg.EXPECT().Update(tt.from, tt.gid, tt.column, tt.value).Return(tt.mock)
		}
		t.Run(fmt.Sprintf("update group information %d", i), func(t *testing.T) {
			err := g.Update(tt.from, tt.gid, tt.column, tt.value)
			assert.Equal(t, tt.expected, err)
		})
	}
}

func TestDelete(t *testing.T) {
	setup(t)
	defer clear(t)

	message := &ws.ChatMsg{
		Type:  ws.System,
		Body:  "该群聊已解散",
		From:  uid,
		To:    gid,
		Extra: []uint{uid, uid + 1},
	}
	cache := []uint{100001, 100002}

	tests := []struct {
		name       string
		mockc      []uint
		mockcErr   error
		mockDelete error
		expected   error
	}{
		{"get cache failed", nil, errorsx.HandleError(errors.New("error")), nil, errorsx.ErrFailed},
		{"members are null", []uint{}, nil, nil, errorsx.ErrGroupNotFound},
		{"delete failed", cache, nil, errorsx.ErrNoAffectedRows, errorsx.ErrPermissiondenied},
		{"delete failed", cache, nil, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{"delete successed", cache, nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockc.EXPECT().GetMembers(gomock.Any()).Return(tt.mockc, tt.mockcErr)
			if tt.mockcErr == nil && len(tt.mockc) > 0 {
				mockg.EXPECT().Delete(gid, uid).Return(tt.mockDelete)
			}
			if tt.expected == nil {
				mockc.EXPECT().Remove(gomock.Any()).Return()
				mockc.EXPECT().RemoveGroupLastActiveTime(gid).Return()
			}
			err := g.Delete(gid, uid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message.Time = msg.Time
				assert.Equal(t, message, msg)
			}
		})
	}

	t.Run("Delete group: Hit cache", func(t *testing.T) {
		mockc.EXPECT().GetMembers(gomock.Any()).Return(cache, nil)
		mockg.EXPECT().Delete(gid, uid).Return(nil)
		mockc.EXPECT().Remove(gomock.Any()).Return()
		mockc.EXPECT().RemoveGroupLastActiveTime(gid).Return()
		err := g.Delete(gid, uid)
		assert.NoError(t, err)
		msg := <-mock.Message
		message.Time = msg.Time
		assert.Equal(t, message, msg)
	})
}

func TestMembers(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.MemberInfo{
		{
			ID:         uid,
			Username:   "test1",
			Phone:      "15815815815",
			Email:      "test1@test.com",
			Avatar:     "",
			Role:       1,
			Created_at: time.Now().Unix(),
		},
		{
			ID:         uid + 1,
			Username:   "test2",
			Phone:      "15815815816",
			Email:      "test2@test.com",
			Avatar:     "",
			Role:       3,
			Created_at: time.Now().Unix(),
		},
	}

	tests := []struct {
		gid         uint
		mock        []*model.MemberInfo
		mockErr     error
		expected    []*model.MemberInfo
		expectedErr error
	}{
		{gid, members[:0], errorsx.HandleError(errors.New("error")), nil, errorsx.ErrFailed},
		{gid, members[:0], nil, nil, errorsx.ErrGroupNotFound},
		{gid, members, nil, members, nil},
	}

	for i, tt := range tests {
		mockg.EXPECT().Members(tt.gid, 0, -1).Return(tt.mock, tt.mockErr)
		t.Run(fmt.Sprintf("get members %d", i), func(t *testing.T) {
			data, err := g.Members(tt.gid)
			assert.Equal(t, tt.expected, data)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestMember(t *testing.T) {
	setup(t)
	defer clear(t)

	member := []*model.MemberInfo{{ID: uid, Username: "test1", Role: model.GroupRoleMember}}
	tests := []struct {
		gid         uint
		uid         uint
		mock        []*model.MemberInfo
		mockErr     error
		expected    *model.MemberInfo
		expectedErr error
	}{
		{gid, uid, nil, errorsx.HandleError(errors.New("error")), nil, errorsx.ErrFailed},
		{gid, uid, nil, nil, nil, errorsx.ErrNotInGroup},
		{gid, uid, member, nil, member[0], nil},
	}

	for i, tt := range tests {
		mockg.EXPECT().Members(tt.gid, tt.uid, 1).Return(tt.mock, tt.mockErr)
		t.Run(fmt.Sprintf("get member %d", i), func(t *testing.T) {
			data, err := g.Member(tt.uid, tt.gid)
			assert.Equal(t, tt.expected, data)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestQueryRole(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1", Role: model.GroupRoleOwner},
		{MemberID: uid + 1, Username: "test2", Role: model.GroupRoleMember},
	}
	tests := []struct {
		ctx         *model.MemberStatusContext
		mock        []*model.GroupMemberRole
		mockErr     error
		expected    *model.MemberStatusContext
		expectedErr error
		mockAll     bool
	}{
		{&model.MemberStatusContext{GID: gid - 1, From: uid, To: uid + 1},
			nil, nil, nil, errorsx.ErrInvalidParams, false},
		{&model.MemberStatusContext{GID: gid, From: uid - 1, To: uid + 1},
			nil, nil, nil, errorsx.ErrInvalidParams, false},
		{&model.MemberStatusContext{GID: gid, From: uid, To: uid + 1},
			nil, errorsx.HandleError(errors.New("error")), nil, errorsx.ErrFailed, true},
		{&model.MemberStatusContext{GID: gid, From: uid, To: uid + 1},
			members[:0], nil,
			&model.MemberStatusContext{GID: gid, From: uid, To: uid + 1, Data: make(map[uint]*model.GroupMemberRole)}, errorsx.ErrUserNotExist, true},
		{&model.MemberStatusContext{GID: gid, From: uid, To: uid + 1},
			members[:1], nil, &model.MemberStatusContext{GID: gid, From: uid, To: uid + 1, Data: map[uint]*model.GroupMemberRole{uid: members[0]}}, errorsx.ErrUserNotExist, true},
		{&model.MemberStatusContext{GID: gid, From: uid, To: uid + 1},
			members, nil, &model.MemberStatusContext{GID: gid, From: uid, To: uid + 1, Data: map[uint]*model.GroupMemberRole{uid: members[0], uid + 1: members[1]}}, nil, true},
		{&model.MemberStatusContext{GID: gid, From: uid, To: uid},
			members[:1], nil, &model.MemberStatusContext{GID: gid, From: uid, To: uid, Data: map[uint]*model.GroupMemberRole{uid: members[0]}}, errorsx.ErrUserNotExist, true},
	}

	for i, tt := range tests {
		if tt.mockAll {
			mockg.EXPECT().QueryRole(gomock.Any(), gomock.Any()).Return(tt.mock, tt.mockErr)
		}
		t.Run(fmt.Sprintf("query role %d", i), func(t *testing.T) {
			data, err := g.QueryRole(tt.ctx)
			assert.Equal(t, tt.expected, data)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestInvite(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		fromStatus int
		toStatus   int
		length     int
		mock       error
		expected   error
	}{
		{0, 0, 0, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed},
		{0, 0, 0, nil, errorsx.ErrUserNotExist},
		{model.GroupRoleMember, 0, 1, nil, errorsx.ErrUserNotExist},
		{model.GroupRoleOwner, model.GroupRoleOwner, 2, nil, errorsx.ErrAlreadyInGroup},
		{model.GroupRoleOwner, model.GroupRoleAdmin, 2, nil, errorsx.ErrAlreadyInGroup},
		{model.GroupRoleOwner, model.GroupRoleMember, 2, nil, errorsx.ErrAlreadyInGroup},
		{model.GroupRoleOwner, 0, 2, nil, nil},
		{model.GroupRoleOwner, model.GroupRoleBan, 2, nil, nil},
	}

	for i, tt := range tests {
		members[0].Role = tt.fromStatus
		members[1].Role = tt.toStatus
		mockg.EXPECT().QueryRole(gomock.Any(), gomock.Any()).Return(members[:tt.length], tt.mock)
		t.Run(fmt.Sprintf("invite %d", i), func(t *testing.T) {
			msg, err := g.Invite(uid, uid+1, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				message := &ws.ChatMsg{
					Type: ws.System,
					From: uid,
					To:   uid + 1,
					Time: msg.Time,
					Body: "邀请 test2 加入群聊 (1000000001)",
				}
				assert.Equal(t, message, msg)
				msg := <-mock.Message
				message.Time = msg.Time
				message.Body = "test1 邀请你加入群聊 (1000000001)"
				message.Extra = gid
				assert.Equal(t, message, msg)
			} else {
				assert.Nil(t, msg)
			}
		})
	}

	members[0].Role = model.GroupRoleOwner
	members[1].Role = model.GroupRoleApplied
	mockg.EXPECT().QueryRole(gomock.Any(), gomock.Any()).Return(members, nil).Times(2)
	t.Run("invite failed", func(t *testing.T) {
		mockg.EXPECT().UpdateStatus(gomock.Any()).Return(errorsx.HandleError(errors.New("error")))
		msg, err := g.Invite(uid, uid+1, gid)
		assert.Equal(t, errorsx.ErrFailed, err)
		assert.Nil(t, msg)
	})
	t.Run("invite", func(t *testing.T) {
		mockg.EXPECT().UpdateStatus(gomock.Any()).Return(nil)
		mockc.EXPECT().AddMemberIfKeyExist(gid, uid+1, gomock.Any())
		msg, err := g.Invite(uid, uid+1, gid)
		assert.Nil(t, err)
		assert.Nil(t, msg)
		msg = <-mock.Message
		message := &ws.ChatMsg{
			Type: ws.System,
			To:   gid,
			Time: msg.Time,
			Body: "test1 通过了 test2 的申请",
		}
		assert.Equal(t, message, msg)
	})
}

func TestApply(t *testing.T) {
	setup(t)
	defer clear(t)

	member := []*model.GroupMemberRole{
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		status  int
		mock    error
		expeced error
		mockAll bool
	}{
		{model.GroupRoleOwner, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleAdmin, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleMember, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleBan, nil, errorsx.ErrBanned, false},
		{-9999, nil, errorsx.ErrInvalidParams, false},
		{0, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleApplied, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleApplied, errorsx.ErrNoAffectedRows, errorsx.ErrFailed, true},
		{model.GroupRoleApplied, nil, nil, true},
		{0, nil, nil, true},
	}

	for i, tt := range tests {
		member[0].Role = tt.status
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(member, nil)
		if tt.mockAll {
			if tt.status == model.GroupRoleApplied {
				mockg.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
			} else {
				mockg.EXPECT().CreateMember(gomock.Any()).Return(tt.mock)
			}
		}
		t.Run(fmt.Sprintf("apply %d", i), func(t *testing.T) {
			err := g.Apply(gid, uid+1)
			assert.Equal(t, tt.expeced, err)
			if tt.expeced == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.Apply,
					Time: msg.Time,
					To:   gid,
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestAcceptInvite(t *testing.T) {
	setup(t)
	defer clear(t)

	msg := ws.ChatMsg{
		Type: ws.System,
		From: uid,
		Time: time.Now().UnixMilli(),
		To:   uid + 1,
	}

	testValidateMessage := []struct {
		time     int64
		expected error
	}{
		// 过期
		{0, errorsx.ErrInvitationHasExpired},
		// 来自未来
		{time.Now().Add(time.Minute).UnixMilli(), errorsx.ErrInvitationHasExpired},
		// 过期
		{time.Now().Add(-(time.Duration(cfg.Common().InviteValidDays()*24)*time.Hour + time.Minute)).UnixMilli(), errorsx.ErrInvitationHasExpired},
		// 有效
		{time.Now().Add(-time.Minute).UnixMilli(), errorsx.ErrInvalidParams}, // message valid
	}

	for i, tt := range testValidateMessage {
		msg.Time = tt.time
		t.Run(fmt.Sprintf("message expired %d", i), func(t *testing.T) {
			err := g.AcceptInvite(msg)
			assert.Equal(t, tt.expected, err)
		})
	}

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}
	msg = ws.ChatMsg{
		Type:  ws.System,
		From:  uid,
		Time:  time.Now().Add(-time.Second).UnixMilli(),
		To:    uid + 1,
		Extra: float64(gid),
	}

	tests := []struct {
		fstatus  int
		tstatus  int
		mock     error
		expected error
		mockAll  bool
	}{
		{0, 0, nil, errorsx.ErrInvitationHasExpired, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleBan, nil, errorsx.ErrBanned, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleApplied, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleInvited, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, model.GroupRoleInvited, nil, nil, true},
		{model.GroupRoleOwner, 0, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, 0, nil, nil, true},
		{model.GroupRoleAdmin, 0, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.fstatus
		members[1].Role = tt.tstatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll && tt.tstatus == 0 {
			mockg.EXPECT().CreateMember(gomock.Any()).Return(tt.mock)
		} else if tt.mockAll {
			mockg.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().AddMemberIfKeyExist(gid, msg.To, gomock.Any())
		}
		t.Run(fmt.Sprintf("accept invite %d", i), func(t *testing.T) {
			err := g.AcceptInvite(msg)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					Time: msg.Time,
					To:   gid,
				}
				if tt.tstatus == 0 {
					message.Body = "test1 邀请 test2 加入群聊"
				} else {
					message.Body = "test1 通过了 test2 的申请"
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestAcceptApply(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		fstatus, tstatus int
		mock             error
		expected         error
		mockAll          bool
	}{
		{0, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleBan, nil, errorsx.ErrBanned, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, 0, nil, errorsx.ErrNotInApplyList, false},
		{model.GroupRoleOwner, model.GroupRoleApplied, nil, nil, true},
		{model.GroupRoleAdmin, model.GroupRoleApplied, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.fstatus
		members[1].Role = tt.tstatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().AddMemberIfKeyExist(gid, uid+1, gomock.Any())
		}
		t.Run(fmt.Sprintf("accept apply %d", i), func(t *testing.T) {
			err := g.AcceptApply(uid, uid+1, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test1 通过了 test2 的申请",
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestRejectApply(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		fstatus, tstatus int
		mock             error
		expected         error
		mockAll          bool
	}{
		{0, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleOwner, model.GroupRoleBan, nil, errorsx.ErrBanned, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, errorsx.ErrAlreadyInGroup, false},
		{model.GroupRoleOwner, 0, nil, nil, false},
		{model.GroupRoleOwner, model.GroupRoleApplied, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, model.GroupRoleApplied, nil, nil, true},
		{model.GroupRoleAdmin, model.GroupRoleApplied, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.fstatus
		members[1].Role = tt.tstatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().DeleteMember(gomock.Any()).Return(tt.mock)
		}
		t.Run(fmt.Sprintf("reject apply %d", i), func(t *testing.T) {
			err := g.RejectApply(uid, uid+1, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil && tt.mockAll {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					From: uid,
					To:   uid + 1,
					Time: msg.Time,
					Body: "test1 拒绝了你的请求",
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestLeave(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		status   int
		mock     error
		expected error
		mockAll  bool
	}{
		{0, nil, errorsx.ErrNotInGroup, false},
		{999, nil, errorsx.ErrInvalidParams, false},
		{-999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleInvited, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, nil, errorsx.ErrOwnerCantLeave, false},
		{model.GroupRoleBan, nil, errorsx.ErrBanned, false},
		{model.GroupRoleMember, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleMember, nil, nil, true},
		{model.GroupRoleAdmin, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.status
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().DeleteMember(gomock.Any()).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().RemoveMember(gid, uid+1)
		}
		t.Run(fmt.Sprintf("leave %d", i), func(t *testing.T) {
			err := g.Leave(uid+1, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil && tt.mockAll {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test2 退出了群聊",
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestKick(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		fStatus, tStatus int
		mock             error
		expected         error
		mockAll          bool
	}{
		{0, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrCantKickAdmin, false},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, errorsx.ErrCantKickAdmin, false},
		{model.GroupRoleOwner, 0, nil, nil, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleInvited, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleApplied, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleMember, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, nil, true},
		{model.GroupRoleAdmin, model.GroupRoleMember, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.fStatus
		members[1].Role = tt.tStatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().DeleteMember(gomock.Any()).Return(tt.mock)
			if tt.expected == nil {
				mockc.EXPECT().RemoveMember(gid, uid+1)
			}
		}

		t.Run(fmt.Sprintf("kick %d", i), func(t *testing.T) {
			err := g.Kick(uid, uid+1, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil && tt.mockAll {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test1 将 test2 踢出了群聊",
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestHandOverOwner(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}

	tests := []struct {
		fStatus, tStatus int
		mock             error
		expected         error
		mockAll          bool
	}{
		{0, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleAdmin, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleOwner, 0, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleOwner, model.GroupRoleMember, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, model.GroupRoleMember, errorsx.ErrPermissionDenied, errorsx.ErrPermissiondenied, true},
		{model.GroupRoleOwner, model.GroupRoleMember, errorsx.ErrNoAffectedRows, errorsx.ErrNotInGroup, true},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, nil, true},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.fStatus
		members[1].Role = tt.tStatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().HandOverOwner(uid, uid+1, gid).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().AddMemberIfKeyExist(gid, uid, gomock.Any())
			mockc.EXPECT().AddMemberIfKeyExist(gid, uid+1, gomock.Any())
		}
		t.Run(fmt.Sprintf("hand over owner %d", i), func(t *testing.T) {
			err := g.HandOverOwner(uid, uid+1, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test2 成为了新的群主",
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestModifyAdmin(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{
		{MemberID: uid, Username: "test1"},
		{MemberID: uid + 1, Username: "test2"},
	}

	testSetAdmin := []struct {
		fStatus, tStatus int
		mock             error
		expected         error
		mockAll          bool
	}{
		{0, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleAdmin, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleMember, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, 0, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleBan, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleApplied, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleInvited, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, errorsx.ErrAlreadyAdmin, false},
		{model.GroupRoleOwner, model.GroupRoleMember, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, nil, true},
	}

	for i, tt := range testSetAdmin {
		members[0].Role = tt.fStatus
		members[1].Role = tt.tStatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().AddMemberIfKeyExist(gid, uid+1, gomock.Any())
		}
		t.Run(fmt.Sprintf("test set admin %d", i), func(t *testing.T) {
			err := g.ModifyAdmin(uid, uid+1, gid, model.GroupRoleAdmin)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test1 将 test2 设置为管理员",
				}
				assert.Equal(t, message, msg)
			}
		})
	}

	testRemoveAdmin := []struct {
		fStatus, tStatus int
		mock             error
		expected         error
		mockAll          bool
	}{
		{0, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleAdmin, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleMember, 0, nil, errorsx.ErrPermissiondenied, false},
		{model.GroupRoleOwner, model.GroupRoleOwner, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, 0, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, 999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, -999, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleBan, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleApplied, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleInvited, nil, errorsx.ErrInvalidParams, false},
		{model.GroupRoleOwner, model.GroupRoleMember, nil, errorsx.ErrAlreadyMember, false},
		{model.GroupRoleOwner, model.GroupRoleAdmin, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleOwner, model.GroupRoleAdmin, nil, nil, true},
	}

	for i, tt := range testRemoveAdmin {
		members[0].Role = tt.fStatus
		members[1].Role = tt.tStatus
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().AddMemberIfKeyExist(gid, uid+1, gomock.Any())
		}
		t.Run(fmt.Sprintf("test remove admin %d", i), func(t *testing.T) {
			err := g.ModifyAdmin(uid, uid+1, gid, model.GroupRoleMember)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test2 不再担任管理员(test1)",
				}
				assert.Equal(t, message, msg)
			}
		})
	}

	t.Run("modify admin params error", func(t *testing.T) {
		err := g.ModifyAdmin(uid, uid+1, gid, 0)
		assert.Equal(t, errorsx.ErrInvalidParams, err)
	})
}

func TestAdminResign(t *testing.T) {
	setup(t)
	defer clear(t)

	members := []*model.GroupMemberRole{{MemberID: uid, Username: "test1"}}

	tests := []struct {
		status   int
		mock     error
		expected error
		mockAll  bool
	}{
		{999, nil, errorsx.ErrInvalidParams, false},
		{-999, nil, errorsx.ErrInvalidParams, false},
		{0, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleApplied, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleInvited, nil, errorsx.ErrNotInGroup, false},
		{model.GroupRoleOwner, nil, errorsx.ErrHandOverOwnerFirst, false},
		{model.GroupRoleMember, nil, errorsx.ErrNotAdmin, false},
		{model.GroupRoleAdmin, errorsx.HandleError(errors.New("error")), errorsx.ErrFailed, true},
		{model.GroupRoleAdmin, nil, nil, true},
	}

	for i, tt := range tests {
		members[0].Role = tt.status
		mockg.EXPECT().QueryRole(gid, gomock.Any()).Return(members, nil)
		if tt.mockAll {
			mockg.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
		}
		if tt.expected == nil {
			mockc.EXPECT().AddMemberIfKeyExist(gid, uid, gomock.Any())
		}
		t.Run(fmt.Sprintf("admin resign %d", i), func(t *testing.T) {
			err := g.AdminResign(uid, gid)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					To:   gid,
					Time: msg.Time,
					Body: "test1 不再担任管理员(test1)",
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestWS(t *testing.T) {
	setup(t)
	defer clear(t)

	message := &ws.ChatMsg{
		Type: ws.System,
		Body: "test2 退出了群聊",
		To:   gid,
	}
	wsHub := mock.NewMockHub()
	wsHub.SendToBroadcast(message)
	msg := <-mock.Message
	assert.Equal(t, message, msg)
}
