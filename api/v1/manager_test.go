package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/websocket"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

var (
	superAdmin       uint = 1001
	managers         []*model.Manager
	setupAdminsOnce  = sync.Once{}
	genAdminDataOnce = sync.Once{}
	createAdminOnce  = sync.Once{}
	countUsers       int
	adminIDs         = make(map[string]uint)
)

func getConfigPath() string {
	return os.Getenv("CHAT_CONFIG")
}

func setupAdmins(t *testing.T) {
	setupAdminsOnce.Do(func() {
		mgrs := []*model.Manager{
			{Permissions: model.MgrSuperAdministrator, Username: "super_admin", Email: "super_admin@mail.com"},
			{Permissions: model.MgrWriteAndRead, Username: "write_admin", Email: "write_admin@mail.com"},
			{Permissions: model.MgrOnlyRead, Username: "read_admin", Email: "read_admin@mail.com"},
		}
		for _, mgr := range mgrs {
			generateAdmin(t, mgr)
			var key string
			if mgr.Permissions == model.MgrSuperAdministrator {
				key = "super"
			} else if mgr.Permissions == model.MgrWriteAndRead {
				key = "write"
			} else {
				key = "read"
			}
			adminIDs[key] = mgr.ID
		}
	})
}

func generateAdmin(t *testing.T, mgr *model.Manager) {
	passwd, _ := utils.HashPassword("aaaaaa")
	mgr.Password = passwd
	if err := s.Manager().Create(mgr); err != nil {
		t.Errorf("generate test admin failed: %v", err)
	}
}

func genAdminData(count int) {
	genAdminDataOnce.Do(func() {
		for i := range count {
			name := fmt.Sprintf("admin_%d", i)
			email := fmt.Sprintf("%s@mail.com", name)
			mgr := &model.Manager{Permissions: model.MgrOnlyRead, Username: name, Password: "aaaaaa", Email: email}
			managers = append(managers, mgr)
		}
	})
}

func generateTestUsers(t *testing.T, count int) []*model.User {
	testUsers := make([]*model.User, count)
	for i := range testUsers {
		name := fmt.Sprintf("test_%d", i+countUsers)
		email := name + "@mail.com"
		phone := fmt.Sprintf("%d", 13013013013+i+countUsers)
		user := &model.User{Username: name, Password: "123456", Phone: phone, Email: email}
		if err := s.User().CreateUser(user); err != nil {
			t.Errorf("create test user failed: %v", err)
		}
		testUsers[i] = user
	}
	countUsers += count
	return testUsers
}

func TestAdminCreate(t *testing.T) {
	createAdminOnce.Do(func() {
		setupAdmins(t)
		genAdminData(20)
		wg := sync.WaitGroup{}
		url := "/api/v1/managers/admins"
		for _, m := range managers {
			wg.Add(1)
			go func(mgr *model.Manager) {
				defer wg.Done()
				t.Run(fmt.Sprintf("create admin %s", mgr.Username), func(t *testing.T) {
					body, _ := json.Marshal(mgr)
					resp := testNoError(t, managerRouter, url, "POST", superAdmin, bytes.NewBuffer(body))
					assert.NotNil(t, resp["data"])
				})
			}(m)
		}
		wg.Wait()
	})
}

func TestAdminLogin(t *testing.T) {
	TestAdminCreate(t)

	wg := sync.WaitGroup{}
	url := "/api/v1/managers/login"
	for i := range managers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			t.Run(fmt.Sprintf("admin login %d", int(superAdmin)+i+1), func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"id": int(superAdmin) + i + 1, "password": "aaaaaa"})
				testNoError(t, managerRouter, url, "POST", 0, bytes.NewBuffer(body))
			})
		}(i)
	}
	wg.Wait()
}

func TestGetConfig(t *testing.T) {
	setupAdmins(t)

	cfg, err := os.ReadFile(getConfigPath())
	assert.NoError(t, err)
	assert.NotEmpty(t, cfg)
	var cfgMap map[string]any
	err = yaml.Unmarshal(cfg, &cfgMap)
	assert.NoError(t, err)
	for _, v := range cfgMap {
		for kk := range v.(map[string]any) {
			if kk == "password" {
				delete(v.(map[string]any), kk)
			}
		}
	}
	expected, err := json.Marshal(cfgMap)
	assert.NoError(t, err)

	url := "/api/v1/managers/config"
	resp := testNoError(t, managerRouter, url, "GET", adminIDs["read"], nil)
	got, err := json.Marshal(resp["data"])
	assert.NoError(t, err)
	assert.JSONEq(t, string(expected), string(got))
}

// func TestSetConfig(t *testing.T) {
// 	setupAdmins(t)

// 	url := "/api/v1/managers/config/set"
// 	t.Run("set cache max_groups", func(t *testing.T) {
// 		origin := s.Config().Cache().MaxGroups()
// 		setConfig(t, url, "cache", "max_groups", "1")
// 		assert.Equal(t, 1, s.Config().Cache().MaxGroups())

// 		setConfig(t, url, "cache", "max_groups", fmt.Sprintf("%d", origin))
// 		assert.Equal(t, origin, s.Config().Cache().MaxGroups())
// 	})

// 	t.Run("set common max_retries", func(t *testing.T) {
// 		origin := s.Config().Common().MaxRetries()
// 		setConfig(t, url, "common", "max_retries", "1")
// 		assert.Equal(t, 1, s.Config().Common().MaxRetries())

// 		setConfig(t, url, "common", "max_retries", fmt.Sprintf("%d", origin))
// 		assert.Equal(t, origin, s.Config().Common().MaxRetries())
// 	})
// }

func setConfig(t *testing.T, url, section, key, value string) {
	body, _ := json.Marshal(map[string]any{"section": section, "key": key, "value": value})
	resp := testNoError(t, managerRouter, url, "PUT", adminIDs["write"], bytes.NewBuffer(body))
	assert.Nil(t, resp["data"])
}

func TestSetAndSaveConfig(t *testing.T) {
	setupAdmins(t)

	url := "/api/v1/managers/config/set"
	maxGroups := s.Config().Cache().MaxGroups()
	MaxRetries := s.Config().Common().MaxRetries()

	save := func(t *testing.T, expected ...any) {
		t.Run("save config", func(t *testing.T) {
			url := "/api/v1/managers/config/save"
			resp := testNoError(t, managerRouter, url, "PUT", adminIDs["write"], nil)
			assert.Nil(t, resp["data"])

			cfg, err := os.ReadFile(getConfigPath())
			assert.NoError(t, err)
			var cfgMap map[string]any
			err = yaml.Unmarshal(cfg, &cfgMap)
			assert.NoError(t, err)
			assert.Equal(t, expected[0], cfgMap["cache"].(map[string]any)["max_groups"])
			assert.Equal(t, expected[1], cfgMap["common"].(map[string]any)["max_retries"])
		})
	}

	set := func(t *testing.T, name string, val ...string) {
		t.Run(name, func(t *testing.T) {
			setConfig(t, url, "cache", "max_groups", val[0])
			got, _ := strconv.Atoi(val[0])
			assert.Equal(t, got, s.Config().Cache().MaxGroups())

			got, _ = strconv.Atoi(val[1])
			setConfig(t, url, "common", "max_retries", val[1])
			assert.Equal(t, got, s.Config().Common().MaxRetries())

		})
	}

	set(t, "set config", "1", "1")
	save(t, 1, 1)
	set(t, "recover config", fmt.Sprintf("%d", maxGroups), fmt.Sprintf("%d", MaxRetries))
	save(t, maxGroups, MaxRetries)
}

func testBanUsers(t *testing.T, url string) {
	setupAdmins(t)
	testUsers := generateTestUsers(t, 10)
	wg := sync.WaitGroup{}
	for _, u := range testUsers {
		wg.Add(1)
		go func(uid uint) {
			defer wg.Done()
			ids := []uint{adminIDs["super"], adminIDs["write"]}
			handler := ids[rand.IntN(len(ids))]
			t.Run(fmt.Sprintf("%d ban %d %s", handler, uid, path.Base(url)), func(t *testing.T) {
				fullURL := fmt.Sprintf(url, uid)
				resp := testNoError(t, managerRouter, fullURL, "PUT", adminIDs["super"], nil)
				assert.Nil(t, resp["data"])
			})
		}(u.ID)
	}
	wg.Wait()

	t.Run("ban user,but has no permissions", func(t *testing.T) {
		fullURL := fmt.Sprintf(url, testUsers[0].ID)
		testHasError(t, managerRouter, fullURL, "PUT", adminIDs["read"], nil, errorsx.ErrPermissiondenied)
	})
}

var banOnce = sync.Once{}

func TestBanUserTemp(t *testing.T) {
	banOnce.Do(func() {
		url := "/api/v1/managers/users/%d/ban/temp"
		testBanUsers(t, url)
	})
}

func TestBanUserPerma(t *testing.T) {
	url := "/api/v1/managers/users/%d/ban/perma"
	testBanUsers(t, url)
}

func TestBanUserNopost(t *testing.T) {
	url := "/api/v1/managers/users/%d/ban/nopost"
	testBanUsers(t, url)
}

func TestBanUserMute(t *testing.T) {
	url := "/api/v1/managers/users/%d/ban/mute"
	testBanUsers(t, url)
}

func TestUnban(t *testing.T) {
	setupAdmins(t)
	tests := make([]uint, 20)
	for i := range tests {
		name := fmt.Sprintf("unban_%d", i+1)
		email := name + "@mail.com"
		phone := fmt.Sprintf("%d", 13013013013+i+countUsers)
		u := &model.User{Username: name, Password: "aaaaaa", Phone: phone, Email: email, BanLevel: model.BanLevelMuted, BanExpireAt: time.Now().Add(time.Hour).Unix()}
		err := s.User().CreateUser(u)
		assert.NoError(t, err)
		tests[i] = u.ID
	}
	url := "/api/v1/managers/users/%d/ban/unban"
	wg := sync.WaitGroup{}
	for _, tt := range tests {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			fullURL := fmt.Sprintf(url, id)
			arr := []string{"super", "write"}
			handler := adminIDs[arr[rand.IntN(len(arr))]]
			resp := testNoError(t, managerRouter, fullURL, "PUT", handler, nil)
			assert.Nil(t, resp["data"])
		}(tt)
	}
	wg.Wait()
	id := tests[0]
	fullURL := fmt.Sprintf(url, id)
	testHasError(t, managerRouter, fullURL, "PUT", adminIDs["read"], nil, errorsx.ErrPermissiondenied)
}

func TestGetBannedList(t *testing.T) {
	TestBanUserTemp(t)

	url := "/api/v1/managers/users/banned"
	body, _ := json.Marshal(&model.Cursor{PageSize: 9, HasMore: true})
	resp := testNoError(t, managerRouter, url, "GET", adminIDs["read"], bytes.NewBuffer(body))
	var data []*model.BanStatus
	jsonData, _ := json.Marshal(resp["data"].(map[string]any)["data"])
	err := json.Unmarshal(jsonData, &data)
	assert.NoError(t, err)
	assert.Equal(t, 9, len(data))
	for _, u := range data {
		assert.Equal(t, model.BanLevelTemporary, u.BanLevel)
		assert.NotEmpty(t, u.ID)
		assert.NotEmpty(t, u.BanExpireAt)
	}
	expectedCursor := &model.Cursor{LastID: uint(1e5 + 9), PageSize: 9, HasMore: true}
	equalStruct(t, expectedCursor, resp["data"].(map[string]any)["cursor"].(map[string]any))
}

func TestCountBanned(t *testing.T) {
	TestBanUserTemp(t)

	expected := countUsers
	url := "/api/v1/managers/users/banned/count"
	wg := sync.WaitGroup{}
	for _, id := range adminIDs {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			resp := testNoError(t, managerRouter, url, "GET", id, nil)
			assert.Equal(t, expected, int(resp["data"].(float64)))
		}(id)
	}
	wg.Wait()
}

func TestGetAdmins(t *testing.T) {
	setupAdmins(t)

	url := "/api/v1/managers/admins"
	expected := []*model.Manager{
		{ID: 1001, Permissions: model.MgrSuperAdministrator, Username: "super_admin", Email: "super_admin@mail.com"},
		{ID: 1002, Permissions: model.MgrWriteAndRead, Username: "write_admin", Email: "write_admin@mail.com"},
		// {ID: 1003, Permissions: model.MgrOnlyRead, Username: "read_admin", Email: "read_admin@mail.com"},
	}
	wg := sync.WaitGroup{}
	for _, id := range adminIDs {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			body, _ := json.Marshal(&model.Cursor{LastID: 0, PageSize: len(adminIDs) - 1, HasMore: true})
			resp := testNoError(t, managerRouter, url, "GET", id, bytes.NewBuffer(body))
			var mgrs []*model.Manager
			jsonData, _ := json.Marshal(resp["data"].(map[string]any)["data"])
			err := json.Unmarshal(jsonData, &mgrs)
			assert.NoError(t, err)

			for _, mgr := range mgrs {
				assert.NotEmpty(t, mgr.Created_At)
				assert.NotEmpty(t, mgr.Updated_At)
				mgr.Created_At = 0
				mgr.Updated_At = 0
			}
			assert.Equal(t, expected, mgrs)
			expectedCursor := &model.Cursor{LastID: adminIDs["write"], PageSize: len(adminIDs) - 1, HasMore: true}
			equalStruct(t, expectedCursor, resp["data"].(map[string]any)["cursor"].(map[string]any))
		}(id)
	}
	wg.Wait()
}

func TestGetAdmin(t *testing.T) {
	setupAdmins(t)

	url := "/api/v1/managers/admins/%d"
	wg := sync.WaitGroup{}
	for _, id := range adminIDs {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			fullURL := fmt.Sprintf(url, adminIDs["super"])
			resp := testNoError(t, managerRouter, fullURL, "GET", id, nil)
			expected := &model.Manager{
				ID:          1001,
				Permissions: model.MgrSuperAdministrator,
				Username:    "super_admin",
				Email:       "super_admin@mail.com",
			}
			equalStruct(t, expected, resp["data"].(map[string]any), "created_at", "updated_at")
			assert.NotEmpty(t, resp["data"].(map[string]any)["created_at"])
			assert.NotEmpty(t, resp["data"].(map[string]any)["updated_at"])
		}(id)
	}
	wg.Wait()
}

func TestUpdateAdminPasswd(t *testing.T) {
	setupAdmins(t)
	TestAdminCreate(t)

	wg := sync.WaitGroup{}
	url := "/api/v1/managers/admins/%d/update/password"
	for i := range len(managers) {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := adminIDs["super"] + uint(len(adminIDs)+i)
			t.Run(fmt.Sprintf("super_admin update passwd %d", id), func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"new": "123456", "confirm": "123456"})
				fullURL := fmt.Sprintf(url, id)
				resp := testNoError(t, managerRouter, fullURL, "PUT", adminIDs["super"], bytes.NewBuffer(body))
				assert.Nil(t, resp["data"])
			})
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := adminIDs["super"] + uint(len(adminIDs)+i)
			t.Run(fmt.Sprintf("update passwd %d", id), func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"new": "654321", "confirm": "654321"})
				fullURL := fmt.Sprintf(url, id)
				resp := testNoError(t, managerRouter, fullURL, "PUT", id, bytes.NewBuffer(body))
				assert.Nil(t, resp["data"])
			})
		}(i)
	}
	wg.Wait()
}

func TestWsStart(t *testing.T) {
	url := "/api/v1/managers/ws/start"
	setupAdmins(t)
	t.Run("start ws,but started", func(t *testing.T) {
		testHasError(t, managerRouter, url, "PUT", adminIDs["write"], nil, errorsx.ErrServerStarted)
	})

	if err := websocket.StopWebsocket(s); err != nil {
		t.Errorf("tried to start websocket,but got an error: %v", err)
	}

	t.Run("start ws,but has no permissions", func(t *testing.T) {
		testHasError(t, managerRouter, url, "PUT", adminIDs["read"], nil, errorsx.ErrPermissiondenied)
	})

	t.Run("start ws", func(t *testing.T) {
		resp := testNoError(t, managerRouter, url, "PUT", adminIDs["super"], nil)
		assert.Nil(t, resp["data"])
	})
}

func TestWsStop(t *testing.T) {
	url := "/api/v1/managers/ws/stop"
	setupAdmins(t)
	t.Run("stop ws,but no permissions", func(t *testing.T) {
		testHasError(t, managerRouter, url, "PUT", adminIDs["read"], nil, errorsx.ErrPermissiondenied)
	})

	t.Run("stop ws", func(t *testing.T) {
		resp := testNoError(t, managerRouter, url, "PUT", adminIDs["super"], nil, nil)
		assert.Nil(t, resp["data"])
	})

	t.Run("stop ws,but stoped", func(t *testing.T) {
		testHasError(t, managerRouter, url, "PUT", adminIDs["write"], nil, errorsx.ErrServerClosed)
	})
}

var deleteAdminOnce = sync.Once{}

func TestDeleteAdmin(t *testing.T) {
	deleteAdminOnce.Do(func() {
		url := "/api/v1/managers/admins/%d"
		operateAdmin(t, url, "DELETE")
	})
}

func TestRestoreAdmin(t *testing.T) {
	url := "/api/v1/managers/admins/%d/restore"
	TestDeleteAdmin(t)
	operateAdmin(t, url, "PUT")
}

func operateAdmin(t *testing.T, url, method string) {
	setupAdmins(t)
	TestAdminCreate(t)

	wg := sync.WaitGroup{}
	for i := range len(managers) {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := adminIDs["super"] + uint(len(adminIDs)+i)
			t.Run(fmt.Sprintf("%d", id), func(t *testing.T) {
				fullURL := fmt.Sprintf(url, id)
				resp := testNoError(t, managerRouter, fullURL, method, adminIDs["super"], nil)
				assert.Nil(t, resp["data"])
			})
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		id := adminIDs["read"]
		fullURL := fmt.Sprintf(url, id)
		t.Run(fmt.Sprintf("%d,but no permissions", id), func(t *testing.T) {
			testHasError(t, managerRouter, fullURL, method, adminIDs["write"], nil, errorsx.ErrPermissiondenied)
		})
		t.Run(fmt.Sprintf("%d,but no permissions", id), func(t *testing.T) {
			testHasError(t, managerRouter, fullURL, method, adminIDs["read"], nil, errorsx.ErrPermissiondenied)
		})
	}()
	wg.Wait()
}

func TestSetAdminPermissions(t *testing.T) {
	setupAdmins(t)
	TestAdminCreate(t)

	url := "/api/v1/managers/admins/%d/permissions"
	wg := sync.WaitGroup{}
	for i := range len(managers) {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := adminIDs["super"] + uint(len(adminIDs)+i)
			t.Run(fmt.Sprintf("set %d's permissions", id), func(t *testing.T) {
				fullURL := fmt.Sprintf(url, id)
				resp := testNoError(t, managerRouter, fullURL, "PUT", adminIDs["super"], nil, map[string]string{
					"permission": fmt.Sprintf("%d", model.MgrWriteAndRead)})
				assert.Nil(t, resp["data"])
			})
		}(i)
	}
	wg.Wait()
}
