package config

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/farnese17/chat/utils/errorsx"
	"gopkg.in/yaml.v3"
)

var (
	config Config
)

type Config interface {
	Get() map[string]any
	Common() Common
	Cache() Cache
	Database() Database
	FileServer() FileServer
	Save() error
	SetCommon(k, v string) error
	SetCache(k, v string) error
}

func LoadConfig(path string) Config {
	// default
	cfg := &config_{
		Common_: &Common_{
			Path_:             path,
			LogDir_:           "./chat/log/",
			ResendInterval_:   3 * time.Second,
			RetryDelay_:       400 * time.Millisecond,
			JitterCoeff_:      0.5,
			MaxRetries_:       3,
			InviteValidDays_:  7,
			TokenValidPeriod_: 48 * time.Hour,
		},
		Database_: &Database_{
			Host_:     "127.0.0.1",
			Port_:     "3306",
			User_:     "root",
			Password_: "123456",
			DbName_:   "chat",
		},
		Cache_: &Cache_{
			Addr_:               "127.0.0.1:6379",
			DbNum_:              1,
			MaxGroups_:          100000,
			AutoFlushInterval_:  50 * time.Millisecond,
			AutoFlushThreshold_: 500,
			RetryDelay_:         time.Millisecond * 10,
		},
		FileServer_: &FileServer_{
			Addr_:    "http://localhost:3000/",
			Path_:    "./chat/storage/files/",
			LogPath_: "./chat/storage/storage.log",
		},
	}

	config = cfg
	// load config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsPermission(err) {
			fmt.Println(err)
			return nil
		}
		if os.IsNotExist(err) {
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Println("create folder failed: ", err)
				return cfg
			}
			data, _ := yaml.Marshal(cfg)
			if err := os.WriteFile(path, data, 0644); err != nil {
				fmt.Println("initialization config file failed: ", err)
			}
		} else {
			fmt.Println("load config file failed: ", err)
		}
		return cfg
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Println("parse the config file failed: ", err)
	}
	cfg.Common_.Path_ = path
	config = cfg
	return cfg
}

func GetConfig() Config {
	return config
}

type config_ struct {
	*Common_     `yaml:"common" json:"common"`
	*Cache_      `yaml:"cache" json:"cache"`
	*Database_   `yaml:"database" json:"database"`
	*FileServer_ `yaml:"file_server" json:"file_server"`
}

func (cfg *config_) Get() map[string]any {
	res := make(map[string]any)
	c := *cfg
	v := reflect.ValueOf(c)
	t := v.Type()
	for i := range v.NumField() {
		field := v.Field(i)
		if !field.CanInterface() || !field.IsValid() {
			continue
		}
		if field.Kind() != reflect.Ptr {
			continue
		}
		vv := field.Elem()
		if !vv.IsValid() {
			continue
		}
		tt := vv.Type()
		m := make(map[string]any)
		for j := range vv.NumField() {
			field := tt.Field(j).Tag.Get("yaml")
			if field == "-" || field == "password" {
				continue
			}

			fieldValue := vv.Field(j)
			if !fieldValue.IsValid() || !fieldValue.CanInterface() {
				continue
			}

			if val, ok := fieldValue.Interface().(time.Duration); ok {
				m[field] = val.String()
			} else {
				m[field] = fieldValue.Interface()
			}
		}
		res[t.Field(i).Tag.Get("yaml")] = m
	}
	return res
}

func (cfg *config_) Common() Common {
	return cfg.Common_
}

func (cfg *config_) Cache() Cache {
	return cfg.Cache_
}

func (cfg *config_) Database() Database {
	return cfg.Database_
}

func (cfg *config_) FileServer() FileServer {
	return cfg.FileServer_
}

func (cfg *config_) Save() error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.Common_.Path_, data, 0644)
}

func (cfg *config_) SetCommon(k, v string) error {
	switch k {
	case "resend_interval":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		cfg.Common_.ResendInterval_ = t
	case "retry_delay":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		cfg.Common_.RetryDelay_ = t
	case "jitter_coeff":
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return errors.New("无效的jitter_coeff值,必须是浮点数")
		}
		if n < 0.01 {
			return errors.New("jitter_corff不应该小于0.01")
		}
		if n > 1 {
			return errors.New("jitter_corff不应该大于1")
		}
		cfg.Common_.JitterCoeff_ = n
	case "max_retries":
		n, err := strconv.Atoi(v)
		if err != nil {
			return errors.New("max_retries的值应该在1-5之间")
		}
		cfg.Common_.MaxRetries_ = n
	case "invite_valid_days":
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		if n < 1 {
			return errors.New("invite_valid_days的值必须不小于1")
		}
		cfg.Common_.InviteValidDays_ = n
	case "token_valid_period":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		if t < 15*time.Minute {
			return errors.New("token有效期太短")
		}
		cfg.Common_.TokenValidPeriod_ = t
	default:
		return errorsx.ErrNoSettingOption
	}
	return nil
}

func (cfg *config_) SetCache(k, v string) error {
	switch k {
	case "max_groups":
		val, _ := strconv.Atoi(v)
		if val < 1 {
			return errors.New("max_groups的值必须大于0")
		}
		cfg.Cache_.MaxGroups_ = val
	case "auto_flush_interval":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		cfg.Cache_.AutoFlushInterval_ = t
	case "auto_flush_threshold":
		val, _ := strconv.Atoi(v)
		if val < 1 {
			return errors.New("auto_flush_threshold的值必须大于1")
		}
		cfg.Cache_.AutoFlushThreshold_ = int32(val)
	case "retry_delay":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		cfg.Cache_.RetryDelay_ = t
	default:
		return errorsx.ErrNoSettingOption
	}
	return nil
}

type Common_ struct {
	Path_             string        `yaml:"config_path" json:"config_path"`
	LogDir_           string        `yaml:"log_dir" json:"log_dir"`
	ResendInterval_   time.Duration `yaml:"resend_interval" json:"resend_interval"`
	RetryDelay_       time.Duration `yaml:"retry_delay" json:"retry_delay"`
	JitterCoeff_      float64       `yaml:"jitter_coeff" json:"jitter_coeff"`
	MaxRetries_       int           `yaml:"max_retries" json:"max_retries"`
	InviteValidDays_  int           `yaml:"invite_valid_days" json:"invite_valid_days"`
	TokenValidPeriod_ time.Duration `yaml:"token_valid_period" json:"token_valid_period"`
}

type Common interface {
	LogDir() string
	ResendInterval() time.Duration
	RetryDelay(n int) time.Duration
	MaxRetries() int
	InviteValidDays() int
	TokenValidPeriod() time.Duration
}

func (c *Common_) LogDir() string {
	return c.LogDir_
}

func (c *Common_) ResendInterval() time.Duration {
	return c.ResendInterval_
}

func (c *Common_) RetryDelay(n int) time.Duration {
	return c.RetryDelay_ * (1 << n) * time.Duration(c.JitterCoeff_+rand.Float64())
}

func (c *Common_) MaxRetries() int {
	return c.MaxRetries_
}

func (c *Common_) InviteValidDays() int {
	return c.InviteValidDays_
}

func (c *Common_) TokenValidPeriod() time.Duration {
	return c.TokenValidPeriod_
}

type Database_ struct {
	Host_     string `yaml:"host" json:"host"`
	Port_     string `yaml:"port" json:"port"`
	User_     string `yaml:"user" json:"user"`
	Password_ string `yaml:"password" json:"-"`
	DbName_   string `yaml:"db_name" json:"db_name"`
}

type Database interface {
	Host() string
	Port() string
	User() string
	Password() string
	DBname() string
}

func (data *Database_) Host() string {
	return data.Host_
}

func (data *Database_) Port() string {
	return data.Port_
}

func (data *Database_) User() string {
	return data.User_
}

func (data *Database_) Password() string {
	return data.Password_
}

func (data *Database_) DBname() string {
	return data.DbName_
}

type Cache_ struct {
	Addr_               string        `yaml:"addr" json:"addr"`
	DbNum_              int           `yaml:"db_num" json:"db_num"`
	MaxGroups_          int           `yaml:"max_groups" json:"max_groups"`
	AutoFlushInterval_  time.Duration `yaml:"auto_flush_interval" json:"auto_flush_interval"`
	AutoFlushThreshold_ int32         `yaml:"auto_flush_threshold" json:"auto_flush_threshold"`
	RetryDelay_         time.Duration `yaml:"retry_delay" json:"retry_delay"`
}

type Cache interface {
	Addr() string
	DBNum() int
	MaxGroups() int
	AutoFlushInterval() time.Duration
	AutoFlushThreshold() int32
	RetryDelay(n int) time.Duration
}

func (cfg *Cache_) Addr() string {
	return cfg.Addr_
}

func (cfg *Cache_) DBNum() int {
	return cfg.DbNum_
}

func (cfg *Cache_) MaxGroups() int {
	return cfg.MaxGroups_
}

func (cfg *Cache_) AutoFlushInterval() time.Duration {
	return cfg.AutoFlushInterval_
}

func (cfg *Cache_) AutoFlushThreshold() int32 {
	return cfg.AutoFlushThreshold_
}

func (cfg *Cache_) RetryDelay(n int) time.Duration {
	return cfg.RetryDelay_ * (1 << n)
}

type FileServer_ struct {
	Addr_    string `yaml:"addr" json:"addr"`
	Path_    string `yaml:"path" json:"path"`
	LogPath_ string `yaml:"log_path"`
}

type FileServer interface {
	Addr() string
	Path() string
	LogPath() string
}

func (fs *FileServer_) Addr() string {
	return fs.Addr_
}

func (fs *FileServer_) Path() string {
	return fs.Path_
}

func (fs *FileServer_) LogPath() string {
	return fs.LogPath_
}

func (cfg *config_) convertToTime(s string) (time.Duration, error) {
	t, err := time.ParseDuration(s)
	if err != nil {
		return -1, errors.New("必须是时间格式,如: 24h,50ms,0.1s")
	}
	return t, nil
}
