package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/farnese17/chat/registry"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/validator"
	ws "github.com/farnese17/chat/websocket"
)

type updateFriendFunctions func(*updateFriendContext) error
type updateFriendContext struct {
	from   uint
	to     uint
	status int
	friend *m.Friend
}

type sendMessageFunctions func(*sendMessageContext) error
type sendMessageContext struct {
	from         uint
	to           uint
	msgType      int
	bodyToFrom   string
	bodyToTarget string
	user         map[uint]string
	once         bool
}

const (
	acceptRequestTo       = "%s 通过了你的好友请求"
	addFriend             = "添加 %s 为好友"
	requestFriend         = "请求添加 %s 为好友"
	requestFriendTo       = "%s 请求添加你为好友"
	rejectFriendRequestTo = "%s 拒绝了你的好友请求"
	rejectFriendRequest   = "拒绝了 %s 的好友请求"
	deleteFriend          = "删除了好友 %s"
	blockedUser           = "将 %s 添加到黑名单"
	unBlockUSer           = "将 %s 移出了黑名单"
)

type FriendService struct {
	service registry.Service
}

func NewFriendService(s registry.Service) *FriendService {
	return &FriendService{s}
}

// 检查好友状态
func (f *FriendService) QueryStatus(from, to uint) (*m.Friend, error) {
	friend, err := f.service.Friend().QueryStatus(f.sortID(from, to))
	if err != nil {
		if errors.Is(err, errorsx.ErrRecordNotFound) {
			return friend, errorsx.ErrNotFound
		}
		return nil, err
	}
	return friend, nil
}

// 好友请求：
func (f *FriendService) Request(from uint, to uint) error {
	status := m.FSReq1To2
	if from > to {
		status = m.FSReq2To1
	}
	ctx := &updateFriendContext{
		from:   from,
		to:     to,
		status: status,
	}
	if err := f.updateStatus(ctx, f.queryStatus(), f.validFriendStatus()); err != nil {
		return err
	}
	msgCtx := &sendMessageContext{
		from:         from,
		to:           to,
		msgType:      ws.System,
		bodyToFrom:   requestFriend,
		bodyToTarget: requestFriendTo,
	}
	if ctx.status == status {
		if err := f.sendMessage(msgCtx, f.getUsername()); err != nil {
			return err
		}
	} else {
		msgCtx.bodyToFrom = addFriend
		msgCtx.bodyToTarget = acceptRequestTo
		if err := f.sendMessage(msgCtx, f.getUsername()); err != nil {
			return err
		}
	}
	return nil
}

// 接受请求：
func (f *FriendService) Accept(from, to uint) error {
	ctx := &updateFriendContext{
		from:   from,
		to:     to,
		status: m.FSAdded,
	}

	if err := f.updateStatus(ctx, f.queryStatus(), f.validFriendStatus()); err != nil {
		return err
	}

	msgCtx := &sendMessageContext{
		from:         from,
		to:           to,
		msgType:      ws.System,
		bodyToFrom:   addFriend,
		bodyToTarget: acceptRequestTo,
	}
	if err := f.sendMessage(msgCtx, f.getUsername()); err != nil {
		return err
	}
	return nil
}

// 拒绝：标记为0
func (f *FriendService) Reject(from, to uint) error {
	ctx := &updateFriendContext{
		from:   from,
		to:     to,
		status: m.RejectFriend,
	}

	if err := f.updateStatus(ctx, f.queryStatus(), f.validFriendStatus()); err != nil {
		return err
	}

	msgCtx := &sendMessageContext{
		from:         from,
		to:           to,
		msgType:      ws.System,
		bodyToFrom:   rejectFriendRequest,
		bodyToTarget: rejectFriendRequestTo,
	}
	if err := f.sendMessage(msgCtx, f.getUsername()); err != nil {
		return err
	}
	return nil
}

func (f *FriendService) updateStatus(ctx *updateFriendContext,
	operations ...updateFriendFunctions) error {
	maxRetries := f.service.Config().Common().MaxRetries()
	for try := 0; try < maxRetries; try++ {
		for _, op := range operations {
			if err := op(ctx); err != nil {
				return err
			}
		}
		id1, id2 := f.sortID(ctx.from, ctx.to)
		ctx.friend.User1 = id1
		ctx.friend.User2 = id2
		ctx.friend.Status = ctx.status
		ctx.friend.Version += 1
		if err := f.service.Friend().UpdateStatus(ctx.friend); err != nil {
			if f.retry(err, try) {
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

func (f *FriendService) sendMessage(ctx *sendMessageContext,
	functions ...sendMessageFunctions) error {
	for _, fn := range functions {
		if err := fn(ctx); err != nil {
			return err
		}
	}

	now := time.Now().UnixMilli()
	msg := &ws.ChatMsg{
		Type: ctx.msgType,
		From: ctx.to,
		To:   ctx.from,
		Body: fmt.Sprintf(ctx.bodyToFrom, ctx.user[ctx.to]),
		Time: now,
	}

	var err error
	err = f.sendToWebsocket(msg)

	if !ctx.once {
		msg := &ws.ChatMsg{
			Type: ctx.msgType,
			From: ctx.from,
			To:   ctx.to,
			Body: fmt.Sprintf(ctx.bodyToTarget, ctx.user[ctx.from]),
			Time: now,
		}
		err = f.sendToWebsocket(msg)
	}
	return err
}

func (f *FriendService) sendToWebsocket(msg *ws.ChatMsg) error {
	hub := f.service.Hub()
	if hub == nil {
		f.storeOfflineMessage(msg.To, msg)
		return errorsx.ErrMessagePushServiceUnavailabel
	}
	hub.SendToChat(msg)
	return nil
}

func (f *FriendService) storeOfflineMessage(id uint, msg any) {
	f.service.Cache().StoreOfflineMessage(id, msg)
}

func (f *FriendService) getUsername() sendMessageFunctions {
	return func(ctx *sendMessageContext) error {
		var user []*m.User
		var err error
		maxRetries := f.service.Config().Common().MaxRetries()
		for try := 0; try < maxRetries; try++ {
			user, err = f.service.Friend().GetUser(ctx.from, ctx.to)
			if err != nil {
				if f.retry(err, try) {
					continue
				}
				return err
			}
			break
		}

		data := make(map[uint]string)
		for _, u := range user {
			data[u.ID] = u.Username
		}
		ctx.user = data
		return nil
	}
}

func (f *FriendService) queryStatus() updateFriendFunctions {
	return func(ctx *updateFriendContext) error {
		friend, err := f.QueryStatus(ctx.from, ctx.to)
		ctx.friend = friend
		if err != nil && !errors.Is(err, errorsx.ErrNotFound) {
			return err
		}
		return nil
	}
}

func (f *FriendService) validFriendStatus() updateFriendFunctions {
	return func(ctx *updateFriendContext) error {
		smaller := ctx.from < ctx.to
		newStatus, status := ctx.status, ctx.friend.Status
		selfBlocked := (smaller && status == m.FSBlock1To2) ||
			(!smaller && status == m.FSBlock2To1)
		otherBlocked := (smaller && status == m.FSBlock2To1) ||
			(!smaller && status == m.FSBlock1To2)
		otherRequest := (smaller && status == m.FSReq2To1) ||
			(!smaller && status == m.FSReq1To2)
		switch newStatus {
		case m.FSReq1To2, m.FSReq2To1:
			switch status {
			case m.FSNull:
				return nil
			case m.FSReq1To2, m.FSReq2To1:
				if otherRequest {
					ctx.status = m.FSAdded
				}
				return nil
			case m.FSAdded:
				return errorsx.ErrAlreadyFriend
			case m.FSBlock1To2, m.FSBlock2To1, m.FSBothBlocked:
				if selfBlocked || status == m.FSBothBlocked {
					return errorsx.ErrAlreadyBlock
				}
				return errorsx.ErrBlocked
			default:
				return errorsx.ErrInvalidParams
			}
		case m.FSAdded:
			switch status {
			case m.FSNull:
				return errorsx.ErrNoRequest
			case m.FSReq1To2, m.FSReq2To1:
				if (smaller && status == m.FSReq2To1) ||
					(!smaller && status == m.FSReq1To2) {
					return nil
				}
				return errorsx.ErrNoRequest
			case m.FSAdded:
				return errorsx.ErrAlreadyFriend
			case m.FSBlock1To2, m.FSBlock2To1, m.FSBothBlocked:
				if selfBlocked || status == m.FSBothBlocked {
					return errorsx.ErrAlreadyBlock
				}
				return errorsx.ErrBlocked
			default:
				return errorsx.ErrInvalidParams
			}
		case m.RejectFriend:
			switch status {
			case m.FSReq1To2, m.FSReq2To1:
				if otherRequest {
					ctx.status = m.FSNull
					return nil
				}
				return errorsx.ErrNoRequest
			case m.FSAdded:
				return errorsx.ErrAlreadyFriend
			case m.FSBlock1To2, m.FSBlock2To1, m.FSBothBlocked:
				return errorsx.ErrNil
			default:
				return errorsx.ErrNoRequest
			}
		case m.FSNull: // delete
			switch status {
			case m.FSAdded:
				return nil
			case m.FSNull, m.FSReq1To2, m.FSReq2To1:
				return errorsx.ErrNil
			case m.FSBlock1To2, m.FSBlock2To1, m.FSBothBlocked:
				return errorsx.ErrNil
			default:
				return errorsx.ErrInvalidParams
			}
		case m.FSBlock1To2, m.FSBlock2To1:
			switch status {
			case m.FSNull, m.FSAdded, m.FSReq1To2, m.FSReq2To1:
				return nil
			case m.FSBlock1To2, m.FSBlock2To1, m.FSBothBlocked:
				if otherBlocked {
					ctx.status = m.FSBothBlocked
					return nil
				}
				if newStatus == status || status == m.FSBothBlocked {
					return errorsx.ErrAlreadyBlock
				}
			default:
				return errorsx.ErrInvalidParams
			}
		case m.UnblockFriend:
			switch status {
			case m.FSNull, m.FSReq1To2, m.FSReq2To1, m.FSAdded:
				return errorsx.ErrNoBlocked
			case m.FSBlock1To2, m.FSBlock2To1:
				if selfBlocked {
					ctx.status = m.FSNull
					return nil
				}
				return errorsx.ErrNoBlocked
			case m.FSBothBlocked:
				if smaller {
					ctx.status = m.FSBlock2To1
				} else {
					ctx.status = m.FSBlock1To2
				}
				return nil
			default:
				return errorsx.ErrInvalidParams
			}
		}
		return nil
	}
}

// 删除好友
func (f *FriendService) Delete(from, to uint) error {
	ctx := &updateFriendContext{
		from:   from,
		to:     to,
		status: m.FSNull,
	}
	if err := f.updateStatus(ctx, f.queryStatus(), f.validFriendStatus()); err != nil {
		return err
	}

	msgctx := &sendMessageContext{
		from:       from,
		to:         to,
		msgType:    ws.System,
		bodyToFrom: deleteFriend,
		once:       true,
	}
	if err := f.sendMessage(msgctx, f.getUsername()); err != nil {
		return err
	}
	return nil
}

func (f *FriendService) Block(from, to uint) error {
	status := m.FSBlock1To2
	if from > to {
		status = m.FSBlock2To1
	}
	ctx := &updateFriendContext{
		from:   from,
		to:     to,
		status: status,
	}
	if err := f.updateStatus(ctx, f.queryStatus(), f.validFriendStatus()); err != nil {
		return err
	}
	// 先发送给对方，让前端处理
	msg := &ws.ChatMsg{
		Type: ws.UpdateBlackList,
		From: from,
		To:   to,
		// Time:  time.Now().UnixMilli(),
		Extra: true,
	}
	var err error
	hub := f.service.Hub()
	if hub == nil {
		f.storeOfflineMessage(msg.To, msg)
		err = errorsx.ErrHandleSuccessed
	} else {
		hub.SendUpdateBlockedListNotify(msg)
	}
	// 发送通知
	msgctx := &sendMessageContext{
		from:       from,
		to:         to,
		msgType:    ws.System,
		bodyToFrom: blockedUser,
		once:       true,
	}
	if err := f.sendMessage(msgctx, f.getUsername()); err != nil &&
		!errors.Is(err, errorsx.ErrMessagePushServiceUnavailabel) {
		return err
	}
	return err
}

// 移出黑名单
func (f *FriendService) Unblock(from, to uint) error {
	ctx := &updateFriendContext{
		from:   from,
		to:     to,
		status: m.UnblockFriend,
	}
	if err := f.updateStatus(ctx, f.queryStatus(), f.validFriendStatus()); err != nil {
		return err
	}

	// 发送给对方，前端处理
	var err error
	msg := &ws.ChatMsg{
		Type: ws.UpdateBlackList,
		From: from,
		To:   to,
		// Time:  time.Now().UnixMilli(),
		Extra: false,
	}
	hub := f.service.Hub()
	if hub == nil {
		f.storeOfflineMessage(msg.To, msg)
		err = errorsx.ErrHandleSuccessed
	} else {
		hub.SendUpdateBlockedListNotify(msg)
	}

	// 返回通知
	msgctx := &sendMessageContext{
		from:       from,
		to:         to,
		msgType:    ws.System,
		bodyToFrom: unBlockUSer,
		once:       true,
	}
	if err := f.sendMessage(msgctx, f.getUsername()); err != nil &&
		!errors.Is(err, errorsx.ErrMessagePushServiceUnavailabel) {
		return err
	}
	return err
}

// 备注
func (f *FriendService) Remark(from, to uint, remark string) error {
	if err := validator.ValidateRemark(remark); err != nil {
		return err
	}
	if err := f.setRemarkOrGroup(from, to, remark, "remark"); err != nil {
		return err
	}
	return nil
}

// 移动分组
func (f *FriendService) SetGroup(from, to uint, group string) error {
	if err := validator.ValidateGroupname(group); err != nil {
		return err
	}
	if err := f.setRemarkOrGroup(from, to, group, "group"); err != nil {
		return err
	}
	return nil
}

func (f *FriendService) setRemarkOrGroup(from, to uint, val, column string) error {
	maxRetries := f.service.Config().Common().MaxRetries()
	for try := 0; try < maxRetries; try++ {
		friend, err := f.QueryStatus(from, to)
		if err != nil {
			return err
		}

		group, remark := &friend.User1Group, &friend.User1Remark
		if from > to {
			group, remark = &friend.User2Group, &friend.User2Remark
		}
		if column == "group" {
			if *group == val {
				return nil
			}
			*group = val
		} else if column == "remark" {
			if *remark == val {
				return nil
			}
			*remark = val
		} else {
			return errorsx.ErrInvalidParams
		}
		friend.Version += 1
		if err := f.service.Friend().UpdateRemarkOrGroup(friend); err != nil {
			if f.retry(err, try) {
				continue
			}
			return err
		}
		break
	}
	return nil
}

// 好友列表
func (f *FriendService) List(id uint) (map[string][]*m.SummaryFriendInfo, error) {
	var friends []*m.SummaryFriendInfo
	var err error
	maxRetries := f.service.Config().Common().MaxRetries()
	for try := 0; try < maxRetries; try++ {
		friends, err = f.service.Friend().List(id)
		if err != nil {
			if f.retry(err, try) {
				continue
			}
			return nil, err
		}
		break
	}

	data := make(map[string][]*m.SummaryFriendInfo)
	for _, friend := range friends {
		if id < friend.UID {
			if friend.Status == m.FSBlock2To1 {
				data["other_blocked"] = append(data["other_blocked"], friend)
			}
			if friend.Status == m.FSBlock1To2 || friend.Status == m.FSBothBlocked {
				data["blocked"] = append(data["blocked"], friend)
			} else if friend.Status == m.FSReq2To1 {
				data["request"] = append(data["request"], friend)
			}
		} else {
			if friend.Status == m.FSBlock1To2 {
				data["other_blocked"] = append(data["other_blocked"], friend)
			}
			if friend.Status == m.FSBlock2To1 || friend.Status == m.FSBothBlocked {
				data["blocked"] = append(data["blocked"], friend)
			} else if friend.Status == m.FSReq1To2 {
				data["request"] = append(data["request"], friend)
			}
		}
		if friend.Status == m.FSAdded {
			if friend.Group != "" {
				data[friend.Group] = append(data[friend.Group], friend)
			} else {
				data["default"] = append(data["default"], friend)
			}
		}
	}
	return data, nil
}

func (f *FriendService) Get(from, to uint) (*m.Friendinfo, error) {
	friend, err := f.service.Friend().Get(from, to)
	if err != nil {
		return nil, err
	}
	return friend, nil
}

// 黑名单列表，我被谁拉黑
func (f *FriendService) BlockedMeList(id uint) ([]uint, error) {
	return f.service.Friend().BlockedMeList(id)
}

func (f *FriendService) Search(id uint, value string, cursor *m.Cursor) (map[string]any, error) {
	if value == "" {
		return nil, errorsx.ErrInputEmpty
	}

	if !cursor.HasMore {
		return nil, nil
	}
	if err := validator.VerfityPageSize(cursor.PageSize); err != nil {
		return nil, err
	}

	cursor, data, err := f.service.Friend().Search(id, value, cursor)
	if err != nil {
		return nil, err
	}

	res := map[string]any{"cursor": cursor, "data": data}
	return res, nil
}

func (f *FriendService) sortID(id1, id2 uint) (uint, uint) {
	if id1 < id2 {
		return id1, id2
	}
	return id2, id1
}

func (f *FriendService) retry(err error, try int) bool {
	maxRetries := f.service.Config().Common().MaxRetries()
	delay := f.service.Config().Common().RetryDelay(try)
	if try < maxRetries-1 && !errors.Is(err, errorsx.ErrOperactionTimeout) {
		time.Sleep(delay)
		return true
	}
	return false
}
