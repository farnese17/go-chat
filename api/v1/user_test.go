package v1_test

import (
    "bytes"
    "encoding/json"
    "fmt"
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
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

var (
    s                 registry.Service
    route             *gin.Engine
    testData          []*m.User
    testDataCount     = 50
    wg                = &sync.WaitGroup{}
    setupUserDataOnce = &sync.Once{}
)

func TestMain(m *testing.M) {
    // addr := "root:123456@tcp(127.0.0.1:33060)/chat_test?charset=utf8mb4&parseTime=True&loc=Local"

    s = registry.SetupService()
    defer s.Shutdown()
    v1.SetupUserService(s)
    v1.SetupGroupService(s)
    v1.SetupFriendService(s)
    go s.Cache().StartFlush()
    route = router.SetupRouter("debug")

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
}

func genTestData() []*m.User {
    data := make([]*m.User, testDataCount)
    for i := range testDataCount {
        suffix := i + len(testData)
        name := "test" + strconv.FormatInt(int64(suffix), 10)
        if len(name) > 8 {
            name = name[len(name)-8:]
        }
        phone := strconv.FormatInt(int64(13*int(1e9)+suffix), 10)
        email := name + "@mail.com"
        password := "$2a$10$QaWiOTPgHqeb3mEQh2BjJemQU5BfNYpryENXE8vz5VcBPGZOMl3wO" // aaaaaa
        data[i] = &m.User{Username: name, Password: password, Phone: phone, Email: email, Avatar: "Thie is an avatar"}
    }
    return data
}

func TestRegister(t *testing.T) {
    setupTestData()
    tests := genTestData()

    wg := &sync.WaitGroup{}
    for i, tt := range tests {
        tt.Password = "aaaaaa"
        wg.Add(1)
        go func(tt *m.User) {
            defer wg.Done()
            t.Run(fmt.Sprintf("register %d", i), func(t *testing.T) {
                body, _ := json.Marshal(tt)
                req := httptest.NewRequest("POST", "/api/v1/user/create", bytes.NewBuffer(body))
                w := httptest.NewRecorder()
                route.ServeHTTP(w, req)

                resp := equalHttpResp(t, w)
                assert.Nil(t, resp["data"])
            })
        }(tt)
    }
    wg.Wait()
}

func TestLogin(t *testing.T) {
    setupTestData()
    wg := &sync.WaitGroup{}
    subTest := func(idx int, account string) {
        defer wg.Done()
        t.Run(fmt.Sprintf("login %s", account), func(t *testing.T) {
            body, _ := json.Marshal(map[string]any{"account": account, "password": "aaaaaa"})
            req := httptest.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
            w := httptest.NewRecorder()

            route.ServeHTTP(w, req)
            resp := equalHttpResp(t, w)

            expected := testData[idx]
            equalStruct(t, expected, resp["data"].(map[string]any))
            assert.NotEmpty(t, resp["token"].(string))
            // token.Store(tt.ID, resp["token"].(string))
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
    wg := &sync.WaitGroup{}
    subTest := func(idx int, account string) {
        defer wg.Done()
        t.Run(fmt.Sprintf("search user %s", account), func(t *testing.T) {
            req := httptest.NewRequest("GET", "/api/v1/user/search", nil)
            addToken(100001, req)
            q := req.URL.Query()
            q.Add("account", account)
            req.URL.RawQuery = q.Encode()
            w := httptest.NewRecorder()

            route.ServeHTTP(w, req)

            resp := equalHttpResp(t, w)
            expected := testData[idx]
            equalStruct(t, expected, resp["data"].(map[string]any))
        })
    }

    for range testDataCount {
        wg.Add(2)
        idx := rand.IntN(testDataCount)
        tt := testData[idx]
        // user := &m.ResponseUserInfo{ID: tt.ID, Username: tt.Username, Phone: tt.Phone, Email: tt.Email, Avatar: tt.Avatar}
        go subTest(idx, tt.Phone)
        go subTest(idx, tt.Email)
    }
    wg.Wait()
}

func TestGetUser(t *testing.T) {
    setupTestData()
    wg := &sync.WaitGroup{}
    for range testDataCount {
        wg.Add(1)
        go func() {
            defer wg.Done()
            idx := rand.IntN(testDataCount)
            tt := testData[idx]
            t.Run(fmt.Sprintf("get user %d", tt.ID), func(t *testing.T) {
                req := httptest.NewRequest("GET", "/api/v1/user/get", nil)
                addToken(tt.ID, req)
                w := httptest.NewRecorder()
                route.ServeHTTP(w, req)

                resp := equalHttpResp(t, w)
                expected := testData[idx]
                equalStruct(t, expected, resp["data"].(map[string]any))
            })
        }()
    }
    wg.Wait()
}

func TestUpdateUser(t *testing.T) {
    setupTestData()
    wg := &sync.WaitGroup{}
    subTest := func(tt *m.User, value, field string) {
        defer wg.Done()
        t.Run(fmt.Sprintf("update user %s", value), func(t *testing.T) {
            body, _ := json.Marshal(map[string]any{"value": value, "field": field})
            req := httptest.NewRequest("POST", "/api/v1/user/updateinfo", bytes.NewBuffer(body))
            addToken(tt.ID, req)
            w := httptest.NewRecorder()
            route.ServeHTTP(w, req)

            resp := equalHttpResp(t, w)
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
    wg := &sync.WaitGroup{}
    for range testDataCount {
        wg.Add(1)
        go func() {
            defer wg.Done()
            idx := rand.IntN(testDataCount)
            tt := testData[idx]
            t.Run(fmt.Sprintf("update password %d", tt.ID), func(t *testing.T) {
                body, _ := json.Marshal(map[string]string{"old": "aaaaaa", "new": "bbbbbb", "confirm": "bbbbbb"})
                req := httptest.NewRequest("POST", "/api/v1/user/updatepassword", bytes.NewBuffer(body))
                addToken(tt.ID, req)
                w := httptest.NewRecorder()
                route.ServeHTTP(w, req)

                resp := equalHttpResp(t, w)
                assert.Nil(t, resp["data"])
            })
        }()
    }
    wg.Wait()
}

func TestDeleteUser(t *testing.T) {
    setupTestData()
    tests := genTestData()
    wg := &sync.WaitGroup{}
    for _, tt := range tests {
        wg.Add(1)
        go func(tt *m.User) {
            defer wg.Done()
            t.Run(fmt.Sprintf("delete user %d", tt.ID), func(t *testing.T) {
                req := httptest.NewRequest("POST", "/api/v1/user/delete", nil)
                addToken(tt.ID, req)
                w := httptest.NewRecorder()
                route.ServeHTTP(w, req)

                resp := equalHttpResp(t, w)
                assert.Nil(t, resp["data"])
            })
        }(tt)
    }
    wg.Wait()
}

// func newRequestAndRun(method string, url string, body *bytes.Buffer) (*http.Request, *httptest.ResponseRecorder) {
//  req := httptest.NewRequest(method, url, body)
//  w := httptest.NewRecorder()
//  route.ServeHTTP(w, req)
//  return req, w
// }

func equalHttpResp(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]any
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, float64(200), resp["status"].(float64))
    assert.Equal(t, "OK", resp["message"].(string))
    return resp
}

func addToken(uid uint, req *http.Request) {
    token, err := s.Cache().GetToken(uid)
    if err != nil || token == "" {
        token, _ = middleware.GenerateToken(uid)
        s.Cache().SetToken(uid, token, s.Config().Common().TokenValidPeriod())
        s.Cache().Flush()
    }
    req.Header.Add("Authorization", "Bearer "+token)
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

