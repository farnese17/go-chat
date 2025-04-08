package repository_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/farnese17/chat/config"
	repo "github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/service/mock"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/logger"
	ws "github.com/farnese17/chat/websocket"
	"github.com/go-redis/redis"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

var (
	cache  repo.Cache
	u      *mock.MockUserRepository
	g      *mock.MockGroupRepository
	client *redis.Client
	// cfg    config.Config
	service *mock.MockService
)

func setupCache(t *testing.T) {
	cfg := config.LoadConfig(os.Getenv("CHAT_CONFIG"))
	logger := logger.SetupLogger()
	client = repo.SetupRedis(cfg)

	ctrl := gomock.NewController(t)
	service = mock.NewMockService(ctrl)
	u = mock.NewMockUserRepository(ctrl)
	g = mock.NewMockGroupRepository(ctrl)
	service.EXPECT().Logger().Return(logger).AnyTimes()
	service.EXPECT().Config().Return(cfg).AnyTimes()
	service.EXPECT().User().Return(u).AnyTimes()
	service.EXPECT().Group().Return(g).AnyTimes()

	cache = repo.NewRedisCache(client, service)
	service.EXPECT().Cache().Return(cache).AnyTimes()
	client.FlushAll()
}

func closeConnection() {
	client.Close()
}

func TestCacheMessageAndGet(t *testing.T) {
	setupCache(t)
	defer closeConnection()

	count := 100
	// get empty cache
	t.Run("cache messages are null", func(t *testing.T) {
		result, err := cache.GetMessage(uint(1e8))
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	var expected []*ws.ChatMsg
	// set cache
	t.Run("set messages cache", func(t *testing.T) {
		expected = generateCacheMessage(count)
		verifyCacheMessage(t, count, expected)
	})

	// remove cache
	t.Run("remove half of message cache", func(t *testing.T) {
		removeMessage(len(expected)/2, &expected)
		verifyCacheMessage(t, count/2, expected)
	})

	// remove all
	t.Run("remove all messages cache", func(t *testing.T) {
		removeMessage(len(expected), &expected)
		verifyCacheMessage(t, len(expected), expected)
	})
}

func TestGetMembersAndCache(t *testing.T) {
	setupCache(t)
	defer closeConnection()

	count := 100
	data := make([]*m.GroupMemberRole, count)
	for i := range count {
		role := m.GroupRoleMember
		if i == 0 {
			role = m.GroupRoleOwner
		} else if i < count/10 {
			role = m.GroupRoleAdmin
		}
		data[i] = &m.GroupMemberRole{MemberID: uint(1e5+1) + uint(i), Role: role}
	}

	var expectedGroups []uint
	t.Run("set members cache", func(t *testing.T) {
		for i := range count / 10 {
			gid := uint(1e9+1) + uint(i)
			expectedGroups = append(expectedGroups, gid)
			g.EXPECT().GetMembersID(gid).Return(data, nil)
			result, err := cache.GetMembersAndCache(gid)
			assert.NoError(t, err)
			assert.Equal(t, len(data), len(result))
			cache.Flush()
		}
	})

	// verify
	expected := make([]uint, count)
	for i := range data {
		expected[i] = data[i].MemberID
	}
	for i := range count / 10 {
		gid := uint(1e9+1) + uint(i)
		t.Run(fmt.Sprintf("verify members cache %d", gid), func(t *testing.T) {
			result, err := cache.GetMembers(gid)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		})
	}

	t.Run("verify groups cache", func(t *testing.T) {
		c := cache.(repo.TestableCache)
		result, err := c.GetGroupsOrMembers(m.CacheGroups)
		assert.NoError(t, err)
		assert.Equal(t, count/10, len(result))
		assert.Equal(t, expectedGroups, result)
	})

	t.Run("verify members role", func(t *testing.T) {
		expected := make([]uint, count/10)
		for i := range count / 10 {
			expected[i] = data[i].MemberID
		}
		result, err := cache.GetAdmin(uint(1e9 + 1))
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	newGroups := make([]uint, 0, count/10/2)
	t.Run("ecive group cache", func(t *testing.T) {
		service.Config().SetCache("max_groups", fmt.Sprintf("%d", count/10/2))
		for i := count / 10; i <= count/10+count/10/2; i++ {
			gid := uint(1e9+1) + uint(i)
			newGroups = append(newGroups, gid)
			g.EXPECT().GetMembersID(gid).Return(data, nil)
			result, err := cache.GetMembersAndCache(gid)
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
			cache.Flush()
		}
		for i := range newGroups {
			result, err := cache.GetMembers(newGroups[i])
			assert.NoError(t, err)
			assert.Equal(t, expected, result)
		}
		groups, err := cache.(repo.TestableCache).GetGroupsOrMembers(m.CacheGroups)
		assert.NoError(t, err)
		assert.Equal(t, newGroups, groups)
	})
}

func TestAddMember(t *testing.T) {
	setupCache(t)
	defer closeConnection()

	gid := uint(1e9 + 1)
	uid := uint(1e5 + 1)
	g.EXPECT().GetMembersID(gid).Return(nil, nil).AnyTimes()
	t.Run("key does not exist", func(t *testing.T) {
		err := cache.AddMemberIfKeyExist(gid, uid, m.GroupRoleMember)
		assert.NoError(t, err)
		result, err := cache.GetMembers(gid)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(result))
	})

	t.Run("add member", func(t *testing.T) {
		cache.AddMember(gid, uid, m.GroupRoleMember)
		cache.Flush()
	})

	t.Run("key exists", func(t *testing.T) {
		err := cache.AddMemberIfKeyExist(gid, uid+1, m.GroupRoleMember)
		assert.NoError(t, err)
		result, err := cache.GetMembers(gid)
		assert.NoError(t, err)
		expected := []uint{uid, uid + 1}
		assert.Equal(t, expected, result)
	})
}

func generateCacheMessage(count int) []*ws.ChatMsg {
	message := ws.ChatMsg{
		Type: ws.Chat,
		From: uint(1e5 + 1),
		Time: time.Now().UnixMilli(),
		Body: "abcd",
	}
	// cache message and generate expected result
	expected := make([]*ws.ChatMsg, count)
	for i := range count {
		id := uint(1e5+1) + uint(i)
		msg := message
		msg.To = id
		expected[i] = &msg
		cache.CacheMessage(id, msg)
	}
	// refresh cache
	cache.Flush()
	return expected
}

func verifyCacheMessage(t *testing.T, count int, expected []*ws.ChatMsg) {
	result := make([]*ws.ChatMsg, 0, count)
	for i := range expected {
		id := expected[i].To
		data, err := cache.GetMessage(id)
		assert.NoError(t, err)
		for _, d := range data {
			var msg *ws.ChatMsg
			json.Unmarshal([]byte(d), &msg)
			result = append(result, msg)
		}
	}
	assert.Equal(t, count, len(expected))
	assert.Equal(t, len(expected), len(result))
	assert.Equal(t, expected, result)
}

func removeMessage(count int, expected *[]*ws.ChatMsg) {
	for i := range count {
		id := (*expected)[i].To
		msg, _ := json.Marshal((*expected)[i])
		cache.RemoveMessage(id, string(msg))
	}
	cache.Flush()
	*expected = (*expected)[count:]
}

func TestCacheUnackMessage(t *testing.T) {
	setupCache(t)
	defer closeConnection()

	c := cache.(repo.BlockCache)
	count := 10
	msgs := make([]ws.HandleBlockMsg, count)
	t.Run("cache unack messages", func(t *testing.T) {
		service.Config().SetCommon("resend_interval", time.Nanosecond.String())
		for i := range count {
			msg := ws.HandleBlockMsg{
				Type: ws.HandleBlock,
				From: uint(1e5 + 1),
				To:   uint(1e5+1) + uint(i+1),
				Time: time.Now().UnixMilli(),
			}
			msgs[i] = msg
			c.CacheUnackMessage(service.Config().Common().ResendInterval(), msg)
		}
		cache.Flush()
	})

	t.Run("get unack messages", func(t *testing.T) {
		result, err := c.GetUnAck()
		assert.NoError(t, err)
		assert.Equal(t, len(msgs), len(result))
		for i, m := range result {
			var msg ws.HandleBlockMsg
			json.Unmarshal([]byte(m), &msg)
			msgs[i].Time = msg.Time
			assert.Equal(t, msgs[i], msg)
		}
	})

	t.Run("ack messages", func(t *testing.T) {
		for _, m := range msgs {
			c.Ack(m.From, m.To, m)
		}
		cache.Flush()
		result, err := c.GetUnAck()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(result))
	})
}

func TestBloomFilter(t *testing.T) {
	setupCache(t)
	defer closeConnection()

	count := 100
	ids := make([]uint, count)
	for i := range count {
		id := uint(1e5+1) + uint(i)
		ids[i] = id
		cache.BFM().BanUser(id)
	}
	cache.Flush()

	t.Run("banned", func(t *testing.T) {
		for i, id := range ids {
			got := cache.BFM().IsBanned(id)
			assert.Equal(t, true, got)

			got = cache.BFM().IsBanned(uint(i))
			assert.Equal(t, false, got)
		}
	})

	t.Run("unban", func(t *testing.T) {
		for _, id := range ids {
			cache.BFM().UnbanUser(id)
		}
		for _, id := range ids {
			got := cache.BFM().IsBanned(id)
			assert.Equal(t, false, got)
		}
	})

	t.Run("muted", func(t *testing.T) {
		expireAt := time.Now().Add(time.Second * 2).Unix()
		for _, id := range ids {
			cache.BFM().AddMute(id, expireAt)
		}
		for _, id := range ids {
			got := cache.BFM().IsMuted(id)
			assert.Equal(t, true, got)
		}

		go cache.BFM().Start()
		time.Sleep(time.Second * 3)

		for _, id := range ids {
			got := cache.BFM().IsMuted(id)
			assert.Equal(t, false, got)
		}
		cache.BFM().Stop()
	})

}

func TestWarmer(t *testing.T) {
	setupCache(t)
	defer closeConnection()

	go cache.StartFlush()
	defer cache.Stop()

	count := 3000
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := range count {
			id := fmt.Sprintf("%d", uint(1e5+1)+uint(i))
			cache.SetBanned(id, m.BanLevelPermanent, time.Hour)
		}
	}()
	go func() {
		defer wg.Done()
		for i := range count {
			id := fmt.Sprintf("%d", uint(1e5+1)+uint(i+count*2))
			cache.SetBanned(id, m.BanLevelMuted, time.Hour)
		}
	}()
	go func() {
		defer wg.Done()
		for i := range count / 3 {
			id := fmt.Sprintf("%d", uint(1e5+1)+uint(i+count*3))
			cache.SetBanned(id, m.BanLevelTemporary, time.Hour)
		}
	}()
	wg.Wait()
	cache.Flush()

	banData := make([]*m.BanStatus, 150)
	for i := range 150 {
		id := uint(1e5+1) + uint(i+count)
		banData[i] = &m.BanStatus{ID: id, BanLevel: m.BanLevelPermanent, BanExpireAt: time.Now().Add(time.Hour).Unix()}
	}
	groupData := make([]*m.GroupLastActiveTime, 10)
	for i := range 10 {
		gid := uint(1e9+1) + uint(i)
		groupData[i] = &m.GroupLastActiveTime{GID: gid, LastTime: 0}
	}
	members := make([]*m.MemberInfo, 10)
	for i := range 10 {
		id := uint(1e5+1) + uint(i)
		members[i] = &m.MemberInfo{ID: id, Role: m.GroupRoleMember}
	}
	gomock.InOrder(
		u.EXPECT().GetBanned(gomock.Any(), gomock.Any()).Return(banData[:100], &m.Cursor{PageSize: 100, LastID: banData[99].ID, HasMore: true}, nil),
		u.EXPECT().GetBanned(gomock.Any(), gomock.Any()).Return(banData[100:], &m.Cursor{PageSize: 100, LastID: banData[99].ID, HasMore: false}, nil),
		g.EXPECT().Groups(gomock.Any(), gomock.Any()).Return(groupData, nil),
		g.EXPECT().Members(gomock.Any(), gomock.Any(), gomock.Any()).Return(members, nil).Times(10),
	)

	t.Run("test warmer", func(t *testing.T) {
		err := repo.Warm(service)
		assert.NoError(t, err)
	})

	t.Run("verfity fill bloom filter", func(t *testing.T) {
		for _, d := range banData {
			got := cache.BFM().IsBanned(d.ID)
			assert.Equal(t, true, got)
		}

		for i := range count {
			id := uint(1e5+1) + uint(i)
			got := cache.BFM().IsBanned(id)
			assert.Equal(t, true, got)
		}

		for i := range count {
			id := uint(1e5+1) + uint(i+count*2)
			got := cache.BFM().IsMuted(id)
			assert.Equal(t, true, got)
		}
	})

	t.Run("verfity last warm time", func(t *testing.T) {
		got, err := cache.Get(m.CacheLatestWarmTime)
		assert.NoError(t, err)
		assert.NotEmpty(t, got)
	})

	t.Run("verfity groups", func(t *testing.T) {
		c := cache.(repo.TestableCache)
		got, err := c.GetGroupsOrMembers(m.CacheGroups)
		assert.NoError(t, err)
		assert.Equal(t, len(groupData), len(got))
		for i, g := range got {
			assert.Equal(t, groupData[i].GID, g)
		}
	})
	t.Run("verfity members", func(t *testing.T) {
		for _, g := range groupData {
			got, err := cache.GetMembers(g.GID)
			assert.NoError(t, err)
			assert.Equal(t, len(members), len(got))
			for i, g := range got {
				assert.Equal(t, members[i].ID, g)
			}
		}
	})

}
