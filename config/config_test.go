package config_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func getPath() string {
	return os.Getenv("CHAT_CONFIG")
}

func getConfigFromDisk(t *testing.T) map[string]any {
	data, err := os.ReadFile(getPath())
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	var cfgMap map[string]any
	err = yaml.Unmarshal(data, &cfgMap)
	assert.NoError(t, err)
	for _, v := range cfgMap {
		assert.NotEmpty(t, v)
		for kk := range v.(map[string]any) {
			if kk == "password" {
				delete(v.(map[string]any), kk)
			}
		}
	}
	return cfgMap
}

func loadConfig() config.Config {
	return config.LoadConfig(getPath())
}

func TestGet(t *testing.T) {
	expected := getConfigFromDisk(t)
	cfg := loadConfig()

	got := cfg.Get()
	assert.NotEmpty(t, got)
	assert.Equal(t, len(expected), len(got))

	expJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	gotJson, err := json.Marshal(got)
	assert.NoError(t, err)
	assert.JSONEq(t, string(expJson), string(gotJson))
}

func TestSetCommon(t *testing.T) {
	cfg := loadConfig()

	tests := []struct {
		name     string
		key      string
		val      string
		expected error
	}{
		{"set resend_interval", "", "1", errorsx.ErrNoSettingOption},
		{"set retry_delay", "retry_delay", "1", errors.New("必须是时间格式,如: 24h,50ms,0.1s")},
		{"set jitter_coeff", "jitter_coeff", "0", errors.New("jitter_corff不应该小于0.01")},
		{"set jitter_coeff", "jitter_coeff", "0.9", nil},
		{"set max_retries", "max_retries", "1", nil},
		{"set invite_valid_days", "invite_valid_days", "1", nil},
		{"set token_valid_period", "token_valid_period", "24h", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.SetCommon(tt.key, tt.val)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expected.Error())
			}
		})
	}

	var verity any
	verity = cfg.Common().MaxRetries()
	assert.Equal(t, 1, verity)
	verity = cfg.Common().InviteValidDays()
	assert.Equal(t, 1, verity)
	verity = cfg.Common().TokenValidPeriod()
	tokenValidPeriod, err := time.ParseDuration("24h")
	assert.NoError(t, err)
	assert.Equal(t, tokenValidPeriod, verity)
}

func TestSetCache(t *testing.T) {
	cfg := loadConfig()

	tests := []struct {
		name     string
		key      string
		val      string
		expected error
	}{
		{"set max_groups", "", "", errorsx.ErrNoSettingOption},
		{"set max_groups", "max_groups", "0", errors.New("max_groups的值必须大于0")},
		{"set max_groups", "max_groups", "1", nil},
		{"set auto_flush_interval", "auto_flush_interval", "1", errors.New("必须是时间格式,如: 24h,50ms,0.1s")},
		{"set auto_flush_interval", "auto_flush_interval", "1s", nil},
		{"set auto_flush_threshold", "auto_flush_threshold", "1", nil},
		{"set retry_delay", "retry_delay", "1s", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.SetCache(tt.key, tt.val)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expected.Error())
			}
		})
	}

	var verity any
	verity = cfg.Cache().MaxGroups()
	assert.Equal(t, 1, verity)
	verity = cfg.Cache().AutoFlushInterval()
	autoFlushInterval, err := time.ParseDuration("1s")
	assert.NoError(t, err)
	assert.Equal(t, autoFlushInterval, verity)
	verity = cfg.Cache().AutoFlushThreshold()
	assert.Equal(t, int32(1), verity)
}

func TestSave(t *testing.T) {
	cfg := loadConfig()

	origin := cfg.Cache().MaxGroups()
	err := cfg.SetCache("max_groups", "1")
	assert.NoError(t, err)
	err = cfg.Save()
	assert.NoError(t, err)

	expected := getConfigFromDisk(t)
	newVal := expected["cache"].(map[string]any)["max_groups"]
	assert.Equal(t, cfg.Cache().MaxGroups(), newVal)

	err = cfg.SetCache("max_groups", fmt.Sprintf("%d", origin))
	assert.NoError(t, err)
	err = cfg.Save()
	assert.NoError(t, err)
}
