package storage

import (
	"fmt"
)

type Option interface {
	GetDSN() string
}

type MysqlOption struct {
	User     string
	Password string
	Addr     string
	Port     string
	DBName   string
}

func (m *MysqlOption) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		m.User, m.Password, m.Addr, m.Port, m.DBName)
}

type SqliteOption struct {
	Path string
	// file or memory
	Mode string
}

func (s *SqliteOption) GetDSN() string {
	if s == nil {
		return "./storage.sqlite"
	}
	if s.Mode == "memory" {
		return "file::memory:?cache=shared"
	}
	return s.Path
}

type RedisOption struct {
	Addr     string
	Password string
	DB       int
}

func (r *RedisOption) GetDSN() string {
	return ""
}
