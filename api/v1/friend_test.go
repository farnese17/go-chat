package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/farnese17/chat/repository"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/stretchr/testify/assert"
)

func clearFriendData() {
	repo := s.User().(repository.TestableRepo)
	repo.ExecSql("DELETE FROM friend")
}

func genRandomFriendTestData(status int) map[int]int {
	users := genTestData()
	testData = append(testData, users...)
	for _, user := range users {
		if err := s.User().CreateUser(user); err != nil {
			panic(fmt.Errorf("setup test data error %v", err))
		}
	}
	data := make(map[int]int)
	for range len(testData) / 2 {
		a, b := randomUser()
		if a > b {
			a, b = b, a
		}
		if v, ok := data[a]; ok && b == v {
			continue
		}

		from, to := testData[a], testData[b]
		f := m.Friend{
			User1:       from.ID,
			User2:       to.ID,
			User1Remark: "remark",
			User1Group:  "group",
			Status:      status}
		if err := s.Friend().UpdateStatus(&f); err != nil {
			continue
		}
		data[a] = b
	}
	return data
}

func TestRequestFriend(t *testing.T) {
	setupTestData()

	for range testDataCount {
		wg.Add(1)
		from, to := randomUser()
		go runUpdataStatusTest(t, "POST", "request", fmt.Sprintf("/api/v1/friends/request/%d", testData[to].ID), from, to)
	}
	wg.Wait()

	id := testData[0].ID
	t.Run(fmt.Sprintf("request friend %d to %d", id, id), func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/friends/request/%d", id), nil)
		addToken(100001, req)
		w := httptest.NewRecorder()
		route.ServeHTTP(w, req)
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, float64(4006), resp["status"].(float64))
		assert.Equal(t, errorsx.ErrInvalidParams.Error(), resp["message"].(string))
		assert.Nil(t, resp["data"])
	})
}

func TestAcceptFriend(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSReq2To1)
	for from, to := range tests {
		wg.Add(1)
		go runUpdataStatusTest(t, "PUT", "accept", fmt.Sprintf("/api/v1/friends/accept/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func TestRejectFriend(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSReq2To1)
	for from, to := range tests {
		wg.Add(1)
		go runUpdataStatusTest(t, "PUT", "reject", fmt.Sprintf("/api/v1/friends/reject/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func TestDeleteFriend(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSAdded)
	for from, to := range tests {
		wg.Add(1)
		go runUpdataStatusTest(t, "DELETE", "delete", fmt.Sprintf("/api/v1/friends/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func TestBlockUser(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSAdded)
	for from, to := range tests {
		wg.Add(1)
		go runUpdataStatusTest(t, "PUT", "block", fmt.Sprintf("/api/v1/friends/block/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func TestUnblockUser(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSBlock1To2)
	for from, to := range tests {
		wg.Add(1)
		go runUpdataStatusTest(t, "PUT", "unblock", fmt.Sprintf("/api/v1/friends/unblock/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func runUpdataStatusTest(t *testing.T, method, name, url string, a, b int) {
	defer wg.Done()
	from, to := testData[a], testData[b]
	t.Run(fmt.Sprintf("%s friend %d to %d", name, from.ID, to.ID), func(t *testing.T) {
		req := httptest.NewRequest(method, url, nil)
		addToken(from.ID, req)
		w := httptest.NewRecorder()
		route.ServeHTTP(w, req)
		resp := equalHttpResp(t, w)
		assert.Nil(t, resp["data"])
	})
}

func randomUser() (int, int) {
	var a, b int
	for a == b {
		a = rand.IntN(len(testData))
		b = rand.IntN(len(testData))
	}
	return a, b
}

func TestRemark(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSAdded)

	for from, to := range tests {
		wg.Add(1)
		go runSetGroupAndRemarkTest(t, "remark", fmt.Sprintf("/api/v1/friends/remark/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func TestSetGroup(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSAdded)

	for from, to := range tests {
		wg.Add(1)
		go runSetGroupAndRemarkTest(t, "group", fmt.Sprintf("/api/v1/friends/setgroup/%d", testData[to].ID), from, to)
	}
	wg.Wait()
}

func runSetGroupAndRemarkTest(t *testing.T, name, url string, a, b int) {
	defer wg.Done()
	from, to := testData[a], testData[b]
	t.Run(fmt.Sprintf("%s %d-%d", name, from.ID, to.ID), func(t *testing.T) {
		req := httptest.NewRequest("PUT", url, nil)
		addToken(uint(from.ID), req)
		q := req.URL.Query()
		q.Add(name, "test ")
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		route.ServeHTTP(w, req)
		resp := equalHttpResp(t, w)
		assert.Nil(t, resp["data"])
	})
}

func TestGetFriend(t *testing.T) {
	clearFriendData()
	tests := genRandomFriendTestData(m.FSAdded)

	wg := sync.WaitGroup{}
	for a, b := range tests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			from, to := testData[a], testData[b]
			t.Run(fmt.Sprintf("get friend %d to %d", from.ID, to.ID), func(t *testing.T) {
				req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/friends/%d", to.ID), nil)
				addToken(from.ID, req)
				w := httptest.NewRecorder()
				route.ServeHTTP(w, req)
				resp := equalHttpResp(t, w)
				f := m.Friendinfo{
					UID:      to.ID,
					Username: to.Username,
					Avatar:   to.Avatar,
					Remark:   "remark",
					Group:    "group",
					Status:   m.FSAdded,
					Phone:    to.Phone,
					Email:    to.Email,
				}
				var expected map[string]any
				jsonData, _ := json.Marshal(f)
				json.Unmarshal(jsonData, &expected)
				for k, v := range resp["data"].(map[string]any) {
					if k == "id" {
						assert.NotEmpty(t, v)
						continue
					}
					assert.Equalf(t, expected[k], v, fmt.Sprintf("%s:\twant: %v\n\tgot: %v", k, expected[k], v))
				}
			})
		}()
	}
	wg.Wait()
}

func TestSearch_Friend(t *testing.T) {
	clear()
	testData = []*m.User{}
	setupTestData()
	n := len(testData) / 10
	cursors := make(map[string]m.Cursor)
	expected := make(map[string][]*m.User)
	base := 10
	for i := 1; i < n; i++ {
		idx := base
		key := "test" + strconv.FormatInt(int64(i), 10)
		data := []*m.User{}
		data = append(data, testData[i])
		for range 10 {
			data = append(data, testData[idx])
			idx++
		}
		expected[key] = data
		cursors[key] = m.Cursor{PageSize: 10, HasMore: true, LastID: testData[0].ID + uint(idx-2)}
		base += 10
	}

	wg := sync.WaitGroup{}
	for value, expect := range expected {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("search friend %s", value), func(t *testing.T) {
				body, _ := json.Marshal(m.Cursor{PageSize: 10, HasMore: true})
				req := httptest.NewRequest("GET", "/api/v1/friends/search", bytes.NewBuffer(body))
				addToken(testData[0].ID, req)
				q := req.URL.Query()
				q.Add("value", value)
				req.URL.RawQuery = q.Encode()
				w := httptest.NewRecorder()
				route.ServeHTTP(w, req)

				resp := equalHttpResp(t, w)
				t.Log(resp)
				cursor := resp["data"].(map[string]any)["cursor"].(map[string]any)
				expectedCursor := cursors[value]
				assert.Equal(t, expectedCursor.PageSize, int(cursor["page_size"].(float64)))
				assert.Equal(t, expectedCursor.HasMore, cursor["has_more"].(bool))
				assert.Equal(t, expectedCursor.LastID, uint(cursor["last_id"].(float64)))
				var f []m.Friendinfo
				jsonData, _ := json.Marshal(expect)
				json.Unmarshal(jsonData, &f)
				var respf []m.Friendinfo
				respData, _ := json.Marshal(expect)
				json.Unmarshal(respData, &respf)
				assert.Equal(t, f, respf)
			})
		}()
	}
	wg.Wait()
}

func TestGetFriendList(t *testing.T) {
	clearFriendData()
	setupTestData()
	n := len(testData) / 10
	genFriendListTestData(n, m.FSAdded)

	wg := sync.WaitGroup{}
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := testData[i].ID
			t.Run(fmt.Sprintf("get friend list %d", id), func(t *testing.T) {
				req := httptest.NewRequest("GET", "/api/v1/friends", nil)
				addToken(id, req)
				w := httptest.NewRecorder()
				route.ServeHTTP(w, req)

				resp := equalHttpResp(t, w)
				assert.NotEmpty(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

// func TestGetBlockedMeList(t *testing.T) {
// 	setupTestData()
// 	clearFriendData()
// 	n := len(testData) / 10
// 	genFriendListTestData(n, m.FSBlock2To1)

// 	expected := make([]any, n)
// 	for i := range expected {
// 		expected[i] = float64(testData[i].ID)
// 	}

// 	wg := &sync.WaitGroup{}
// 	for i := range n {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			id := testData[i].ID
// 			t.Run(fmt.Sprintf("get blocked me list %d", id), func(t *testing.T) {
// 				req := httptest.NewRequest("GET", "/api/v1/friend/blockedmelist", nil)
// 				addToken(id, req)
// 				w := httptest.NewRecorder()
// 				route.ServeHTTP(w, req)

// 				resp := equalHttpResp(t, w)
// 				assert.Equal(t, expected[i+1:n], resp["data"])
// 			})
// 		}()
// 	}
// 	wg.Wait()
// }

func genFriendListTestData(n int, status int) {
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			f := m.Friend{User1: testData[i].ID, User2: testData[j].ID, Status: status}
			if err := s.Friend().UpdateStatus(&f); err != nil {
				panic(err)
			}
		}
	}
}
