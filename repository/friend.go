package repository

import (
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"gorm.io/gorm"
)

type FriendRepository interface {
	QueryStatus(id1, id2 uint) (*m.Friend, error)
	UpdateStatus(friend *m.Friend) error
	GetUser(id ...uint) ([]*m.User, error)
	UpdateRemarkOrGroup(friend *m.Friend) error
	Get(from, to uint) (*m.Friendinfo, error)
	Search(id uint, value string, cursor *m.Cursor) (*m.Cursor, []*m.Friendinfo, error)
	List(id uint) ([]*m.SummaryFriendInfo, error)
	BlockedMeList(id uint) ([]uint, error)
}

type SQLFriendRepository struct {
	db *gorm.DB
}

func NewSQLFriendRepository(db *gorm.DB) FriendRepository {
	return &SQLFriendRepository{db}
}

func (s *SQLFriendRepository) QueryStatus(id1, id2 uint) (*m.Friend, error) {
	var friend *m.Friend
	err := s.db.Where("user1= ? AND user2 = ?", id1, id2).
		First(&friend).Error
	if err := errorsx.HandleError(err); err != nil {
		return friend, err
	}
	return friend, nil
}

func (s *SQLFriendRepository) UpdateStatus(friend *m.Friend) error {
	result := s.db.Where("user1 = ? AND user2 = ? AND version = ?", friend.User1, friend.User2, friend.Version-1).
		Save(friend)
	if result.Error != nil {
		return errorsx.HandleError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}

func (s *SQLFriendRepository) GetUser(id ...uint) ([]*m.User, error) {
	var user []*m.User
	err := s.db.Model(&m.User{}).Limit(len(id)).
		Select("id,username").
		Where("id IN ?", id).
		Find(&user).Error

	if err := errorsx.HandleError(err); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *SQLFriendRepository) List(id uint) ([]*m.SummaryFriendInfo, error) {
	var friend []*m.SummaryFriendInfo
	err := s.db.Model(&m.Friend{}).
		Select(`u.id AS uid,u.username ,u.avatar,friend.status,friend.id,u.ban_level,u.ban_expire_at,
			if(user1 = ?,user1_remark,user2_remark) AS remark,
			if(user1 = ?,user1_group,user2_group) AS`+" `group` ", id, id).
		Joins(`LEFT JOIN`+" `user` "+` AS u ON u.id = if(user1 = ?,user2,user1)`, id).
		Where("user1 = ? OR user2 = ?", id, id).
		Find(&friend).Error
	return friend, err
}

func (s *SQLFriendRepository) Get(from, to uint) (*m.Friendinfo, error) {
	id1, id2 := from, to
	if from > to {
		id1, id2 = to, from
	}
	var friend *m.Friendinfo
	result := s.db.Model(&m.Friend{}).
		Select(`u.id AS uid,u.username,u.avatar,u.phone,u.email,friend.id,friend.status,u.ban_level,u.ban_expire_at,
			if(user1 = ?,user1_remark,user2_remark) AS remark,
			if(user1 = ?,user1_group,user2_group) AS`+" `group` ", from, from).
		Joins(`LEFT JOIN`+" `user` "+`AS u ON u.id = if(user1 = ?,user2,user1)`, from).
		Where("user1 = ? AND user2 = ?", id1, id2).
		First(&friend)

	if err := errorsx.HandleError(result.Error); err != nil {
		return nil, err
	}
	return friend, nil
}

func (s *SQLFriendRepository) UpdateRemarkOrGroup(friend *m.Friend) error {
	result := s.db.Where("user1 = ? AND user2 = ? AND version = ?",
		friend.User1, friend.User2, friend.Version-1).
		Updates(&friend) // where id

	if err := errorsx.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}

func (s *SQLFriendRepository) Search(id uint, value string, cursor *m.Cursor) (*m.Cursor, []*m.Friendinfo, error) {
	var result []*m.Friendinfo
	selected := " u.id,username,avatar,phone,email,u.created_at,ban_level,ban_expire_at,status,user1_remark,user2_remark,user1_group,user2_group,user1,user2 "
	sql := `
		SELECT u.id AS uid,username,avatar,u.created_at,ban_level,ban_expire_at,
			if(status IS NULL,10,status) AS status,
			if(user1 = ? OR user2 = ? OR u.id = ?,phone,NULL) AS phone,
			if(user1 = ? OR user2 = ? OR u.id = ?,email,NULL) AS email,
			if(user1 = ?,user1_remark,user2_remark) AS remark,
			if(user1 = ?,user1_group,user2_group) AS` + " `group` " +
		`FROM (
			(SELECT` + selected +
		`FROM user AS u
			 LEFT JOIN friend AS f ON user1 = if(u.id < ?,u.id,?) AND user2 = if(u.id < ?,?,u.id)
			 WHERE u.id = ? AND u.id > ? AND (status IS NULL OR status NOT IN ?))
			UNION
			(SELECT` + selected +
		`FROM user AS u
			 LEFT JOIN friend AS f ON user1 = if(u.id < ?,u.id,?) AND user2 = if(u.id < ?,?,u.id)
			 WHERE MATCH(username) AGAINST(? IN BOOLEAN MODE) AND u.id > ? AND (status IS NULL OR status NOT IN ?)
			 LIMIT ?)
			) AS u
		ORDER BY 
			CASE status 
			WHEN ? THEN 1
			WHEN ? THEN 2
			WHEN ? THEN 2
			WHEN 0 THEN 9
			ELSE
				if(uid = ?,0,10)
			END
	`
	err := s.db.Raw(sql,
		id, id, id,
		id, id, id,
		id, id,
		id, id, id, id, value, cursor.LastID, []int{m.FSBlock2To1, m.FSBlock1To2, m.FSBothBlocked},
		id, id, id, id, "*"+value+"*", cursor.LastID, []int{m.FSBlock2To1, m.FSBlock1To2, m.FSBothBlocked}, cursor.PageSize+1,
		m.FSAdded, m.FSReq1To2, m.FSBlock2To1, id).Scan(&result).Error
	if err := errorsx.HandleError(err); err != nil {
		return cursor, nil, err
	}
	if len(result) > cursor.PageSize {
		result = result[:len(result)-1]
		cursor.LastID = result[len(result)-1].UID
	} else {
		cursor.HasMore = false
	}

	return cursor, result, nil
}

func (s *SQLFriendRepository) BlockedMeList(id uint) ([]uint, error) {
	var data []uint
	err := s.db.Model(&m.Friend{}).
		Select("if(user1 = ?,user2,user1)", id).
		Where("(user1 = ? AND (`status` = ? OR `status` = ?)) OR (user2 = ? AND (`status` = ? OR `status` = ?))",
			id, m.FSBlock2To1, m.FSBothBlocked, id, m.FSBlock1To2, m.FSBothBlocked).
		Find(&data).Error

	if err := errorsx.HandleError(err); err != nil {
		return nil, err
	}
	return data, nil
}
