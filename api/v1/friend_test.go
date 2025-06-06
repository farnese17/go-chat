package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
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
		url := fmt.Sprintf("/api/v1/friends/request/%d", id)
		testHasError(t, route, url, "POST", 100001, nil, errorsx.ErrInvalidParams)
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
		resp := testNoError(t, route, url, method, from.ID, nil)
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
		resp := testNoError(t, route, url, "PUT", from.ID, nil, map[string]string{
			"name": "test "})
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
				url := fmt.Sprintf("/api/v1/friends/%d", to.ID)
				resp := testNoError(t, route, url, "GET", from.ID, nil)
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

	url := "/api/v1/friends/search"
	wg := sync.WaitGroup{}
	for value, expect := range expected {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("search friend %s", value), func(t *testing.T) {
				body, _ := json.Marshal(m.Cursor{PageSize: 10, HasMore: true})
				resp := testNoError(t, route, url, "GET", testData[0].ID, bytes.NewBuffer(body), map[string]string{
					"value": value})

				var f []m.Friendinfo
				jsonData, _ := json.Marshal(expect)
				json.Unmarshal(jsonData, &f)
				var respf []m.Friendinfo
				respData, _ := json.Marshal(expect)
				json.Unmarshal(respData, &respf)
				assert.Equal(t, f, respf)

				expectedCursor := cursors[value]
				equalStruct(t, expectedCursor, resp["data"].(map[string]any)["cursor"].(map[string]any))
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

	url := "/api/v1/friends"
	wg := sync.WaitGroup{}
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := testData[i].ID
			t.Run(fmt.Sprintf("get friend list %d", id), func(t *testing.T) {
				resp := testNoError(t, route, url, "GET", id, nil)
				assert.NotEmpty(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

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
