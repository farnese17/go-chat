package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"slices"

	"github.com/farnese17/chat/registry"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/validator"
	ws "github.com/farnese17/chat/websocket"
	"go.uber.org/zap"
)

const (
	inviteMsg        = "邀请 %s 加入群聊 %s"
	invitedMsg       = "%s 邀请你加入群聊 %s"
	joinedMsg        = "%s 通过了 %s 的申请"
	acceptInviteMsg  = "%s 邀请 %s 加入群聊"
	rejectApply      = "%s 拒绝了你的请求"
	deleteGroupMsg   = "该群聊已解散"
	setAdminMsg      = "%s 将 %s 设置为管理员"
	leaveMsg         = "%s 退出了群聊"
	kickMsg          = "%s 将 %s 踢出了群聊"
	handOverOwnerMsg = "%s 成为了新的群主"
	removeAdminMsg   = "%s 不再担任管理员(%s)"
)

type memberOperation int

const (
	invite memberOperation = iota
	apply
	acceptInvite
	handleApply
	handleLeave
	handleKick
	setAdmin
	removeAdmin
	adminResign
	handOverOwner
)

type GroupService struct {
	service registry.Service
}

func NewGroupService(s registry.Service) *GroupService {
	return &GroupService{s}
}

// 创建群组
func (g *GroupService) Create(group *m.Group) (*m.Group, error) {
	if err := validator.Validate(group); err != nil {
		g.service.Logger().Warn("Failed to create group: invalid JSON", zap.Error(err))
		return nil, err
	}
	if err := g.service.Group().Create(group); err != nil {
		if errors.Is(err, errorsx.ErrForeignKeyViolated) {
			g.service.Logger().Warn("Created group failed: user not exsit", zap.Uint("id", group.Owner))
			return nil, errorsx.ErrUserNotExist
		}
		g.service.Logger().Error("Failed to create group", zap.Error(err))
		return nil, err
	}

	// 删除缓存，避免空集
	key := m.CacheGroup + strconv.FormatUint(uint64(group.GID), 10)
	g.service.Cache().Remove(key)
	return group, nil
}

// 搜索群组
func (g *GroupService) SearchByID(gid uint) (*m.Group, error) {
	if err := validator.ValidateGID(gid); err != nil {
		return nil, err
	}
	group, err := g.service.Group().SearchByID(gid)
	if err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			return nil, errorsx.ErrGroupNotFound
		}
		g.service.Logger().Error("Failed to search group", zap.Error(err))
		return nil, err
	}
	return group, nil
}

func (g *GroupService) SearchByName(name string, cursor *m.Cursor) (map[string]any, error) {
	if err := validator.VerfityPageSize(cursor.PageSize); err != nil {
		return nil, err
	}

	if err := validator.ValidateGroupname(name); err != nil {
		return nil, err
	}
	groups, cursor, err := g.service.Group().SearchByName(name, cursor)
	if err != nil {
		g.service.Logger().Error("Failed to search by name", zap.Error(err))
		return nil, err
	}
	data := map[string]any{"groups": groups, "cursor": cursor}
	return data, nil
}

// 更新群组信息
func (g *GroupService) Update(from, gid uint, column string, value string) error {
	if err := validator.ValidateGIDAndUID(gid, from); err != nil {
		return errorsx.ErrInvalidParams
	}
	if column != "name" && column != "desc" {
		g.service.Logger().Warn("Failed to update groupo information: invalid column", zap.String("cloumn", column))
		return errorsx.ErrInvalidParams
	}

	if err := g.service.Group().Update(from, gid, column, value); err != nil {
		if errors.Is(err, errorsx.ErrNoAffectedRows) {
			return errorsx.ErrPermissiondenied
		}
		g.service.Logger().Error("Failed to update group information", zap.Error(err))
		return err
		// return err
	}
	return nil
}

// 解散群组
func (g *GroupService) Delete(gid uint, uid uint) error {
	if err := validator.ValidateGIDAndUID(gid, uid); err != nil {
		return errorsx.ErrInvalidParams
	}

	members, err := g.service.Cache().GetMembers(gid)
	if err != nil {
		g.service.Logger().Error("Failed to get members id", zap.Error(err))
		return errorsx.ErrFailed
	}
	if len(members) == 0 {
		g.service.Logger().Error("Failed to delete group: members not found", zap.Error(err))
		return errorsx.ErrGroupNotFound
	}
	if err := g.service.Group().Delete(gid, uid); err != nil {
		if errors.Is(err, errorsx.ErrNoAffectedRows) {
			return errorsx.ErrPermissiondenied
		}
		g.service.Logger().Error("Failed to delete group", zap.Error(err))
		return err
	}

	// 删除缓存
	key := m.CacheGroup + strconv.FormatUint(uint64(gid), 10)
	g.service.Cache().Remove(key)
	g.service.Cache().RemoveGroupLastActiveTime(gid)
	g.service.Logger().Info("finished delete group", zap.Uint("gid", gid), zap.Uint("uid", uid))

	// 广播解散消息
	message := g.newMessage(uid, gid, deleteGroupMsg, members)
	hub := g.service.Hub()
	if hub == nil {
		g.cacheMessage(message, members...)
		return errorsx.ErrHandleSuccessed
	}
	hub.SendDeleteGroupNotify(message)
	return nil
}

// 获取群组列表
func (g *GroupService) List(uid uint) ([]*m.SummaryGroupInfo, error) {
	return g.service.Group().List(uid)
}

// 获取群组成员列表
func (g *GroupService) Members(gid uint) ([]*m.MemberInfo, error) {
	if err := validator.ValidateGID(gid); err != nil {
		return nil, errorsx.ErrInvalidParams
	}
	members, err := g.service.Group().Members(gid, 0, -1)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, errorsx.ErrGroupNotFound
	}
	return members, nil
}

// 获取成员信息
func (g *GroupService) Member(uid uint, gid uint) (*m.MemberInfo, error) {
	if err := validator.ValidateGIDAndUID(gid, uid); err != nil {
		return nil, errorsx.ErrInvalidParams
	}
	member, err := g.service.Group().Members(gid, uid, 1)
	if err != nil {
		g.service.Logger().Error("Get member failed", zap.Error(err))
		return nil, err
	}
	if len(member) == 0 {
		return nil, errorsx.ErrNotInGroup
	}
	return member[0], nil
}

// 邀请加入群组
func (g *GroupService) Invite(from, to, gid uint) (*ws.ChatMsg, error) {
	ctx := &m.MemberStatusContext{
		GID:       gid,
		From:      from,
		To:        to,
		NewStatus: m.GroupRoleInvited,
	}

	if err := g.validateStatus(ctx, invite); err != nil {
		return nil, err
	}
	if ctx.NewStatus == m.GroupRoleMember {
		if err := g.service.Group().UpdateStatus(ctx); err != nil {
			return nil, err
		}
		g.service.Cache().AddMemberIfKeyExist(gid, to, m.GroupRoleMember)
	}
	groupname := ctx.Data[from].Groupname
	if strings.TrimSpace(groupname) == "" {
		groupname += "(" + strconv.FormatInt(int64(gid), 10) + ")"
	}

	var err error
	if ctx.NewStatus == m.GroupRoleInvited {
		body := fmt.Sprintf(invitedMsg, ctx.Data[from].Username, groupname)
		msg := g.newMessage(from, to, body, gid)
		hub := g.service.Hub()
		if hub != nil {
			hub.SendToChat(msg)
		} else {
			g.cacheMessage(msg, to)
			err = errorsx.ErrMessagePushServiceUnavailabel
		}
	} else if ctx.NewStatus == m.GroupRoleMember {
		body := fmt.Sprintf(joinedMsg, ctx.Data[from].Username, ctx.Data[to].Username)
		return nil, g.broadcase(gid, body)
	}
	msg := g.newMessage(ctx.From, ctx.To, fmt.Sprintf(inviteMsg, ctx.Data[to].Username, groupname), nil)
	return msg, err
}

func (g *GroupService) validateStatus(ctx *m.MemberStatusContext, op memberOperation) error {
	ctx, err := g.QueryRole(ctx)
	if err != nil {
		return err
	}
	fStatus, tStatus := 0, 0
	if ctx != nil {
		if ctx.Data[ctx.From] != nil {
			fStatus = ctx.Data[ctx.From].Role
		}
		if ctx.Data[ctx.To] != nil {
			tStatus = ctx.Data[ctx.To].Role
		}
	}
	switch op {
	case invite:
		if fStatus != m.GroupRoleOwner && fStatus != m.GroupRoleAdmin {
			return errorsx.ErrPermissiondenied
		}
		switch tStatus {
		case 0:
			// ctx.NoStatus = true
			return nil
		case m.GroupRoleOwner, m.GroupRoleAdmin, m.GroupRoleMember:
			ctx.NewStatus = 0
			return errorsx.ErrAlreadyInGroup
		case m.GroupRoleApplied:
			ctx.NewStatus = m.GroupRoleMember
		default:
			return nil
		}
	case apply:
		switch fStatus {
		case 0:
			ctx.NoStatus = true
			return nil
		case m.GroupRoleApplied:
			return nil
		case m.GroupRoleOwner, m.GroupRoleAdmin, m.GroupRoleMember:
			return errorsx.ErrAlreadyInGroup
		case m.GroupRoleBan:
			return errorsx.ErrBanned
		default:
			return errorsx.ErrInvalidParams
		}
	case acceptInvite:
		if fStatus != m.GroupRoleOwner && fStatus != m.GroupRoleAdmin {
			return errorsx.ErrInvitationHasExpired
		}
		switch tStatus {
		case 0:
			ctx.NoStatus = true
		case m.GroupRoleInvited:
			return nil
		case m.GroupRoleMember, m.GroupRoleAdmin, m.GroupRoleOwner:
			ctx.NewStatus = 0
			return errorsx.ErrAlreadyInGroup
		case m.GroupRoleBan:
			return errorsx.ErrBanned
		default:
			return errorsx.ErrInvalidParams
		}
	case handleApply:
		if fStatus != m.GroupRoleOwner && fStatus != m.GroupRoleAdmin {
			return errorsx.ErrPermissiondenied
		}
		switch tStatus {
		case m.GroupRoleApplied:
			return nil
		case 0:
			ctx.NoStatus = true
		case m.GroupRoleMember, m.GroupRoleAdmin, m.GroupRoleOwner:
			ctx.NewStatus = 0
			return errorsx.ErrAlreadyInGroup
		case m.GroupRoleBan:
			return errorsx.ErrBanned
		default:
			return errorsx.ErrInvalidParams
		}
	case handleLeave:
		switch fStatus {
		case 0:
			return errorsx.ErrNotInGroup
		case m.GroupRoleMember, m.GroupRoleApplied, m.GroupRoleAdmin:
			return nil
		case m.GroupRoleOwner:
			return errorsx.ErrOwnerCantLeave
		case m.GroupRoleBan:
			return errorsx.ErrBanned
		default:
			return errorsx.ErrInvalidParams
		}
	case handleKick:
		if fStatus != m.GroupRoleOwner && fStatus != m.GroupRoleAdmin {
			return errorsx.ErrPermissiondenied
		}
		switch tStatus {
		case m.GroupRoleMember:
			return nil
		case 0:
			return errorsx.ErrUserNotExist
		case m.GroupRoleOwner, m.GroupRoleAdmin:
			return errorsx.ErrCantKickAdmin
		default:
			return errorsx.ErrInvalidParams
		}
	case setAdmin:
		if fStatus != m.GroupRoleOwner {
			return errorsx.ErrPermissiondenied
		}
		if ctx.From == ctx.To {
			return errorsx.ErrCantSetMyselfAdmin
		}
		switch tStatus {
		case m.GroupRoleMember:
			return nil
		case m.GroupRoleAdmin:
			return errorsx.ErrAlreadyAdmin
		default:
			return errorsx.ErrInvalidParams
		}
	case removeAdmin:
		if fStatus != m.GroupRoleOwner {
			return errorsx.ErrPermissiondenied
		}
		switch tStatus {
		case m.GroupRoleAdmin:
			return nil
		case m.GroupRoleMember:
			return errorsx.ErrAlreadyMember
		default:
			return errorsx.ErrInvalidParams
		}
	case adminResign:
		switch fStatus {
		case m.GroupRoleAdmin:
			ctx.NewStatus = m.GroupRoleMember
			return nil
		case m.GroupRoleOwner:
			return errorsx.ErrHandOverOwnerFirst
		case m.GroupRoleMember:
			return errorsx.ErrNotAdmin
		case 0, m.GroupRoleApplied, m.GroupRoleInvited:
			return errorsx.ErrNotInGroup
		default:
			return errorsx.ErrInvalidParams
		}
	case handOverOwner:
		if fStatus != m.GroupRoleOwner {
			return errorsx.ErrPermissiondenied
		}
		switch tStatus {
		case m.GroupRoleMember, m.GroupRoleAdmin:
			return nil
		default:
			return errorsx.ErrNotInGroup
		}
	}
	return nil
}

// 申请加入群组
func (g *GroupService) Apply(gid, uid uint) error {
	ctx := &m.MemberStatusContext{
		GID:       gid,
		From:      uid,
		NewStatus: m.GroupRoleApplied,
	}

	if err := g.validateStatus(ctx, apply); err != nil {
		return err
	}

	ctx.To = uid
	if ctx.NoStatus {
		if err := g.service.Group().CreateMember(ctx); err != nil {
			return err
		}
	} else {
		if err := g.service.Group().UpdateStatus(ctx); err != nil {
			return err
		}
	}

	message := &ws.ChatMsg{
		Type: ws.Apply,
		Time: time.Now().UnixMilli(),
		To:   gid,
	}
	hub := g.service.Hub()
	if hub != nil {
		hub.SendToApply(message)
	} else {
		return errorsx.ErrMessagePushServiceUnavailabel
	}
	return nil
}

// 加入群组
// 接受邀请 -> 添加成员 -> 广播加入消息
func (g *GroupService) AcceptInvite(msg ws.ChatMsg) error {
	if !g.msgIsValid(msg.Time) {
		return errorsx.ErrInvitationHasExpired
	}
	gid, ok := msg.Data.(float64)
	if !ok {
		return errorsx.ErrInvalidParams
	}

	ctx := &m.MemberStatusContext{
		GID:       uint(gid),
		From:      msg.From,
		To:        msg.To,
		NewStatus: m.GroupRoleMember,
	}
	if err := g.validateStatus(ctx, acceptInvite); err != nil {
		return err
	}

	var body string
	if ctx.NoStatus {
		if err := g.service.Group().CreateMember(ctx); err != nil {
			if errors.Is(err, errorsx.ErrDuplicateEntry) {
				return errorsx.ErrOperactionFailed
			}
			return err
		}
		body = fmt.Sprintf(acceptInviteMsg, ctx.Data[ctx.From].Username, ctx.Data[ctx.To].Username)
	} else {
		if err := g.service.Group().UpdateStatus(ctx); err != nil {
			return err
		}
		body = fmt.Sprintf(joinedMsg, ctx.Data[ctx.From].Username, ctx.Data[ctx.To].Username)
	}
	g.service.Cache().AddMemberIfKeyExist(uint(gid), msg.To, m.GroupRoleMember)
	g.broadcase(uint(gid), body)
	return nil
}

// 拒绝邀请 -> 发送拒绝消息给邀请者
// func (g *GroupService) RejectInvite()

// 接受申请
func (g *GroupService) AcceptApply(from, to, gid uint) error {
	ctx := &m.MemberStatusContext{
		GID:       gid,
		From:      from,
		To:        to,
		NewStatus: m.GroupRoleMember,
	}

	if err := g.validateStatus(ctx, handleApply); err != nil {
		return err
	}
	if ctx.NoStatus {
		return errorsx.ErrNotInApplyList
	}

	if err := g.service.Group().UpdateStatus(ctx); err != nil {
		return err
	}
	g.service.Cache().AddMemberIfKeyExist(gid, to, m.GroupRoleMember)
	body := fmt.Sprintf(joinedMsg, ctx.Data[ctx.From].Username, ctx.Data[ctx.To].Username)
	if err := g.broadcase(gid, body); err != nil {
		return err
	}
	return nil
}

// 拒绝申请 -> 删除申请行
func (g *GroupService) RejectApply(from, to, gid uint) error {
	ctx := &m.MemberStatusContext{
		GID:  gid,
		From: from,
		To:   to,
	}

	if err := g.validateStatus(ctx, handleApply); err != nil {
		if errors.Is(err, errorsx.ErrUserNotExist) {
			return nil
		}
		return err
	}

	if ctx.NoStatus {
		return nil
	}

	if err := g.service.Group().DeleteMember(ctx); err != nil {
		return err
	}

	body := fmt.Sprintf(rejectApply, ctx.Data[from].Username)
	msg := g.newMessage(from, to, body, nil)
	hub := g.service.Hub()
	if hub != nil {
		hub.SendToChat(msg)
	} else {
		g.service.Cache().CacheMessage(msg.To, msg)
		return errorsx.ErrMessagePushServiceUnavailabel
	}
	return nil
}

// 离开群组
/* 群主不能直接退出群聊，应该先移交群主
管理员和普通成员可以直接退出
*/
func (g *GroupService) Leave(uid, gid uint) error {
	ctx := &m.MemberStatusContext{
		GID:  gid,
		From: uid,
	}

	if err := g.validateStatus(ctx, handleLeave); err != nil {
		if errors.Is(err, errorsx.ErrUserNotExist) {
			return nil
		}
		return err
	}

	ctx.To = uid
	if err := g.service.Group().DeleteMember(ctx); err != nil {
		return err
	}
	err := g.removeCacheMember(gid, uid)
	body := fmt.Sprintf(leaveMsg, ctx.Data[uid].Username)
	if err := g.broadcase(gid, body); err != nil {
		return err
	}
	return err
}

// 踢出群组
/* 群主和管理员不能被踢出群聊
只能够踢出普通成员，请求者应该具有管理员以上权限
*/
func (g *GroupService) Kick(from, to, gid uint) error {
	ctx := &m.MemberStatusContext{
		GID:  gid,
		From: from,
		To:   to,
	}
	if err := g.validateStatus(ctx, handleKick); err != nil {
		if errors.Is(err, errorsx.ErrUserNotExist) {
			return nil
		}
		return err
	}

	if err := g.service.Group().DeleteMember(ctx); err != nil {
		return err
	}

	err := g.removeCacheMember(gid, to)

	msg := fmt.Sprintf(kickMsg, ctx.Data[from].Username, ctx.Data[to].Username)
	if err := g.broadcase(gid, msg); err != nil {
		return err
	}

	return err
}

// 移交群主
func (g *GroupService) HandOverOwner(from, to uint, gid uint) error {
	ctx := &m.MemberStatusContext{
		GID:  gid,
		From: from,
		To:   to,
	}

	if err := g.validateStatus(ctx, handOverOwner); err != nil {
		return err
	}

	if err := g.service.Group().HandOverOwner(from, to, gid); err != nil {
		if errors.Is(err, errorsx.ErrNoAffectedRows) {
			return errorsx.ErrNotInGroup
		}
		if errors.Is(err, errorsx.ErrPermissionDenied) {
			return errorsx.ErrPermissiondenied
		}
		return err
	}

	g.service.Cache().AddMemberIfKeyExist(gid, from, m.GroupRoleMember)
	g.service.Cache().AddMemberIfKeyExist(gid, to, m.GroupRoleOwner)
	body := fmt.Sprintf(handOverOwnerMsg, ctx.Data[to].Username)
	if err := g.broadcase(gid, body); err != nil {
		return err
	}

	return nil
}

// 获取权限
func (g *GroupService) QueryRole(ctx *m.MemberStatusContext) (*m.MemberStatusContext, error) {
	if err := validator.ValidateGID(ctx.GID); err != nil {
		return nil, errorsx.ErrInvalidParams
	}
	ids := []uint{ctx.From}
	if ctx.To != 0 {
		ids = append(ids, ctx.To)
	}
	for _, id := range ids {
		if err := validator.ValidateUID(id); err != nil {
			return nil, errorsx.ErrInvalidParams
		}
	}

	result, err := g.service.Group().QueryRole(ctx.GID, ids...)
	if err != nil {
		g.service.Logger().Error("Failed to query role", zap.Error(err))
		return nil, err
	}

	data := make(map[uint]*m.GroupMemberRole)
	for _, m := range result {
		data[m.MemberID] = m
	}
	ctx.Data = data
	if len(data) < len(ids) {
		return ctx, errorsx.ErrUserNotExist
	}

	return ctx, nil
}

// 修改管理员
// 只有群主有权限设置管理员
// 群主和管理员自身都有权限撤销管理员
func (g *GroupService) ModifyAdmin(from, to, gid uint, newStatus int) error {
	if newStatus != m.GroupRoleAdmin && newStatus != m.GroupRoleMember {
		return errorsx.ErrInvalidParams
	}

	ctx := &m.MemberStatusContext{
		GID:       gid,
		From:      from,
		To:        to,
		NewStatus: newStatus,
	}

	var body string
	if newStatus == m.GroupRoleAdmin {
		if err := g.validateStatus(ctx, setAdmin); err != nil {
			return err
		}
		body = fmt.Sprintf(setAdminMsg, ctx.Data[from].Username, ctx.Data[to].Username)
	} else {
		if err := g.validateStatus(ctx, removeAdmin); err != nil {
			return err
		}
		body = fmt.Sprintf(removeAdminMsg, ctx.Data[to].Username, ctx.Data[from].Username)
	}

	if err := g.service.Group().UpdateStatus(ctx); err != nil {
		return err
	}

	g.service.Cache().AddMemberIfKeyExist(gid, to, ctx.NewStatus)

	if err := g.broadcase(gid, body); err != nil {
		return err
	}

	return nil
}

func (g *GroupService) AdminResign(uid, gid uint) error {
	ctx := &m.MemberStatusContext{
		GID:       gid,
		From:      uid,
		NewStatus: m.GroupRoleMember,
	}

	if err := g.validateStatus(ctx, adminResign); err != nil {
		if errors.Is(err, errorsx.ErrUserNotExist) {
			return nil
		}
		return err
	}

	ctx.To = uid
	if err := g.service.Group().UpdateStatus(ctx); err != nil {
		return err
	}

	g.service.Cache().AddMemberIfKeyExist(gid, uid, m.GroupRoleMember)
	body := fmt.Sprintf(removeAdminMsg, ctx.Data[uid].Username, ctx.Data[uid].Username)
	if err := g.broadcase(gid, body); err != nil {
		return err
	}

	return nil
}

func (g *GroupService) broadcase(gid uint, body string) error {
	message := &ws.ChatMsg{
		Type: ws.System,
		Body: body,
		Time: time.Now().UnixMilli(),
		To:   gid,
	}
	hub := g.service.Hub()
	if hub == nil {
		ids, err := g.service.Cache().GetMembersAndCache(message.To)
		if err != nil {
			return err
		}
		g.cacheMessage(message, ids...)
		return errorsx.ErrMessagePushServiceUnavailabel
	}

	hub.SendToBroadcast(message)
	return nil
}

func (g *GroupService) cacheMessage(msg any, uid ...uint) {
	for _, id := range uid {
		g.service.Cache().CacheMessage(id, msg)
	}
}
func (g *GroupService) removeCacheMember(gid, uid uint) error {
	maxRetries := g.service.Config().Common().MaxRetries()
	for try := 0; try < maxRetries; try++ {
		if err := g.service.Cache().RemoveMember(gid, uid); err == nil {
			return nil
		}
		delay := g.service.Config().Cache().RetryDelay(try)
		time.Sleep(delay)
	}
	return errorsx.ErrHandleSuccessed
}

// Ban
// func (g *GroupService) Ban(from, to string, gid uint) error {
//  return nil
// }

// 解除Ban
// func (g *GroupService) RemoveBan() {}

// 禁止成员列表
// func (g *GroupService) BanList() {}

func (g *GroupService) msgIsValid(msgTime int64) bool {
	now := time.Now().UnixMilli()
	validDays := g.service.Config().Common().InviteValidDays()
	msgValidityPeriod := int64(validDays) * 24 * time.Hour.Milliseconds()
	return now > msgTime && now-msgTime <= msgValidityPeriod
}

func (g *GroupService) newMessage(from, to uint, body string, data any) *ws.ChatMsg {
	return &ws.ChatMsg{
		Type: ws.System,
		From: from,
		To:   to,
		Body: body,
		Time: time.Now().UnixMilli(),
		Data: data,
	}
}

// 发布公告
func (g *GroupService) ReleaseAnnounce(data *m.GroupAnnouncement) error {
	uid, gid := data.CreatedBy, data.GroupID
	if err := g.hasManageAnnouncementPermission(gid, uid); err != nil {
		return err
	}
	if err := g.service.Group().ReleaseAnnounce(data); err != nil {
		return errorsx.ErrOperactionFailed
	}
	return nil
}

// 删除公告
func (g *GroupService) DeleteAnnounce(uid, gid, announceID uint) error {
	// if err := g.hasManageAnnouncementPermission(gid, uid); err != nil {
	//  return err
	// }
	if err := g.service.Group().DeleteAnnounce(gid, uid, announceID); err != nil {
		if err == errorsx.ErrNoAffectedRows {
			return errorsx.ErrPermissiondenied
		}
		return err
	}
	return nil
}

// 查看公告
func (g *GroupService) ViewAnnounce(uid, gid uint, cursor *m.Cursor) (map[string]any, error) {
	if err := validator.VerfityPageSize(cursor.PageSize); err != nil {
		return nil, err
	}
	// if err := g.hasViewAnnouncementPermission(gid, uid); err != nil {
	//  return nil, err
	// }
	announce, cursor, err := g.service.Group().ViewAnnounce(gid, uid, cursor)
	if err != nil {
		return nil, errorsx.ErrOperactionFailed
	}
	data := map[string]any{"data": announce, "cursor": cursor}
	return data, nil
}

// 查看最新公告
func (g *GroupService) ViewLatestAnnounce(uid, gid uint) (*m.GroupAnnounceInfo, error) {
	// if err := g.hasViewAnnouncementPermission(gid, uid); err != nil {
	//  return nil, err
	// }
	announce, _, err := g.service.Group().ViewAnnounce(gid, uid, nil)
	if err != nil {
		return nil, err
	}
	if len(announce) == 0 {
		return nil, nil
	}
	return announce[0], nil
}

func (g *GroupService) hasManageAnnouncementPermission(gid, uid uint) error {
	return g.hasPermissionOfAnnounce(gid, uid, m.GroupRoleOwner, m.GroupRoleAdmin)
}

func (g *GroupService) hasViewAnnouncementPermission(gid, uid uint) error {
	return g.hasPermissionOfAnnounce(gid, uid, m.GroupRoleMember, m.GroupRoleAdmin, m.GroupRoleOwner)
}

func (g *GroupService) hasPermissionOfAnnounce(gid, uid uint, roles ...int) error {
	ctx := &m.MemberStatusContext{
		GID:  gid,
		From: uid,
		// To:   uid,
	}
	ctx, err := g.QueryRole(ctx)
	if err != nil {
		return err
	}
	role := ctx.Data[uid].Role
	if slices.Contains(roles, role) {
		return nil
	}
	return errorsx.ErrPermissiondenied
}
