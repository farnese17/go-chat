package config

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
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
	defaultCfg := &config_{
		Common_: &Common_{
			HttpPort_:          "8080",
			ManagerPort_:       "9000",
			Path_:              path,
			LogDir_:            "./chat/log/",
			RetryDelay_:        400 * time.Millisecond,
			JitterCoeff_:       0.5,
			MaxRetries_:        3,
			InviteValidDays_:   7,
			TokenValidPeriod_:  48 * time.Hour,
			CheckAckTimeout_:   time.Second * 2,
			MessageAckTiemout_: time.Second * 3,
			ResendBatchSize_:   100,
		},
		Database_: &Database_{},
		Cache_: &Cache_{
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

	config = defaultCfg
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
				return defaultCfg
			}

			data, _ := defaultCfg.encodeYamlWithComment()
			if err := os.WriteFile(path, data, 0644); err != nil {
				fmt.Println("initialization config file failed: ", err)
			}
		} else {
			fmt.Println("load config file failed: ", err)
		}
		return defaultCfg
	}

	loadedCfg := new(config_)
	*loadedCfg = *defaultCfg
	if err := yaml.Unmarshal(data, &loadedCfg); err != nil {
		fmt.Println("parse the config file failed: ", err)
	}
	loadedCfg.Common_.Path_ = path
	// 合并
	loadedCfg = mergeConfig(defaultCfg, loadedCfg)
	config = loadedCfg
	loadedCfg.Save()
	return loadedCfg
}

// 将可能存在的空值填充为默认值
func mergeConfig(src, dest *config_) *config_ {
	exclude := []string{"host", "port", "user", "db_name", "db_num", "addr", "password", "path", "log_dir", "log_path"}
	var merge func(reflect.Value, reflect.Value)
	merge = func(v1, v2 reflect.Value) {
		if v1.Kind() == reflect.Ptr {
			if v1.IsNil() {
				return
			}
			v1 = v1.Elem()
			// v2来源于解引v1，再反序列化覆盖，保证结构完整
			v2 = v2.Elem()
		}
		if v1.Kind() != reflect.Struct {
			return
		}
		t1 := v1.Type()
		for i := range v1.NumField() {
			tag := t1.Field(i).Tag.Get("yaml")
			if slices.Contains(exclude, tag) {
				continue
			}
			f1 := v1.Field(i)
			f2 := v2.Field(i)
			if f2.IsZero() && f2.CanSet() {
				f2.Set(f1)
			}
			merge(f1, f2)
		}
	}

	v1, v2 := reflect.ValueOf(src), reflect.ValueOf(dest)
	merge(v1, v2)
	return dest
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
	c, _ := yaml.Marshal(cfg)
	var res map[string]any
	yaml.Unmarshal(c, &res)
	var handler func(map[string]any)
	handler = func(m map[string]any) {
		if m == nil {
			return
		}
		for k, v := range m {
			if val, ok := v.(map[string]any); ok {
				handler(val)
			} else if k == "password" {
				delete(m, k)
			}
		}
	}
	handler(res)
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
	data, err := cfg.encodeYamlWithComment()
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.Common_.Path_, data, 0644)
}

func (cfg *config_) SetCommon(k, v string) error {
	switch k {
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
	case "check_ack_timeout":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		if t <= 0 {
			return errors.New("检查间隔太短")
		}
		cfg.Common_.CheckAckTimeout_ = t
	case "message_ack_timeout":
		t, err := cfg.convertToTime(v)
		if err != nil {
			return err
		}
		if t <= 0 {
			return errors.New("重发间隔太短")
		}
		cfg.Common_.MessageAckTiemout_ = t
	case "resend_batch_size":
		val, _ := strconv.ParseInt(v, 10, 64)
		if val < 1 {
			return errors.New("resend_batch_size的值必须大于0")
		}
		cfg.Common_.ResendBatchSize_ = val
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
	HttpAddress_       string        `yaml:"http_address" json:"http_address" comment:"服务器地址"`
	Manager_Address_   string        `yaml:"manager_address" json:"manager_address" comment:"管理服务器地址"`
	HttpPort_          string        `yaml:"http_port" json:"http_port" comment:"端口"`
	ManagerPort_       string        `yaml:"manager_port" json:"manager_port" comment:"管理端口"`
	Path_              string        `yaml:"path" json:"path" comment:"配置文件路径"`
	LogDir_            string        `yaml:"log_dir" json:"log_dir" comment:"日志目录"`
	RetryDelay_        time.Duration `yaml:"retry_delay" json:"retry_delay" comment:"重试退避基数"`
	JitterCoeff_       float64       `yaml:"jitter_coeff" json:"jitter_coeff" comment:"重试退避抖动系数"`
	MaxRetries_        int           `yaml:"max_retries" json:"max_retries" comment:"最大重试次数"`
	InviteValidDays_   int           `yaml:"invite_valid_days" json:"invite_valid_days" comment:"群组邀请有效期"`
	TokenValidPeriod_  time.Duration `yaml:"token_valid_period" json:"token_valid_period" comment:"token有效期"`
	CheckAckTimeout_   time.Duration `yaml:"check_ack_timeout" json:"check_ack_timeout" comment:"检查未确认消息的间隔"`
	MessageAckTiemout_ time.Duration `yaml:"message_ack_timeout" json:"message_ack_timeout" comment:"等待确认的消息的超时时间"`
	ResendBatchSize_   int64         `yaml:"resend_batch_size" json:"resend_batch_size" comment:"获取未确认消息用于重发的批大小"`
}

type Common interface {
	HttpPort() string
	HttpAddress() string
	ManagerPort() string
	ManagerAddress() string
	LogDir() string
	RetryDelay(n int) time.Duration
	MaxRetries() int
	InviteValidDays() int
	TokenValidPeriod() time.Duration
	CheckAckTimeout() time.Duration
	MessageAckTiemout() time.Duration
	ResendBatchSize() int64
}

func (c *Common_) HttpPort() string {
	return c.HttpPort_
}

func (c *Common_) HttpAddress() string {
	return c.HttpAddress_
}

func (c *Common_) ManagerPort() string {
	return c.ManagerPort_
}

func (c *Common_) ManagerAddress() string {
	return c.Manager_Address_
}

func (c *Common_) LogDir() string {
	return c.LogDir_
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

func (c *Common_) CheckAckTimeout() time.Duration {
	return c.CheckAckTimeout_
}

func (c *Common_) MessageAckTiemout() time.Duration {
	return c.MessageAckTiemout_
}

func (c *Common_) ResendBatchSize() int64 {
	return c.ResendBatchSize_
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
	Addr_               string        `yaml:"addr" json:"addr" comment:"redis地址: 127.0.0.1:6379"`
	Password_           string        `yaml:"password" json:"-" comment:"redis密码"`
	DbNum_              int           `yaml:"db_num" json:"db_num" comment:"redis仓库"`
	MaxGroups_          int           `yaml:"max_groups" json:"max_groups" comment:"最大缓存群组数量"`
	AutoFlushInterval_  time.Duration `yaml:"auto_flush_interval" json:"auto_flush_interval" comment:"redis管道刷新间隔"`
	AutoFlushThreshold_ int32         `yaml:"auto_flush_threshold" json:"auto_flush_threshold" comment:"redis管道自动刷新阈值"`
	RetryDelay_         time.Duration `yaml:"retry_delay" json:"retry_delay" comment:"redis重试退避基数"`
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
	Addr_    string `yaml:"addr" json:"addr" comment:"文件储存系统地址"`
	Path_    string `yaml:"path" json:"path" comment:"文件储存目录"`
	LogPath_ string `yaml:"log_path" comment:"文件储存系统日志"`
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

func (cfg *config_) encodeYamlWithComment() ([]byte, error) {
	// 先使用标准库序列化
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	// 分行处理
	lines := strings.Split(string(data), "\n")
	idx := 0
	res := []string{}

	var addComment func(reflect.Value, string)
	addComment = func(v reflect.Value, indent string) {
		if v.Kind() != reflect.Struct {
			return
		}
		t := v.Type()
		for i := range v.NumField() {
			field := v.Field(i)
			if !field.IsValid() {
				return
			}
			if cm := t.Field(i).Tag.Get("comment"); cm != "" {
				res = append(res, indent+"# "+cm)
			}
			res = append(res, lines[idx])
			idx++
			if field.Kind() == reflect.Ptr {
				field = field.Elem()
			}
			addComment(field, "  "+indent)
		}
	}
	v := reflect.ValueOf(*cfg)
	addComment(v, "")
	return []byte(strings.Join(res, "\n")), nil
}
