package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"

	v1 "github.com/farnese17/chat/api/v1"
	"github.com/farnese17/chat/middleware"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/router"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	s                 registry.Service
	route             *gin.Engine
	managerRouter     *gin.Engine
	testData          []*m.User
	testDataCount     = 50
	wg                = &sync.WaitGroup{}
	setupUserDataOnce = &sync.Once{}
)

func TestMain(m *testing.M) {
	// addr := "root:123456@tcp(127.0.0.1:33060)/chat_test?charset=utf8mb4&parseTime=True&loc=Local"

	s = registry.SetupService("")
	defer s.Shutdown()
	v1.SetupUserService(s)
	v1.SetupGroupService(s)
	v1.SetupFriendService(s)
	v1.SetupManagerService(s)
	go s.Cache().StartFlush()
	route = router.SetupRouter("release")
	managerRouter = router.SetupManagerRouter("release")

	clear()

	code := m.Run()
	if code != 0 {
		panic(code)
	}
}

func setupTestData() {
	setupUserDataOnce.Do(func() {
		testData = genTestData()
		for _, user := range testData {
			if err := s.User().CreateUser(user); err != nil {
				panic(fmt.Errorf("setup test data error %v", err))
			}
		}
	})
}

func clear() {
	repo := s.User().(repository.TestableRepo)
	repo.ExecSql("DELETE FROM `group`")
	repo.ExecSql("DELETE FROM `user`")
	repo.ExecSql("ALTER TABLE `user` AUTO_INCREMENT = 100001")
	repo.ExecSql("ALTER TABLE `group` AUTO_INCREMENT = 1000000001")
	repo.ExecSql("ALTER TABLE group_person AUTO_INCREMENT = 1")
	repo.ExecSql("DELETE FROM `manager`")
	repo.ExecSql("ALTER TABLE `manager` AUTO_INCREMENT = 1001")
}

func genTestData() []*m.User {
	data := make([]*m.User, testDataCount)
	passwd, _ := utils.HashPassword("aaaaaa")
	for i := range testDataCount {
		suffix := i + len(testData)
		name := "test" + strconv.FormatInt(int64(suffix), 10)
		if len(name) > 8 {
			name = name[len(name)-8:]
		}
		phone := strconv.FormatInt(int64(13*int(1e9)+suffix), 10)
		email := name + "@mail.com"
		data[i] = &m.User{Username: name,
			Password: passwd,
			Phone:    phone,
			Email:    email,
			Avatar:   "Thie is an avatar"}
	}
	return data
}

func TestRegister(t *testing.T) {
	setupTestData()
	tests := genTestData()

	url := "/api/v1/users"
	wg := &sync.WaitGroup{}
	for i, tt := range tests {
		tt.Password = "aaaaaa"
		wg.Add(1)
		go func(tt *m.User) {
			defer wg.Done()
			t.Run(fmt.Sprintf("register %d", i), func(t *testing.T) {
				body, _ := json.Marshal(tt)
				testNoError(t, route, url, "POST", 0, bytes.NewBuffer(body))
			})
		}(tt)
	}
	wg.Wait()
}

func TestLogin(t *testing.T) {
	setupTestData()

	url := "/api/v1/login"
	wg := &sync.WaitGroup{}
	subTest := func(idx int, account string) {
		defer wg.Done()
		t.Run(fmt.Sprintf("login %s", account), func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"account": account, "password": "aaaaaa"})
			resp := testNoError(t, route, url, "POST", 0, bytes.NewBuffer(body))
			expected := testData[idx]
			equalStruct(t, expected, resp["data"].(map[string]any))
			assert.NotEmpty(t, resp["token"].(string))
		})
	}

	for i, tt := range testData {
		wg.Add(3)
		go subTest(i, strconv.FormatUint(uint64(tt.ID), 10))
		go subTest(i, tt.Phone)
		go subTest(i, tt.Email)
	}
	wg.Wait()
}

func TestSearchUser(t *testing.T) {
	setupTestData()

	url := "/api/v1/users/search"
	wg := &sync.WaitGroup{}
	subTest := func(idx int, account string) {
		defer wg.Done()
		t.Run(fmt.Sprintf("search user %s", account), func(t *testing.T) {
			resp := testNoError(t, route, url, "GET", 10001, nil, map[string]string{
				"account": account})
			expected := testData[idx]
			equalStruct(t, expected, resp["data"].(map[string]any))
		})
	}

	for range testDataCount {
		wg.Add(2)
		idx := rand.IntN(testDataCount)
		tt := testData[idx]
		go subTest(idx, tt.Phone)
		go subTest(idx, tt.Email)
	}
	wg.Wait()
}

func TestGetUser(t *testing.T) {
	setupTestData()

	url := "/api/v1/users"
	wg := &sync.WaitGroup{}
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount)
			tt := testData[idx]
			t.Run(fmt.Sprintf("get user %d", tt.ID), func(t *testing.T) {
				resp := testNoError(t, route, url, "GET", tt.ID, nil)
				expected := testData[idx]
				equalStruct(t, expected, resp["data"].(map[string]any))
			})
		}()
	}
	wg.Wait()
}

func TestUpdateUser(t *testing.T) {
	setupTestData()

	url := "/api/v1/users"
	wg := &sync.WaitGroup{}
	subTest := func(tt *m.User, value, field string) {
		defer wg.Done()
		t.Run(fmt.Sprintf("update user %s", value), func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"value": value, "field": field})
			resp := testNoError(t, route, url, "PUT", tt.ID, bytes.NewBuffer(body))
			assert.Nil(t, resp["data"])
		})
	}
	for i := range testDataCount {
		wg.Add(4)
		idx := rand.IntN(testDataCount)
		tt := testData[idx]
		go subTest(tt, fmt.Sprintf("update avatar%d", i), "avatar")
		go subTest(tt, fmt.Sprintf("user%d", i), "username")
		go subTest(tt, strconv.FormatInt(int64(i), 10)+tt.Email, "email")
		phone := strconv.FormatInt(int64(15*int(1e9)+i), 10)
		go subTest(tt, phone, "phone")
	}
	wg.Wait()
}

func TestUpdatepassword(t *testing.T) {
	setupTestData()

	url := "/api/v1/users/password"
	wg := &sync.WaitGroup{}
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount)
			tt := testData[idx]
			t.Run(fmt.Sprintf("update password %d", tt.ID), func(t *testing.T) {
				body, _ := json.Marshal(map[string]string{"old": "aaaaaa", "new": "bbbbbb", "confirm": "bbbbbb"})
				resp := testNoError(t, route, url, "PUT", tt.ID, bytes.NewBuffer(body))
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestDeleteUser(t *testing.T) {
	setupTestData()

	url := "/api/v1/users"
	wg := &sync.WaitGroup{}
	for _, tt := range testData {
		wg.Add(1)
		go func(tt *m.User) {
			defer wg.Done()
			t.Run(fmt.Sprintf("delete user %d", tt.ID), func(t *testing.T) {
				resp := testNoError(t, route, url, "DELETE", tt.ID, nil)
				assert.Nil(t, resp["data"])
			})
		}(tt)
	}
	wg.Wait()
}

func equalHttpResp(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(200), resp["status"].(float64))
	assert.Equal(t, "OK", resp["message"].(string))
	return resp
}

func addToken(uid uint, req *http.Request) string {
	token, err := s.Cache().GetToken(uid)
	if err != nil || token == "" {
		token, _ = middleware.GenerateToken(uid)
		s.Cache().SetToken(uid, token, s.Config().Common().TokenValidPeriod())
		s.Cache().Flush()
	}
	req.Header.Add("Authorization", "Bearer "+token)
	return token
}

func equalStruct(t *testing.T, expected any, resp map[string]any, noEqual ...string) {
	jsonData, _ := json.Marshal(&expected)
	var m map[string]any
	json.Unmarshal(jsonData, &m)

	for k, v := range resp {
		if slices.Contains(noEqual, k) {
			continue
		}
		assert.Equalf(t, m[k], v, fmt.Sprintf("%s: %v not equal to %v", k, m[k], v))
	}
}

func equalError(t *testing.T, expected error, w *httptest.ResponseRecorder) {
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, errorsx.GetStatusCode(expected), int(resp["status"].(float64)))
	assert.Equal(t, expected.Error(), resp["message"].(string))
	assert.Nil(t, resp["data"])
}

func testHasError(t *testing.T, router *gin.Engine, url, method string, handler uint, body io.Reader, expected error, query ...map[string]string) {
	w := sendRequest(router, url, method, handler, body, query...)
	equalError(t, expected, w)
}

func testNoError(t *testing.T, router *gin.Engine, url, method string, handler uint, body io.Reader, query ...map[string]string) map[string]any {
	w := sendRequest(router, url, method, handler, body, query...)
	resp := equalHttpResp(t, w)
	return resp
}

func sendRequest(router *gin.Engine, url, method string, handler uint, body io.Reader, query ...map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, body)
	if handler != 0 {
		addToken(handler, req)
	}
	if len(query) > 0 {
		q := req.URL.Query()
		for k, v := range query[0] {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
