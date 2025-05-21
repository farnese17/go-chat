package repository

import (
	"container/heap"
	"encoding/json"
	"errors"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

type Cache interface {
	StartFlush()
	Stop()
	Get(key string) (string, error)
	Set(key string, val any, expire time.Duration)
	StorePendingMessage(message any, sign int64)
	StoreOfflineMessage(id uint, message any)
	GetOfflineMessages(id uint) ([]string, error)
	GetPendingMessages() ([]string, error)
	RemoveOfflineMessage(id uint, message string)
	RemovePendingMessage(msgID string, receiver uint, sign int64) error
	GetMembersAndCache(gid uint) ([]uint, error)
	GetMembers(gid uint) ([]uint, error)
	GetAdmin(gid uint) ([]uint, error)
	AddMember(gid, member uint, role int)
	AddMemberIfKeyExist(gid, member uint, role int) error
	RemoveMember(gid, member uint) error
	SetGroupLastActiveTime(gid uint, lasttime int64)
	RemoveGroupLastActiveTime(gid uint)
	SetExpiration(key string, expire time.Duration)
	Remove(key string)
	Flush() error

	SetToken(id uint, token string, expire time.Duration)
	GetToken(id uint) (string, error)
	SetBanned(id string, level int, expire time.Duration)
	IsBanned(id uint) bool
	IsBanPermanent(id uint) bool
	IsBanMuted(id uint) bool
	BFM() BloomFilter
	GetBanned(cursor uint64) (
		newCursor uint64, ids []uint, level []int, ttl []int64, err error)
}

type RedisCache struct {
	client             *redis.Client
	pipe               redis.Pipeliner
	done               chan struct{}
	count              *int32
	flush              chan struct{}
	service            Service
	bloomFilterManager BloomFilter
}

func NewRedisCache(client *redis.Client, service Service) Cache {
	cache := &RedisCache{
		client:             client,
		pipe:               client.Pipeline(),
		done:               make(chan struct{}),
		count:              new(int32),
		flush:              make(chan struct{}),
		service:            service,
		bloomFilterManager: newBloomFilterManager(100000, blhash1, blhash2, blhash3),
	}
	go cache.startSync()
	return cache
}

func (rc *RedisCache) StartFlush() {
	inerval := rc.service.Config().Cache().AutoFlushInterval()
	ticker := time.NewTicker(inerval)
	defer ticker.Stop()
	defer close(rc.flush)

	for {
		select {
		case <-ticker.C:
			count := atomic.LoadInt32(rc.count)
			if count > 0 {
				rc.execAndReset()
			}
		case <-rc.flush:
			rc.execAndReset()
			ticker.Reset(inerval)
		case <-rc.done:
			return
		}
	}
}

func (rc *RedisCache) Stop() {
	rc.BFM().Stop()
	close(rc.done)
	rc.set(m.CacheLatestWarmTime, time.Now().Unix(), 0)
	rc.execAndReset()
}
func (rc *RedisCache) Flush() error {
	atomic.StoreInt32(rc.count, 0)
	_, err := rc.pipe.Exec()
	if err != nil {
		rc.service.Logger().Error("Failed to flush cache", zap.Error(err))
		return err
	}
	return nil
}

func (rc *RedisCache) execAndReset() {
	atomic.StoreInt32(rc.count, 0)
	if _, err := rc.pipe.Exec(); err != nil {
		rc.service.Logger().Error("Failed to submit redis pipe", zap.Error(err))
	}
}

func (rc *RedisCache) Get(key string) (string, error) {
	return rc.get(key)
}
func (rc *RedisCache) Set(key string, val any, expire time.Duration) {
	rc.set(key, val, expire)
}

func (rc *RedisCache) StorePendingMessage(message any, sign int64) {
	key := m.CacheMessagePending
	rc.storeToSortedSet(key, float64(sign), message)
}

// message
func (rc *RedisCache) StoreOfflineMessage(id uint, message any) {
	key := m.CacheMessage + strconv.Itoa(int(id))
	rc.storeToSet(key, message)
}
func (rc *RedisCache) GetOfflineMessages(id uint) ([]string, error) {
	key := m.CacheMessage + strconv.Itoa(int(id))
	data, err := rc.getFromSet(key)
	if err != nil {
		if errors.Is(err, errorsx.ErrNotFound) {
			return data, nil
		}
		rc.service.Logger().Error("Failed to get cache message", zap.Error(err))
		return data, err
	}
	return data, nil
}
func (rc *RedisCache) RemoveOfflineMessage(id uint, message string) {
	key := m.CacheMessage + strconv.Itoa(int(id))
	rc.removeFromSet(key, message)
}

func (rc *RedisCache) GetPendingMessages() ([]string, error) {
	key := m.CacheMessagePending
	count := rc.service.Config().Common().ResendBatchSize()
	timeout := rc.service.Config().Common().MessageAckTiemout()
	t := time.Now().Add(-timeout).UnixMilli()
	max := strconv.FormatInt(t, 10)
	result, err := rc.client.ZRangeByScore(key, redis.ZRangeBy{Min: "-inf", Max: max, Offset: 0, Count: count + 1}).Result()
	if err != nil {
		rc.service.Logger().Error("Failed to get pending messages", zap.Error(err))
		return nil, err
	}
	return result, nil
}

func (rc *RedisCache) RemovePendingMessage(msgID string, receiver uint, sign int64) error {
	key := m.CacheMessagePending
	script := redis.NewScript(`
		local key = KEYS[1]
		local msgID = ARGV[1]
		local receiver = tonumber(ARGV[2])
		local score = ARGV[3]

		local offset = 0
		while true do
			local msgs = redis.call("ZRANGEBYSCORE",key,score,score,"limit",offset,30)
			if #msgs == 0 then
				return
			end
			for i=1,#msgs do
				local msg = cjson.decode(msgs[i])
				if msg.id == msgID and msg.to == receiver then 
					redis.call("ZREM",key,msgs[i])
					return 
				end
			end

			if #msgs < 30 then 
				return
			end
			offset = offset + 30
		end
	`)

	var err error
	maxTries := rc.service.Config().Common().MaxRetries()
	for i := 0; i < maxTries; i++ {
		err = script.Run(rc.client, []string{key}, msgID, receiver, sign).Err()
		if err == nil {
			return nil
		}
		rc.service.Logger().Error("Failed to remove pending message", zap.Error(err))
		delay := rc.service.Config().Cache().RetryDelay(i)
		time.Sleep(delay)
	}
	return err
}

// 获取群组成员并缓存
func (rc *RedisCache) GetMembersAndCache(gid uint) ([]uint, error) {
	if members := rc.getMembers(gid, "0", "+inf"); members != nil {
		return members, nil
	}
	return rc.getMembersAndCache(gid)
}

// 获取群组管理员并缓存所有成员
func (rc *RedisCache) GetAdmin(gid uint) ([]uint, error) {
	if members := rc.getMembers(gid, "1", "2"); members != nil {
		return members, nil
	}
	return rc.getMembersAndCache(gid, m.GroupRoleAdmin, m.GroupRoleOwner)
}

// 获取群组成员但不缓存
func (rc *RedisCache) GetMembers(gid uint) ([]uint, error) {
	if members := rc.getMembers(gid, "0", "+inf"); members != nil {
		return members, nil
	}
	members, err := rc.getMembersFromDB(gid)
	if err != nil {
		return nil, err
	}
	return rc.filterMembers(members), nil
}

// default select member,admin,owner
func (rc *RedisCache) getMembersAndCache(gid uint, want ...int) ([]uint, error) {
	members, err := rc.getMembersFromDB(gid)
	if err != nil {
		return nil, err
	}

	key := m.CacheGroup + strconv.FormatUint(uint64(gid), 10)
	rc.SetExpiration(key, groupCacheExpire())
	// 防止缓存穿透，插入一个带有极小分数的成员作为标记
	if len(members) == 0 {
		rc.AddMember(gid, 0, -9999)
		return nil, nil
	}

	rc.SetGroupLastActiveTime(gid, time.Now().Unix()) // set lasttime
	for _, m := range members {
		rc.AddMember(gid, m.MemberID, m.Role) // cache member
	}
	rc.service.Logger().Info("Add group cache")

	rc.evictGroupCache()
	return rc.filterMembers(members, want...), nil
}

// default select member,admin,owner
func (rc *RedisCache) filterMembers(members []*m.GroupMemberRole, want ...int) []uint {
	result := make([]uint, len(members))
	if len(want) == 0 {
		want = append(want, m.GroupRoleMember, m.GroupRoleAdmin, m.GroupRoleOwner)
	}
	for i, member := range members {
		if slices.Contains(want, member.Role) {
			result[i] = member.MemberID
		}
	}
	return result
}

func (rc *RedisCache) evictGroupCache() {
	/*⁃ lua脚本
	  ⁃   插入缓存
	  ⁃   插入群组最后活跃时间(groups)、群组成员(group)
	  ⁃   检查缓存的群组数量是否超出限制
	  ⁃   检查groups长度，超出则获取前n个索引
	  ⁃   超出则删除最早活跃时间的n个群组
	  ⁃   删除n个group
	  ⁃   删除groups前n个索引 */
	/*
	   -- KEYS[1]: groups 键
	   -- KEYS[2]: 群组缓存键
	   -- ARGV[1]: 缓存数量限制
	*/
	evictGroupCache := redis.NewScript(`
    local groupsKey = KEYS[1]
    local membersKey = KEYS[2]
    local maxGroups = tonumber(ARGV[1])

    local groupCount = redis.call("ZCARD",groupsKey)
    if groupCount <= maxGroups then
         return 0
    end

    local removeCount = groupCount - maxGroups
    local oldestGroup = redis.call("ZRANGE",groupsKey,0,removeCount-1)
    for i,gid in ipairs(oldestGroup) do 
        redis.call("DEL",membersKey..gid)
        redis.call("ZREM",groupsKey,gid)
    end 
    return removeCount
    `)

	if result, err := evictGroupCache.Run(rc.client,
		[]string{m.CacheGroups, m.CacheGroup},
		rc.service.Config().Cache().MaxGroups()).Result(); err != nil {
		rc.service.Logger().Error("Failed to evict cache", zap.Error(err))
	} else if count, ok := result.(int64); ok {
		rc.service.Logger().Info("Evicted cache", zap.Int64("count", count))
	}
}

func (rc *RedisCache) getMembersFromDB(gid uint) ([]*m.GroupMemberRole, error) {
	members, err := rc.service.Group().GetMembersID(gid)
	if err != nil {
		rc.service.Logger().Error("Failed to get members", zap.Error(err))
		return nil, errorsx.ErrFailed
	}
	return members, nil
}

func (rc *RedisCache) SetGroupLastActiveTime(gid uint, lasttime int64) {
	rc.storeToSortedSet(m.CacheGroups, float64(lasttime), gid)
}
func (rc *RedisCache) RemoveGroupLastActiveTime(gid uint) {
	key := m.CacheGroups
	rc.removeFromSortedSet(key, gid)
}
func (rc *RedisCache) AddMember(gid, member uint, role int) {
	key := m.CacheGroup + strconv.FormatUint(uint64(gid), 10)
	rc.storeToSortedSet(key, float64(role), member)
}
func (rc *RedisCache) AddMemberIfKeyExist(gid, member uint, role int) error {
	/*
	   KEYS[1]: key
	   ARGV[1]: memberID
	   ARGV[2]: role
	*/
	insertOrDiscard := redis.NewScript(`
    local key = KEYS[1]
    local member = ARGV[1]
    local score = ARGV[2]

    local exist = redis.call("EXISTS",key)
    if exist == 1 then
        return redis.call("ZADD",key,score,member)
    end
    return -1
    `)

	var err error
	key := m.CacheGroup + strconv.FormatUint(uint64(gid), 10)
	maxRetries := rc.service.Config().Common().MaxRetries()
	for try := 0; try < maxRetries; try++ {
		err = insertOrDiscard.Run(rc.client, []string{key}, member, role).Err()
		if err == nil || errors.Is(err, redis.Nil) {
			return nil
		}
		delay := rc.service.Config().Cache().RetryDelay(try)
		time.Sleep(delay)
	}

	return errorsx.HandleError(err)
}
func (rc *RedisCache) RemoveMember(gid, member uint) error {
	key := m.CacheGroup + strconv.FormatUint(uint64(gid), 10)
	rc.removeFromSortedSet(key, member)
	return rc.Flush()
}
func (rc *RedisCache) getMembers(gid uint, start, end string) []uint {
	var members []uint
	key := m.CacheGroup + strconv.FormatUint(uint64(gid), 10)
	data, err := rc.getFromSortedSet(key, start, end)
	if err == nil {
		rc.service.Logger().Info("Hit cache")
		for _, m := range data {
			id, _ := strconv.ParseUint(m, 10, 64)
			members = append(members, uint(id))
		}
		rc.SetExpiration(key, groupCacheExpire())
		return members
	} else if !errors.Is(err, errorsx.ErrNotFound) {
		rc.service.Logger().Error("Get cache error", zap.Error(err))
	} else {
		rc.service.Logger().Info("Cache miss")
	}
	return nil
}

func (rc *RedisCache) Remove(key string) {
	rc.remove(key)
}

func (r *RedisCache) storeToSet(key string, value any) {
	jsonData, _ := json.Marshal(value)
	r.pipe.SAdd(key, jsonData)
	r.incrCount(1)
}

func (r *RedisCache) storeToSortedSet(key string, score float64, member any) {
	jsonData, _ := json.Marshal(member)
	r.pipe.ZAdd(key, redis.Z{
		Score:  score,
		Member: jsonData,
	})
	r.incrCount(1)
}

func (r *RedisCache) getFromSet(key string) ([]string, error) {
	data, err := r.client.SMembers(key).Result()
	return data, r.handleError(err)
}

func (r *RedisCache) getFromSortedSet(key string, start, end string) ([]string, error) {
	data, err := r.client.ZRangeByScore(key, redis.ZRangeBy{
		Min: start,
		Max: end,
	}).Result()
	return data, r.handleError(err)
}

// token
func (rc *RedisCache) SetToken(id uint, token string, expire time.Duration) {
	key := m.CacheToken + strconv.FormatUint(uint64(id), 10)
	rc.set(key, token, expire)
}
func (rc *RedisCache) GetToken(id uint) (string, error) {
	key := m.CacheToken + strconv.FormatUint(uint64(id), 10)
	return rc.get(key)
}
func (rc *RedisCache) SetBanned(id string, level int, expire time.Duration) {
	key := m.CacheBanned + id
	rc.set(key, level, expire)
}
func (rc *RedisCache) IsBanned(id uint) bool {
	level := rc.isBanned(id)
	return level == m.BanLevelPermanent || level == m.BanLevelTemporary
}
func (rc *RedisCache) IsBanPermanent(id uint) bool {
	return rc.isBanned(id) == m.BanLevelPermanent
}
func (rc *RedisCache) IsBanMuted(id uint) bool {
	return rc.isBanned(id) == m.BanLevelMuted
}
func (rc *RedisCache) isBanned(id uint) int {
	key := m.CacheBanned + strconv.FormatUint(uint64(id), 10)
	val, _ := rc.get(key)
	level, _ := strconv.ParseInt(val, 10, 64)
	return int(level)
}
func (rc *RedisCache) set(key string, val any, expire time.Duration) {
	rc.pipe.Set(key, val, expire)
	rc.incrCount(1)
}
func (rc *RedisCache) get(key string) (string, error) {
	maxRetries := rc.service.Config().Common().MaxRetries()
	var val string
	var err error
	for try := 0; try < maxRetries; try++ {
		val, err = rc.client.Get(key).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			delay := rc.service.Config().Cache().RetryDelay(try)
			time.Sleep(delay)
			continue
		}
		return val, rc.handleError(err)
	}
	return val, rc.handleError(err)
}

func (r *RedisCache) remove(key string) {
	r.pipe.Del(key)
	r.incrCount(1)
}

func (r *RedisCache) removeFromSet(key string, value any) {
	r.pipe.SRem(key, value)
	r.incrCount(1)
}

func (r *RedisCache) removeFromSortedSet(key string, value any) {
	r.pipe.ZRem(key, value)
	r.incrCount(1)
}

func (r *RedisCache) SetExpiration(key string, timeout time.Duration) {
	r.pipe.Expire(key, timeout)
	r.incrCount(1)
}
func (rc *RedisCache) GetBanned(cursor uint64) (
	newCursor uint64, ids []uint, level []int, ttl []int64, err error) {
	/*
	   KEYS[1] = pattern
	   ARGV[1] = cursor
	   ARGV[2] = count
	*/
	getBanLevelAndTTL := redis.NewScript(`
            local keys = KEYS[1]
            local cursor = tonumber(ARGV[1])
            local count = tonumber(ARGV[2])

            local k = redis.call("SCAN",cursor,"MATCH",keys,"COUNT",count)
            local id = {}
            local values = {}
            local ttl = {}
    
            for i,key in ipairs(k[2]) do
                id[i] = tonumber(string.sub(key,string.len(keys),-1))
                values[i] = tonumber(redis.call("GET",key))
                ttl[i] = redis.call("TTL",key)
            end
            return {tonumber(k[1]),id,values,ttl}
        `)
	v, err := getBanLevelAndTTL.Run(rc.client, []string{m.CacheBanned + "*"}, cursor, 3000).Result()
	if err != nil {
		return cursor, nil, nil, nil, err
	}
	newCursor = uint64(v.([]any)[0].(int64))
	gotID := v.([]any)[1].([]any)
	gotVals := v.([]any)[2].([]any)
	gotTTL := v.([]any)[3].([]any)
	for i := range gotID {
		ids = append(ids, uint(gotID[i].(int64)))
		level = append(level, int(gotVals[i].(int64)))
		ttl = append(ttl, gotTTL[i].(int64))
	}
	return
}
func (r *RedisCache) incrCount(n int32) {
	count := atomic.AddInt32(r.count, n)
	if count > r.service.Config().Cache().AutoFlushThreshold() {
		r.flush <- struct{}{}
	}
}

func (r *RedisCache) handleError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	r.service.Logger().Error(errStr)
	switch {
	case err == redis.Nil:
		return errorsx.ErrNotFound
	default:
		return err
	}
}

func (r *RedisCache) startSync() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	lastSyncTime := time.Now().Unix()
	for {
		select {
		case t := <-ticker.C:
			now := t.Unix()
			var cursor uint64
			for {
				data, newCursor, err := r.client.ZScan(m.CacheGroups, cursor, "*", 1000).Result()
				if err != nil {
					r.service.Logger().Error("Failed to get cache groups", zap.Error(err))
					break
				}

				var updates []*m.GroupLastActiveTime
				for i := 0; i < len(data); i += 2 {
					ts, _ := strconv.ParseInt(data[i+1], 10, 64)
					if ts > lastSyncTime {
						gid, _ := strconv.ParseUint(data[i], 10, 64)
						updates = append(updates, &m.GroupLastActiveTime{GID: uint(gid), LastTime: ts})
					}
				}
				if len(updates) > 0 {
					r.service.Group().UpdateLastTime(updates)
				}
				if newCursor == 0 {
					break
				}
				cursor = newCursor
			}
			lastSyncTime = now
		case <-r.done:
			r.service.Logger().Info("Turn off syncing last active time...")
			return
		}
	}
}

type warmer struct {
	service Service
}

func Warm(service Service) error {
	w := &warmer{service}
	fn := []func(Service) error{
		w.warmBloomFilterFromCache,
		w.warmBannedUser,
		w.warmGroups,
	}
	for _, fn := range fn {
		if err := fn(w.service); err != nil {
			return err
		}
	}
	w.service.Cache().Set(m.CacheLatestWarmTime, time.Now().Unix(), 0)
	w.service.Cache().Flush()
	return nil
}

func (w *warmer) warmBloomFilterFromCache(s Service) error {
	var cursor uint64
	count := 0
	for {
		c, ids, levels, ttl, err := s.Cache().GetBanned(cursor)
		if err != nil {
			w.service.Logger().Error("Feiled to warm bloom filter", zap.Error(err))
			return err
		}
		count += len(levels)
		for i, level := range levels {
			w.fillingBloomFilter(ids[i], level, ttl[i])
		}
		if c == 0 {
			break
		}
		cursor = c
	}
	w.service.Logger().Info("Warmed bloom filter from cache complete", zap.Int("count", count))
	return nil
}

func (w *warmer) fillingBloomFilter(id uint, level int, expire int64) {
	if level == m.BanLevelPermanent {
		w.service.Cache().BFM().BanUser(id)
	} else if level == m.BanLevelMuted {
		w.service.Cache().BFM().AddMute(id, expire)
	}
}
func (w *warmer) warmBannedUser(s Service) error {
	val, err := s.Cache().Get(m.CacheLatestWarmTime)
	if err != nil && !errors.Is(err, errorsx.ErrNotFound) {
		w.service.Logger().Error("Failed to warm banned user", zap.Error(err))
		return err
	}

	lastWarmTime, _ := strconv.ParseInt(val, 10, 64)
	cursor := &m.Cursor{PageSize: 100, LastID: 0, HasMore: true}
	count := 0
	for cursor.HasMore {
		users, c, err := s.User().GetBanned(cursor, lastWarmTime)
		if err != nil {
			w.service.Logger().Error("Failed to warm banned user", zap.Error(err))
			return err
		}
		count += len(users)
		now := time.Now().Unix()
		for _, user := range users {
			expire := user.BanExpireAt - now
			w.fillingBloomFilter(user.ID, user.BanLevel, user.BanExpireAt)
			key := m.CacheBanned + strconv.Itoa(int(user.ID))
			s.Cache().Set(key, user.BanLevel, time.Duration(expire)*time.Second)
		}
		cursor = c
		if err := s.Cache().Flush(); err != nil {
			return err
		}
	}
	w.service.Logger().Info("Warmed banned user from DB complete", zap.Int("count", count))
	return nil
}

func (w *warmer) warmGroups(s Service) error {
	val, err := s.Cache().Get(m.CacheLatestWarmTime)
	if err != nil && !errors.Is(err, errorsx.ErrNotFound) {
		w.service.Logger().Error("Failed to warm group members", zap.Error(err))
		return err
	}

	lastWarmTime, _ := strconv.ParseInt(val, 10, 64)
	groups, err := s.Group().Groups(s.Config().Cache().MaxGroups(), lastWarmTime)
	if err != nil {
		w.service.Logger().Error("Failed to warm group members: get groups", zap.Error(err))
		return err
	}
	count := 0
	for _, g := range groups {
		members, err := s.Group().Members(g.GID, 0, -1)
		if err != nil {
			w.service.Logger().Error("Failed to warm group members: get members", zap.Error(err))
			return err
		}
		count += len(members)
		s.Cache().SetGroupLastActiveTime(g.GID, g.LastTime)
		key := m.CacheGroup + strconv.FormatUint(uint64(g.GID), 10)
		for _, data := range members {
			s.Cache().AddMember(g.GID, data.ID, data.Role)
		}

		// 缓存过期抖动
		s.Cache().SetExpiration(key, groupCacheExpire())
	}
	if err := s.Cache().Flush(); err != nil {
		return err
	}
	w.service.Logger().Info("Warmed group members complete",
		zap.Int("groups", len(groups)), zap.Int("members", count))
	return nil
}

func groupCacheExpire() time.Duration {
	scope := []int{1, -1}
	jitter := time.Duration(rand.IntN(30)*scope[rand.IntN(2)]) * time.Second
	return jitter + time.Minute*15
}

func (rc *RedisCache) BFM() BloomFilter {
	return rc.bloomFilterManager
}

type BloomFilter interface {
	Start()
	Stop()
	BanUser(id uint)
	UnbanUser(id uint)
	IsBanned(id uint) bool
	AddMute(id uint, expireAt int64)
	IsMuted(id uint) bool
}

type bloomFilterManager struct {
	banned    *bloomFilter
	muted     *bloomFilter
	mutedList *muteList
	done      chan struct{}
}

func newBloomFilterManager(size int, f ...hashFunc) BloomFilter {
	return &bloomFilterManager{
		banned:    newBloomFilter(size, f...),
		muted:     newBloomFilter(size, f...),
		mutedList: newMuteList(),
		done:      make(chan struct{}),
	}
}
func (bfm *bloomFilterManager) Start() {
	go bfm.sendInvalidNotify()
}
func (bfm *bloomFilterManager) Stop() {
	close(bfm.done)
}
func (bfm *bloomFilterManager) BanUser(id uint) {
	bfm.banned.Add(id)
}
func (bfm *bloomFilterManager) UnbanUser(id uint) {
	bfm.banned.Remove(id)
}
func (bfm *bloomFilterManager) IsBanned(id uint) bool {
	return bfm.banned.Exist(id)
}

func (bfm *bloomFilterManager) AddMute(id uint, expireAt int64) {
	bfm.muted.Add(id)
	heap.Push(bfm.mutedList, &muteValidityPeriod{id, expireAt})
}
func (bfm *bloomFilterManager) IsMuted(id uint) bool {
	return bfm.muted.Exist(id)
}

type hashFunc func(string) uint32
type bloomFilter struct {
	bits     []uint8
	hashFunc []hashFunc
	n        int
	mu       sync.RWMutex
}

func newBloomFilter(size int, f ...hashFunc) *bloomFilter {
	return &bloomFilter{
		bits:     make([]uint8, size),
		hashFunc: f,
		n:        size,
		mu:       sync.RWMutex{},
	}
}

func (b *bloomFilter) Add(id uint) {
	b.mu.Lock()
	defer b.mu.Unlock()
	uid := strconv.Itoa(int(id))
	for _, f := range b.hashFunc {
		idx := f(uid) % uint32(b.n)
		if b.bits[idx] < math.MaxUint8 {
			b.bits[idx]++
		}
	}
}

// first check cache，second remove
func (b *bloomFilter) Remove(id uint) {
	b.mu.Lock()
	defer b.mu.Unlock()
	uid := strconv.Itoa(int(id))
	for _, f := range b.hashFunc {
		idx := f(uid) % uint32(b.n)
		if b.bits[idx] > 0 {
			b.bits[idx]--
		}
	}
}

func (b *bloomFilter) Exist(id uint) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	uid := strconv.Itoa(int(id))
	count := 0
	for _, f := range b.hashFunc {
		idx := f(uid) % uint32(b.n)
		if b.bits[idx] > 0 {
			count++
		}
	}
	return count == len(b.hashFunc)
}

func blhash1(s string) uint32 {
	h := fnv.New32()
	h.Write([]byte(s))
	return h.Sum32()
}

func blhash2(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func blhash3(s string) uint32 {
	h := fnv.New64()
	h.Write([]byte(s))
	return uint32(h.Sum64())
}

func (bfm *bloomFilterManager) sendInvalidNotify() {
	for {
		select {
		case <-bfm.done:
			return
		default:
			if bfm.mutedList.Len() > 0 {
				waiting := bfm.mutedList.Pop().(*muteValidityPeriod)
				t := waiting.expireAt - time.Now().Unix()
				if t > 0 {
					time.Sleep(time.Second * time.Duration(t))
				}
				bfm.muted.Remove(waiting.id)
			} else {
				// free time
				time.Sleep(time.Minute * 5)
			}
		}
	}
}

type muteValidityPeriod struct {
	id       uint
	expireAt int64
}
type muteList []*muteValidityPeriod

func newMuteList() *muteList {
	h := &muteList{}
	heap.Init(h)
	return h
}

func (h muteList) Len() int           { return len(h) }
func (h muteList) Less(i, j int) bool { return h[i].expireAt < h[j].expireAt }
func (h muteList) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *muteList) Push(x any)        { *h = append(*h, x.(*muteValidityPeriod)) }
func (h *muteList) Pop() any {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[:n-1]
	return x
}

type TestableCache interface {
	CountSet(key string) int64
	GetGroupsOrMembers(key string) ([]uint, error)
}

func (rc *RedisCache) CountSet(key string) int64 {
	c, _ := rc.client.SCard(key).Result()
	return c
}

func (rc *RedisCache) GetGroupsOrMembers(key string) ([]uint, error) {
	data, err := rc.client.ZRangeByScore(key, redis.ZRangeBy{
		Min: "-inf", Max: "+Inf"}).Result()
	if err != nil {
		return nil, err
	}

	res := make([]uint, len(data))
	for i := range data {
		id, _ := strconv.Atoi(data[i])
		res[i] = uint(id)
	}
	return res, err
}
