package service_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/farnese17/chat/service/mock"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	ws "github.com/farnese17/chat/websocket"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestQueryStatus(t *testing.T) {
	setup(t)
	defer clear(t)

	var friend = &model.Friend{User1: uid, User2: uid + 1, Status: 1, Version: 1}
	tests := []struct {
		mock     error
		expected error
	}{
		{errorsx.ErrRecordNotFound, errorsx.ErrNotFound},
		{errors.New("error"), errorsx.ErrFailed},
		{nil, nil},
	}

	for i, tt := range tests {
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, tt.mock)
		t.Run(fmt.Sprintf("query status %d", i), func(t *testing.T) {
			data, err := f.QueryStatus(uid, uid+1)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				assert.Equal(t, friend, data)
			}
		})
	}
}

var user = []*model.User{
	// {Model: gorm.Model{ID: uid}, Username: "test1"},
	// {Model: gorm.Model{ID: uid + 1}, Username: "test2"}}
	{ID: uid, Username: "test1"},
	{ID: uid + 1, Username: "test2"}}
var friend = &model.Friend{User1: uid, User2: uid + 1, Version: 1}
var name = map[uint]string{uid: "test1", uid + 1: "test2"}

func TestRequest(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSAdded, nil, errorsx.ErrAlreadyFriend},
		{uid, uid + 1, model.FSBlock1To2, nil, errorsx.ErrAlreadyBlock},
		{uid, uid + 1, model.FSBlock2To1, nil, errorsx.ErrBlocked},
		{uid, uid + 1, model.FSBothBlocked, nil, errorsx.ErrAlreadyBlock},
		{uid, uid + 1, model.FSReq1To2, nil, nil},
		{uid, uid + 1, model.FSNull, nil, nil},
		{uid, uid + 1, 0, nil, nil},
		{uid, uid + 1, 999, nil, errorsx.ErrInvalidParams},
		{uid, uid + 1, -999, nil, errorsx.ErrInvalidParams},
		{uid + 1, uid, model.FSAdded, nil, errorsx.ErrAlreadyFriend},
		{uid + 1, uid, model.FSBlock1To2, nil, errorsx.ErrBlocked},
		{uid + 1, uid, model.FSBlock2To1, nil, errorsx.ErrAlreadyBlock},
		{uid + 1, uid, model.FSBothBlocked, nil, errorsx.ErrAlreadyBlock},
		{uid + 1, uid, model.FSReq2To1, nil, nil},
		{uid + 1, uid, model.FSNull, nil, nil},
	}

	for i, tt := range tests {
		friend.Status = tt.status
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.expected == nil {
			mockf.EXPECT().UpdateStatus(friend).Return(tt.mock)
			mockf.EXPECT().GetUser(gomock.Any()).Return(user, nil)
		}
		t.Run(fmt.Sprintf("request %d", i), func(t *testing.T) {
			err := f.Request(tt.from, tt.to)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					From: tt.to,
					To:   tt.from,
					Time: msg.Time,
					Body: fmt.Sprintf("请求添加 %s 为好友", name[tt.to]),
				}
				assert.Equal(t, message, msg)
				msg = <-mock.Message
				message.From = tt.from
				message.To = tt.to
				message.Body = fmt.Sprintf("%s 请求添加你为好友", name[tt.from])
				assert.Equal(t, message, msg)
			}
		})
	}

	tests = []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSReq2To1, nil, nil},
		{uid + 1, uid, model.FSReq1To2, nil, nil},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("request: status transfer %d", i), func(t *testing.T) {
			friend.Status = tt.status
			mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
			mockf.EXPECT().UpdateStatus(friend).Return(nil)
			mockf.EXPECT().GetUser(gomock.Any()).Return(user, nil)
			err := f.Request(tt.from, tt.to)
			assert.NoError(t, err)
			msg := <-mock.Message
			message := &ws.ChatMsg{
				Type: ws.System,
				From: tt.to,
				To:   tt.from,
				Time: msg.Time,
				Body: fmt.Sprintf("添加 %s 为好友", name[tt.to]),
			}
			assert.Equal(t, message, msg)
			msg = <-mock.Message
			message.From = tt.from
			message.To = tt.to
			message.Body = fmt.Sprintf("%s 通过了你的好友请求", name[tt.from])
			assert.Equal(t, message, msg)
		})
	}

	t.Run("request: retry", func(t *testing.T) {
		friend.Status = 0
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil).Times(cfg.Common().MaxRetries())
		mockf.EXPECT().UpdateStatus(friend).Return(errors.New("error")).Times(cfg.Common().MaxRetries())
		err := f.Request(uid, uid+1)
		assert.Equal(t, errorsx.ErrFailed, err)
	})
}

func TestAccept(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSAdded, nil, errorsx.ErrAlreadyFriend},
		{uid, uid + 1, model.FSBlock1To2, nil, errorsx.ErrAlreadyBlock},
		{uid, uid + 1, model.FSBlock2To1, nil, errorsx.ErrBlocked},
		{uid, uid + 1, model.FSBothBlocked, nil, errorsx.ErrAlreadyBlock},
		{uid, uid + 1, model.FSNull, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, model.FSReq1To2, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, model.FSReq2To1, nil, nil},
		{uid, uid + 1, 0, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, 999, nil, errorsx.ErrInvalidParams},
		{uid, uid + 1, -999, nil, errorsx.ErrInvalidParams},
		{uid + 1, uid, model.FSAdded, nil, errorsx.ErrAlreadyFriend},
		{uid + 1, uid, model.FSBlock1To2, nil, errorsx.ErrBlocked},
		{uid + 1, uid, model.FSBlock2To1, nil, errorsx.ErrAlreadyBlock},
		{uid + 1, uid, model.FSBothBlocked, nil, errorsx.ErrAlreadyBlock},
		{uid + 1, uid, model.FSNull, nil, errorsx.ErrNoRequest},
		{uid + 1, uid, model.FSReq2To1, nil, errorsx.ErrNoRequest},
		{uid + 1, uid, model.FSReq1To2, nil, nil},
	}

	for i, tt := range tests {
		friend.Status = tt.status
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.expected == nil {
			mockf.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
			mockf.EXPECT().GetUser(tt.from, tt.to).Return(user, nil)
		}
		t.Run(fmt.Sprintf("accept friend request %d", i), func(t *testing.T) {
			err := f.Accept(tt.from, tt.to)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					From: tt.to,
					To:   tt.from,
					Time: msg.Time,
					Body: fmt.Sprintf("添加 %s 为好友", name[tt.to]),
				}
				assert.Equal(t, message, msg)
				msg = <-mock.Message
				message.From = tt.from
				message.To = tt.to
				message.Body = fmt.Sprintf("%s 通过了你的好友请求", name[tt.from])
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestReject(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSAdded, nil, errorsx.ErrAlreadyFriend},
		{uid, uid + 1, model.FSBlock1To2, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSBlock2To1, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSBothBlocked, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSNull, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, model.FSReq1To2, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, model.FSReq2To1, nil, nil},
		{uid, uid + 1, 0, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, 999, nil, errorsx.ErrNoRequest},
		{uid, uid + 1, -999, nil, errorsx.ErrNoRequest},
		{uid + 1, uid, model.FSAdded, nil, errorsx.ErrAlreadyFriend},
		{uid + 1, uid, model.FSBlock1To2, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSBlock2To1, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSBothBlocked, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSNull, nil, errorsx.ErrNoRequest},
		{uid + 1, uid, model.FSReq2To1, nil, errorsx.ErrNoRequest},
		{uid + 1, uid, model.FSReq1To2, nil, nil},
	}

	for i, tt := range tests {
		friend.Status = tt.status
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.expected == nil {
			mockf.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
			mockf.EXPECT().GetUser(gomock.Any()).Return(user, nil)
		}
		t.Run(fmt.Sprintf("reject friend %d", i), func(t *testing.T) {
			err := f.Reject(tt.from, tt.to)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					From: tt.to,
					To:   tt.from,
					Time: msg.Time,
					Body: fmt.Sprintf("拒绝了 %s 的好友请求", name[tt.to]),
				}
				assert.Equal(t, message, msg)
				msg = <-mock.Message
				message.From = tt.from
				message.To = tt.to
				message.Body = fmt.Sprintf("%s 拒绝了你的好友请求", name[tt.from])
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestDeleteFriend(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSBlock1To2, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSBlock2To1, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSBothBlocked, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSNull, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSReq1To2, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSReq2To1, nil, errorsx.ErrNil},
		{uid, uid + 1, model.FSAdded, nil, nil},
		{uid, uid + 1, 0, nil, errorsx.ErrNil},
		{uid, uid + 1, 999, nil, errorsx.ErrInvalidParams},
		{uid, uid + 1, -999, nil, errorsx.ErrInvalidParams},
		{uid + 1, uid, model.FSBlock1To2, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSBlock2To1, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSBothBlocked, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSNull, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSReq1To2, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSReq2To1, nil, errorsx.ErrNil},
		{uid + 1, uid, model.FSAdded, nil, nil},
	}

	for i, tt := range tests {
		friend.Status = tt.status
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.expected == nil {
			mockf.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
			mockf.EXPECT().GetUser(gomock.Any()).Return(user, nil)
		}
		t.Run(fmt.Sprintf("delete friend %d", i), func(t *testing.T) {
			err := f.Delete(tt.from, tt.to)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.Message
				message := &ws.ChatMsg{
					Type: ws.System,
					From: tt.to,
					To:   tt.from,
					Time: msg.Time,
					Body: fmt.Sprintf("删除了好友 %s", name[tt.to]),
				}
				assert.Equal(t, message, msg)
			}
		})
	}
}

func TestBlock(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSBlock1To2, nil, errorsx.ErrAlreadyBlock},
		{uid, uid + 1, model.FSBothBlocked, nil, errorsx.ErrAlreadyBlock},
		{uid, uid + 1, model.FSNull, nil, nil},
		{uid, uid + 1, model.FSReq1To2, nil, nil},
		{uid, uid + 1, model.FSReq2To1, nil, nil},
		{uid, uid + 1, model.FSAdded, nil, nil},
		{uid, uid + 1, model.FSBlock2To1, nil, nil},
		{uid, uid + 1, 0, nil, nil},
		{uid, uid + 1, 999, nil, errorsx.ErrInvalidParams},
		{uid, uid + 1, -999, nil, errorsx.ErrInvalidParams},
		{uid + 1, uid, model.FSBlock2To1, nil, errorsx.ErrAlreadyBlock},
		{uid + 1, uid, model.FSBothBlocked, nil, errorsx.ErrAlreadyBlock},
		{uid + 1, uid, model.FSNull, nil, nil},
		{uid + 1, uid, model.FSReq1To2, nil, nil},
		{uid + 1, uid, model.FSReq2To1, nil, nil},
		{uid + 1, uid, model.FSAdded, nil, nil},
		{uid + 1, uid, model.FSBlock1To2, nil, nil},
	}

	for i, tt := range tests {
		friend.Status = tt.status
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.expected == nil {
			mockf.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
			mockf.EXPECT().GetUser(gomock.Any()).Return(user, nil)
		}
		t.Run(fmt.Sprintf("block friend %d", i), func(t *testing.T) {
			err := f.Block(tt.from, tt.to)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.HandleBlock
				message := &ws.HandleBlockMsg{
					Type:  ws.HandleBlock,
					From:  tt.from,
					To:    tt.to,
					Block: true,
				}
				assert.Equal(t, message, msg)
				chatMsg := <-mock.Message
				chatMessage := &ws.ChatMsg{
					Type: ws.System,
					From: tt.to,
					To:   tt.from,
					Time: chatMsg.Time,
					Body: fmt.Sprintf("将 %s 添加到黑名单", name[tt.to]),
				}
				assert.Equal(t, chatMessage, chatMsg)
			}
		})
	}
}

func TestUnblock(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		status   int
		mock     error
		expected error
	}{
		{uid, uid + 1, model.FSNull, nil, errorsx.ErrNoBlocked},
		{uid, uid + 1, model.FSReq1To2, nil, errorsx.ErrNoBlocked},
		{uid, uid + 1, model.FSReq2To1, nil, errorsx.ErrNoBlocked},
		{uid, uid + 1, model.FSAdded, nil, errorsx.ErrNoBlocked},
		{uid, uid + 1, model.FSBlock2To1, nil, errorsx.ErrNoBlocked},
		{uid, uid + 1, model.FSBothBlocked, nil, nil},
		{uid, uid + 1, model.FSBlock1To2, nil, nil},
		{uid, uid + 1, 0, nil, errorsx.ErrNoBlocked},
		{uid, uid + 1, 999, nil, errorsx.ErrInvalidParams},
		{uid, uid + 1, -999, nil, errorsx.ErrInvalidParams},
		{uid + 1, uid, model.FSNull, nil, errorsx.ErrNoBlocked},
		{uid + 1, uid, model.FSReq1To2, nil, errorsx.ErrNoBlocked},
		{uid + 1, uid, model.FSReq2To1, nil, errorsx.ErrNoBlocked},
		{uid + 1, uid, model.FSAdded, nil, errorsx.ErrNoBlocked},
		{uid + 1, uid, model.FSBlock1To2, nil, errorsx.ErrNoBlocked},
		{uid + 1, uid, model.FSBothBlocked, nil, nil},
		{uid + 1, uid, model.FSBlock2To1, nil, nil},
	}

	for i, tt := range tests {
		friend.Status = tt.status
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.expected == nil {
			mockf.EXPECT().UpdateStatus(gomock.Any()).Return(tt.mock)
			mockf.EXPECT().GetUser(gomock.Any()).Return(user, nil)
		}
		t.Run(fmt.Sprintf("unblock friend %d", i), func(t *testing.T) {
			err := f.Unblock(tt.from, tt.to)
			assert.Equal(t, tt.expected, err)
			if tt.expected == nil {
				msg := <-mock.HandleBlock
				message := &ws.HandleBlockMsg{
					Type: ws.HandleBlock,
					From: tt.from,
					To:   tt.to,
				}
				assert.Equal(t, message, msg)
				chatMsg := <-mock.Message
				chatMessage := &ws.ChatMsg{
					Type: ws.System,
					From: tt.to,
					To:   tt.from,
					Time: chatMsg.Time,
					Body: fmt.Sprintf("将 %s 移出了黑名单", name[tt.to]),
				}
				assert.Equal(t, chatMessage, chatMsg)
			}
		})
	}
}

func TestRemark(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		value    string
		mockAll  bool
	}{
		{uid, uid + 1, "", false},
		{uid, uid + 1, "abc", true},
		{uid, uid + 1, "aaaa", true},
		{uid, uid + 1, "!@#$%^&*", true},
		{uid, uid + 1, strings.Repeat("a", 8), true},
		{uid + 1, uid, "", false},
		{uid + 1, uid, "abc", true},
		{uid + 1, uid, "aaaa", true},
		{uid + 1, uid, "!@#$%^&*", true},
		{uid + 1, uid, strings.Repeat("a", 8), true},
	}

	for i, tt := range tests {
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.mockAll {
			mockf.EXPECT().UpdateRemarkOrGroup(gomock.Any()).Return(nil)
		}
		t.Run(fmt.Sprintf("remark %d", i), func(t *testing.T) {
			err := f.Remark(tt.from, tt.to, tt.value)
			assert.NoError(t, err)
		})
	}

	t.Run("remark value is too long", func(t *testing.T) {
		err := f.Remark(uid, uid+1, strings.Repeat("a", 9))
		assert.Equal(t, errors.New("备注长度不能超过8个字符"), err)
	})
}

func TestSetGroup(t *testing.T) {
	setup(t)
	defer clear(t)

	tests := []struct {
		from, to uint
		value    string
		mockAll  bool
	}{
		{uid, uid + 1, "", false},
		{uid, uid + 1, "abc", true},
		{uid, uid + 1, "aaaa", true},
		{uid, uid + 1, "!@#$%^&*", true},
		{uid, uid + 1, strings.Repeat("a", 20), true},
		{uid + 1, uid, "", false},
		{uid + 1, uid, "abc", true},
		{uid + 1, uid, "aaaa", true},
		{uid + 1, uid, "!@#$%^&*", true},
		{uid + 1, uid, strings.Repeat("a", 20), true},
	}

	for i, tt := range tests {
		mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil)
		if tt.mockAll {
			mockf.EXPECT().UpdateRemarkOrGroup(gomock.Any()).Return(nil)
		}
		t.Run(fmt.Sprintf("set group %d", i), func(t *testing.T) {
			err := f.SetGroup(tt.from, tt.to, tt.value)
			assert.NoError(t, err)
		})
	}

	t.Run("group value is too long", func(t *testing.T) {
		err := f.SetGroup(uid, uid+1, strings.Repeat("a", 21))
		assert.Equal(t, errors.New("群组名称长度不能超过20个字符"), err)
	})

	friend1 := &model.Friend{User1: uid, User2: uid + 1, Version: 1}
	t.Run("retry set group", func(t *testing.T) {
		gomock.InOrder(
			mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend, nil),
			mockf.EXPECT().UpdateRemarkOrGroup(gomock.Any()).Return(errors.New("error")),
			mockf.EXPECT().QueryStatus(uid, uid+1).Return(friend1, nil),
			mockf.EXPECT().UpdateRemarkOrGroup(gomock.Any()).Return(nil),
		)
		err := f.SetGroup(uid, uid+1, "aaaa")
		assert.NoError(t, err)
	})
}

func TestFriendList(t *testing.T) {
	setup(t)
	defer clear(t)

	data := []*model.SummaryFriendInfo{
		{UID: uid + 1, Username: "test2", Group: "GroupA", Status: model.FSAdded},
		{UID: uid + 2, Username: "test1", Group: "GroupB", Status: model.FSAdded},
		{UID: uid + 3, Username: "test4", Group: "", Status: model.FSBlock1To2},
		{UID: uid + 4, Username: "test3", Group: "", Status: model.FSBlock1To2},
		{UID: uid + 5, Username: "test6", Group: "", Status: model.FSBlock2To1},
		{UID: uid + 6, Username: "test5", Group: "", Status: model.FSBlock2To1},
		{UID: uid + 7, Username: "test8", Group: "", Status: model.FSBothBlocked},
		{UID: uid + 8, Username: "test7", Group: "", Status: model.FSBothBlocked},
		{UID: uid + 9, Username: "test10", Group: "", Status: model.FSReq1To2},
		{UID: uid + 10, Username: "test9", Group: "", Status: model.FSReq1To2},
		{UID: uid + 11, Username: "test12", Group: "", Status: model.FSReq2To1},
		{UID: uid + 12, Username: "test11", Group: "", Status: model.FSReq2To1},
		{UID: uid + 13, Username: "test14", Group: "", Status: model.FSAdded},
		{UID: uid + 14, Username: "test13", Group: "", Status: model.FSAdded},
		{UID: uid + 15, Username: "test16", Group: "GroupA", Status: model.FSAdded},
		{UID: uid + 16, Username: "test15", Group: "GroupB", Status: model.FSAdded},
	}

	expected := map[string][]*model.SummaryFriendInfo{
		"GroupA":        {data[0], data[14]},
		"GroupB":        {data[1], data[15]},
		"default":       {data[12], data[13]},
		"blocked":       {data[2], data[3], data[6], data[7]},
		"request":       {data[10], data[11]},
		"other_blocked": {data[4], data[5]},
	}

	t.Run("friend list: unsorted", func(t *testing.T) {
		mockf.EXPECT().List(uid).Return(data, nil)
		got, err := f.List(uid)
		assert.NoError(t, err)
		assert.Equal(t, expected, got)
	})

	tests := []struct {
		name        string
		mock        []*model.SummaryFriendInfo
		mockErr     error
		expected    map[string][]*model.SummaryFriendInfo
		expectedErr error
	}{
		{"friend list: no data", []*model.SummaryFriendInfo{}, nil, make(map[string][]*model.SummaryFriendInfo), nil},
		{"friend list: timeout", nil, errorsx.ErrOperactionTimeout, nil, errorsx.ErrOperactionTimeout},
	}

	for _, tt := range tests {
		mockf.EXPECT().List(uid).Return(tt.mock, tt.mockErr)
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.List(uid)
			assert.Equal(t, tt.expectedErr, err)
			// if !reflect.DeepEqual(tt.expected, got) {
			// 	t.Errorf("\nwant: %v\ngot:  %v", tt.expected, got)
			// }
			assert.Equal(t, tt.expected, got)
		})
	}

	t.Run("friend list: retry", func(t *testing.T) {
		gomock.InOrder(
			mockf.EXPECT().List(uid).Return([]*model.SummaryFriendInfo{}, errors.New("error")),
			mockf.EXPECT().List(uid).Return([]*model.SummaryFriendInfo{}, errors.New("error")),
			mockf.EXPECT().List(uid).Return(data, nil),
		)
		got, err := f.List(uid)
		assert.NoError(t, err)
		assert.Equal(t, expected, got)
	})
}

// func TestBlockMeList(t *testing.T) {
// 	setup(t)
// 	defer clear(t)
// }

func TestSearch(t *testing.T) {
	setup(t)
	defer clear(t)

	data := []*model.Friendinfo{
		{ID: uid + 1, Username: "test2", Group: "GroupA", Status: model.FSAdded},
		{ID: uid + 2, Username: "test1", Group: "GroupB", Status: model.FSAdded},
	}

	tests := []struct {
		value       string
		cursor      *model.Cursor
		mock        []*model.Friendinfo
		expected    map[string]any
		expectedErr error
	}{
		{"", nil, nil, nil, errorsx.ErrInputEmpty},
		{"test", &model.Cursor{PageSize: 0, LastID: 0, HasMore: true}, nil, nil, errorsx.ErrPageSizeTooSmall},
		{"test", &model.Cursor{PageSize: 31, LastID: 0, HasMore: true}, nil, nil, errorsx.ErrPageSizeTooBig},
		{"test", &model.Cursor{PageSize: 1, LastID: 0, HasMore: false}, nil, nil, nil},
		{"test", &model.Cursor{PageSize: 15, LastID: 0, HasMore: true}, data, map[string]any{"cursor": &model.Cursor{PageSize: 15, LastID: 0, HasMore: true}, "data": data}, nil},
	}

	for i, tt := range tests {
		if tt.mock != nil {
			mockf.EXPECT().Search(uid, tt.value, tt.cursor).Return(&model.Cursor{PageSize: 15, LastID: 0, HasMore: true}, tt.mock, nil)
		}
		t.Run(fmt.Sprintf("search friend %d", i), func(t *testing.T) {
			data, err := f.Search(uid, tt.value, tt.cursor)
			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expected, data)
		})
	}
}
