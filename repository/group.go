package repository

import (
	"fmt"
	"math"
	"strings"
	"time"

	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"gorm.io/gorm"
)

type GroupRepository interface {
	Create(group *m.Group) error
	SearchByID(gid uint) (*m.Group, error)
	SearchByName(name string, cursor *m.Cursor) ([]*m.Group, *m.Cursor, error)
	Delete(gid, uid uint) error
	GetMembersID(gid uint) ([]*m.GroupMemberRole, error)
	Apply(gid, inviteID, targetID uint) error
	CreateMember(ctx *m.MemberStatusContext) error
	DeleteMember(ctx *m.MemberStatusContext) error
	UpdateStatus(ctx *m.MemberStatusContext) error
	Members(gid, uid any, limit int) ([]*m.MemberInfo, error)
	Groups(limit int, lasttime int64) ([]*m.GroupLastActiveTime, error)
	UpdateLastTime(data []*m.GroupLastActiveTime) error
	QueryRole(gid uint, uid ...uint) ([]*m.GroupMemberRole, error)
	HandOverOwner(from, to uint, gid uint) error
	Update(from, gid uint, cloumn string, value string) error
	ReleaseAnnounce(data *m.GroupAnnouncement) error
	ViewAnnounce(gid, uid any, cursor *m.Cursor) ([]*m.GroupAnnounceInfo, *m.Cursor, error)
	DeleteAnnounce(gid, uid, announceID uint) error
	List(uid uint) ([]*m.SummaryGroupInfo, error)
}

type SQLGroupRepository struct {
	db *gorm.DB
}

func NewSQLGroupRepository(db *gorm.DB) GroupRepository {
	return &SQLGroupRepository{db}
}

func (s *SQLGroupRepository) Create(group *m.Group) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		if err := tx.Create(&m.GroupPerson{
			MemberID:  group.Owner,
			GroupID:   group.GID,
			Role:      m.GroupRoleOwner,
			InviterID: group.Owner}).Error; err != nil {
			return err
		}
		return nil
	})
	return errorsx.HandleError(err)
}

func (s *SQLGroupRepository) SearchByID(gid uint) (*m.Group, error) {
	var group *m.Group
	err := s.db.Where("gid = ?", gid).First((&group)).Error
	return group, errorsx.HandleError(err)
}

func (s *SQLGroupRepository) SearchByName(name string, cursor *m.Cursor) ([]*m.Group, *m.Cursor, error) {
	var groups []*m.Group
	if err := s.db.Where("`group`.gid > ? AND name LIKE ?", cursor.LastID, "%"+name+"%").Limit(cursor.PageSize + 1).
		Find(&groups).Error; err != nil {
		return nil, cursor, errorsx.HandleError(err)
	}
	if len(groups) > cursor.PageSize {
		groups = groups[:len(groups)-1]
		cursor.LastID = groups[len(groups)-1].GID
	} else {
		cursor.HasMore = false
	}
	return groups, cursor, nil
}

func (s *SQLGroupRepository) Delete(gid, uid uint) error {
	result := s.db.Where("gid = ? AND owner = ?", gid, uid).Delete(&m.Group{})
	if result.Error != nil {
		return errorsx.HandleError(result.Error)
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}

func (s *SQLGroupRepository) GetMembersID(gid uint) ([]*m.GroupMemberRole, error) {
	var uid []*m.GroupMemberRole
	err := s.db.Select("member_id,role").
		Model(&m.GroupPerson{}).
		Where("group_id = ? AND role > ?", gid, 0).Find(&uid).Error
	return uid, errorsx.HandleError(err)
}

func (s *SQLGroupRepository) Apply(gid, inviteID, targetID uint) error {
	member := m.GroupPerson{
		MemberID:  targetID,
		GroupID:   gid,
		Role:      m.GroupRoleApplied,
		InviterID: inviteID,
	}
	err := s.db.Create(&member).Error
	return errorsx.HandleError(err)
}

func (s *SQLGroupRepository) Members(gid, uid any, limit int) ([]*m.MemberInfo, error) {
	var members []*m.MemberInfo
	query := s.db.Model(&m.GroupPerson{}).
		Select("u.id,u.username,u.phone,u.email,u.avatar,group_person.role,group_person.created_at,u.ban_level,u.ban_expire_at").
		Joins("LEFT JOIN `user` AS u ON u.id = `group_person`.member_id").
		Where("group_id = ?", gid)

	if limit == 1 {
		query.Where("member_id = ?", uid).Limit(limit)
	}
	err := query.Order("group_person.role,u.username").Find(&members).Error
	return members, errorsx.HandleError(err)
}

func (s *SQLGroupRepository) Groups(limit int, lasttime int64) ([]*m.GroupLastActiveTime, error) {
	var groups []*m.GroupLastActiveTime
	err := s.db.Model(&m.Group{}).
		Select("gid,last_time").Limit(limit).
		Where("last_time >= ?", lasttime).
		Find(&groups).
		Order("last_time DESC").Error
	return groups, errorsx.HandleError(err)
}

func (s *SQLGroupRepository) UpdateLastTime(data []*m.GroupLastActiveTime) error {
	var ids []string
	caseStr := "CASE gid "
	for _, d := range data {
		caseStr += fmt.Sprintf("WHEN %d THEN %d ", d.GID, d.LastTime)
		ids = append(ids, fmt.Sprintf("%d", d.GID))
	}
	caseStr += "END"
	idStr := strings.Join(ids, ",")
	query := fmt.Sprintf("UPDATE `group` SET last_time = %s WHERE gid IN (%s)", caseStr, idStr)

	return s.db.Debug().Exec(query).Error
}

func (s *SQLGroupRepository) HandOverOwner(from, to uint, gid uint) error {
	tx := s.db.Begin()
	result := tx.Model(&m.GroupPerson{}).
		Where("group_id = ? AND member_id = ? AND role = ?",
			gid, from, m.GroupRoleOwner).
		Update("role", m.GroupRoleMember)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return errorsx.ErrPermissionDenied
	}

	result = tx.Model(&m.GroupPerson{}).
		Where("group_id = ? AND member_id = ? AND role IN ?",
			gid, to, []int{m.GroupRoleAdmin, m.GroupRoleMember}).
		Update("role", m.GroupRoleOwner)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return errorsx.ErrNoAffectedRows
	}

	result = tx.Model(&m.Group{}).
		Where("gid = ? AND owner = ?",
			gid, from).
		Update("owner", to)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected != 1 {
		tx.Rollback()
		return errorsx.ErrPermissionDenied
	}

	return tx.Commit().Error
}

func (s *SQLGroupRepository) QueryRole(gid uint, uid ...uint) ([]*m.GroupMemberRole, error) {
	var roles []*m.GroupMemberRole
	err := s.db.Model(&m.User{}).Limit(len(uid)).
		Select(`gp.id,
				user.id AS member_id,
				gp.role,
				gp.version,
				user.username,
				g.name AS groupname`).
		Joins("JOIN `group` AS g ON g.gid = ?", gid).
		Joins("LEFT JOIN group_person AS gp ON gp.group_id = g.gid AND gp.member_id = user.id").
		Where("user.id IN ?", uid).
		Find(&roles).Error
	if err := errorsx.HandleError(err); err != nil {
		return nil, err
	}
	return roles, nil
}

func (s *SQLGroupRepository) Update(from, gid uint, cloumn string, value string) error {
	result := s.db.Model(&m.Group{}).
		Joins("JOIN group_person AS gp ON gp.group_id = ? AND gp.member_id = ? AND gp.role IN ?", gid, from, []int{m.GroupRoleOwner, m.GroupRoleAdmin}).
		Where("gid = ?", gid).
		Update(cloumn, value)
	if err := errorsx.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}

func (s *SQLGroupRepository) ReleaseAnnounce(data *m.GroupAnnouncement) error {
	result := s.db.Create(data)
	if err := errorsx.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}

func (s *SQLGroupRepository) ViewAnnounce(gid, uid any, cursor *m.Cursor) ([]*m.GroupAnnounceInfo, *m.Cursor, error) {
	var announce []*m.GroupAnnounceInfo
	query := s.db.Table("`group_announcement` AS ga").
		Select(`ga.id,ga.content,ga.updated_at,
		IFNULL(u.username,ga.created_by) AS created_by`).
		Joins("JOIN group_person AS gp ON gp.group_id = ga.group_id AND gp.member_id = ? AND gp.role IN ?",
			uid, []int{m.GroupRoleMember, m.GroupRoleAdmin, m.GroupRoleOwner}).
		Joins("LEFT JOIN `user` AS u ON u.id = ga.created_by").
		Where("ga.group_id = ?", gid)

	limit := 1
	if cursor != nil {
		limit = cursor.PageSize + 1
		if cursor.LastID == 0 {
			cursor.LastID = math.MaxUint64
		}
		query.Where("ga.id < ?", cursor.LastID)
	}

	err := query.Limit(limit).Order("ga.id DESC").
		Find(&announce).Error
	if err := errorsx.HandleError(err); err != nil {
		return nil, cursor, err
	}

	if cursor != nil {
		if len(announce) > cursor.PageSize {
			announce = announce[:len(announce)-1]
			cursor.LastID = announce[len(announce)-1].ID
		} else {
			cursor.HasMore = false
		}
	}

	return announce, cursor, nil
}

// func (s *SQLGroupRepository) ViewLatestAnnounce(gid uint) (*m.GroupAnnounceInfo, error) {
// 	var announce *m.GroupAnnounceInfo
// 	err := s.db.Table("group_announcement AS ga").
// 		Select("ga.id,ga.content,ga.updated_at,u.username AS created_by").
// 		Joins("LEFT JOIN `user` AS u ON u.id = created_by").
// 		Where("groupid = ?", gid).
// 		Order("ga.id DESC").
// 		First(&announce).Error
// 	return announce, errorsx.HandleError(err)
// }

func (s *SQLGroupRepository) DeleteAnnounce(gid, uid, announceID uint) error {
	sql := `DELETE ga FROM group_announcement AS ga
			JOIN group_person AS gp ON gp.group_id = ga.group_id AND gp.member_id = ? AND gp.role IN ?
			WHERE ga.id = ? AND ga.group_id = ?`
	result := s.db.Exec(sql,
		uid, []int{m.GroupRoleOwner, m.GroupRoleAdmin},
		announceID, gid)
	if err := errorsx.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}

func (s *SQLGroupRepository) List(uid uint) ([]*m.SummaryGroupInfo, error) {
	var groups []*m.SummaryGroupInfo
	err := s.db.Table("`group_person` AS gp").
		Select("g.gid,g.`name` AS `groupname`").
		Where("member_id = ?", uid).
		Joins("LEFT JOIN `group` AS g ON g.gid = gp.group_id").
		Find(&groups).Error

	return groups, errorsx.HandleError(err)
}

func (s *SQLGroupRepository) CreateMember(ctx *m.MemberStatusContext) error {
	err := s.db.Create(&m.GroupPerson{
		MemberID:  ctx.To,
		GroupID:   ctx.GID,
		Role:      ctx.NewStatus,
		InviterID: ctx.From,
	}).Error
	return errorsx.HandleError(err)
}

func (s *SQLGroupRepository) DeleteMember(ctx *m.MemberStatusContext) error {
	from, to := ctx.Data[ctx.From], ctx.Data[ctx.To]
	sql := `DELETE gp1 FROM group_person AS gp1
			JOIN group_person AS gp2 ON gp2.id = ? AND gp2.version = ?
			WHERE gp1.id = ? AND gp1.version = ?`
	err := s.db.Exec(sql,
		from.ID, from.Version,
		to.ID, to.Version).Error
	return errorsx.HandleError(err)
}

func (s *SQLGroupRepository) UpdateStatus(ctx *m.MemberStatusContext) error {
	now := time.Now().Unix()
	from, to := ctx.Data[ctx.From], ctx.Data[ctx.To]
	sql := `UPDATE group_person AS gp1
			JOIN group_person AS gp2 ON gp2.id = ? AND gp2.version = ?
			SET gp1.role = ?,gp1.inviter_id = ?,gp1.created_at = ?,gp1.version = ?		
			WHERE gp1.id = ? AND gp1.version = ?`
	result := s.db.Exec(sql,
		from.ID, from.Version,
		ctx.NewStatus, ctx.From, now, to.Version+1,
		to.ID, to.Version)

	if err := errorsx.HandleError(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return errorsx.ErrNoAffectedRows
	}
	return nil
}
