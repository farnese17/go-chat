package v1_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/service/model"
	m "github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/validator"
	ws "github.com/farnese17/chat/websocket"
	"github.com/stretchr/testify/assert"
)

var (
	testGroupData []*m.Group
)

func setupTestGroupData() {
	testGroupData = genTestGroupData()
	for _, g := range testGroupData {
		if err := s.Group().Create(g); err != nil {
			panic(err)
		}
	}
}

func genTestGroupData() []*m.Group {
	testGroupData := make([]*m.Group, testDataCount)
	uid := uint(1e5)
	for i := range testDataCount {
		uid++
		testGroupData[i] = &m.Group{
			Name:    "group" + strconv.FormatInt(int64(i), 10),
			Owner:   uid,
			Founder: uid,
			Desc:    "This is description",
		}
	}
	return testGroupData
}

func clearGroupData() {
	repo := s.User().(repository.TestableRepo)
	repo.ExecSql("DELETE FROM `group`")
	repo.ExecSql("ALTER TABLE `group` AUTO_INCREMENT = 1000000001")
	repo.ExecSql("ALTER TABLE group_person AUTO_INCREMENT = 1")
}

func TestCreateGroup(t *testing.T) {
	setupTestData()

	tests := genTestGroupData()
	url := "/api/v1/groups"
	wg := &sync.WaitGroup{}
	for i, tt := range tests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("create group %s", tt.Name), func(t *testing.T) {
				body, _ := json.Marshal(tt)
				var uid uint
				if i < len(testData) {
					uid = testData[i].ID
				}
				resp := testNoError(t, route, url, "POST", uid, bytes.NewBuffer(body), nil)
				assert.NotEmpty(t, resp["data"])
				jsonData, _ := json.Marshal(resp["data"])
				var g *m.Group
				json.Unmarshal(jsonData, &g)
				if err := validator.ValidateGID(g.GID); err != nil {
					t.Errorf("invalid gid: %d", g.GID)
				}
				tt.GID = g.GID
				tt.CreatedAt = g.CreatedAt
				tt.LastTime = g.LastTime
				assert.Equal(t, tt, g)
			})
		}()
	}
	wg.Wait()
}

func TestSearchByGID(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount)
			tt := testGroupData[idx]
			t.Run(fmt.Sprintf("search group by gid %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d", tt.GID)
				resp := testNoError(t, route, url, "GET", tt.Owner, nil)
				jsonData, _ := json.Marshal(resp["data"])
				var g *m.Group
				json.Unmarshal(jsonData, &g)
				tt.CreatedAt = g.CreatedAt
				assert.Equal(t, tt, g)
			})
		}()
	}
	wg.Wait()
}

func TestSearchGroupByName(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	url := "/api/v1/groups/search"
	wg := &sync.WaitGroup{}
	for i := range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("search group by name %d", i), func(t *testing.T) {
				body, _ := json.Marshal(&m.Cursor{PageSize: 1, HasMore: true})
				resp := testNoError(t, route, url, "GET", testGroupData[0].Owner, bytes.NewBuffer(body), map[string]string{
					"name": "group"})
				cursor := &model.Cursor{LastID: uint(1e9 + 1), PageSize: 1, HasMore: true}
				equalStruct(t, cursor, resp["data"].(map[string]any)["cursor"].(map[string]any))
				var g []*m.Group
				jsong, _ := json.Marshal(resp["data"].(map[string]any)["groups"])
				json.Unmarshal(jsong, &g)
				testGroupData[0].CreatedAt = g[0].CreatedAt
				assert.Equal(t, testGroupData[:1], g)
			})
		}()
	}
	wg.Wait()
}

func TestSearchGroupByName_Cursor(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	tests := []struct {
		cursor         m.Cursor
		expected       []*m.Group
		expectedCursor m.Cursor
	}{
		{m.Cursor{PageSize: 15, LastID: 0, HasMore: true}, testGroupData[:15], m.Cursor{PageSize: 15, LastID: uint(1e9) + 15, HasMore: true}},
		{m.Cursor{PageSize: 15, LastID: uint(int(1e9)+testDataCount) - 5, HasMore: true}, testGroupData[testDataCount-5:], m.Cursor{PageSize: 15, LastID: uint(int(1e9)+testDataCount) - 5, HasMore: false}},
		{m.Cursor{PageSize: 15, LastID: uint(int(1e9) + testDataCount/2), HasMore: true}, testGroupData[testDataCount/2 : 15+testDataCount/2], m.Cursor{PageSize: 15, LastID: uint(int(1e9) + 15 + testDataCount/2), HasMore: true}},
		{m.Cursor{PageSize: 15, LastID: uint(int(1e9)+testDataCount) + 1, HasMore: true}, []*m.Group{}, m.Cursor{PageSize: 15, LastID: uint(int(1e9)+testDataCount) + 1, HasMore: false}},
	}

	url := "/api/v1/groups/search"
	wg := &sync.WaitGroup{}
	for _, tt := range tests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("use cursor search group by name %v", tt.cursor), func(t *testing.T) {
				body, _ := json.Marshal(tt.cursor)
				resp := testNoError(t, route, url, "GET", testData[0].ID, bytes.NewBuffer(body), map[string]string{
					"name": "group"})
				equalStruct(t, tt.expectedCursor, resp["data"].(map[string]any)["cursor"].(map[string]any))
				var g []*m.Group
				jsong, _ := json.Marshal(resp["data"].(map[string]any)["groups"])
				json.Unmarshal(jsong, &g)
				for i := range g {
					tt.expected[i].CreatedAt = g[i].CreatedAt
				}
				assert.Equal(t, tt.expected, g)
			})
		}()
	}
	wg.Wait()
}

func TestInvite(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}

	from := testGroupData[0].Owner
	gid := testGroupData[0].GID
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount-1) + 1
			tt := testData[idx]
			t.Run(fmt.Sprintf("invite user join group %d", tt.ID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/invitations/%d", gid, tt.ID)
				resp := testNoError(t, route, url, "POST", from, nil)
				msg := &ws.ChatMsg{
					Type: ws.System,
					From: testData[0].ID,
					To:   tt.ID,
					Body: fmt.Sprintf("邀请 %s 加入群聊 group0", tt.Username),
				}
				equalStruct(t, msg, resp["data"].(map[string]any), "time")
				assert.NotEmpty(t, resp["data"].(map[string]any)["time"])
			})
		}()
	}

	wg.Wait()
}

func TestAcceptInvite(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	msg := ws.ChatMsg{
		Type: ws.System,
		Time: time.Now().Add(-time.Minute).UnixMilli(),
	}
	for i := range testDataCount {
		wg.Add(1)
		idx := i + 1
		for idx == i+1 {
			idx = rand.IntN(testDataCount-1) + 1
		}
		msg.From = uint(1e5) + uint(idx)
		msg.To = uint(1e5) + uint(i) + 1
		msg.Data = int(1e9) + idx
		go func(msg ws.ChatMsg) {
			defer wg.Done()
			t.Run(fmt.Sprintf("accept invite %d", msg.To), func(t *testing.T) {
				body, _ := json.Marshal(msg)
				url := fmt.Sprintf("/api/v1/groups/%v/invitations/accept", msg.Data)
				resp := testNoError(t, route, url, "PUT", msg.To, bytes.NewBuffer(body))
				assert.Nil(t, resp["data"])
			})
		}(msg)
	}
	wg.Wait()
}

func TestApply(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	for i := range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := i + 1
			for idx == i+1 {
				idx = rand.IntN(testDataCount-1) + 1
			}
			uid := uint(1e5) + uint(i+1)
			gid := int(1e9) + idx
			t.Run(fmt.Sprintf("%d apply join group %d", uid, gid), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/applications", gid)
				resp := testNoError(t, route, url, "POST", uid, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func genTestMembers(t *testing.T, status int) {
	for _, tt := range testGroupData[1:] {
		ctx := &m.MemberStatusContext{
			GID:       tt.GID,
			From:      tt.Owner,
			To:        testGroupData[0].Owner,
			NewStatus: status,
		}
		if err := s.Group().CreateMember(ctx); err != nil {
			t.Error(err)
		}
	}
}

func TestAcceptApply(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleApplied)
	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("accept join group %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/applications/%d/accept", tt.GID, testGroupData[0].Owner)
				resp := testNoError(t, route, url, "PUT", tt.Owner, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestRejectApply(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleApplied)
	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("reject join group %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/applications/%d/reject", tt.GID, testGroupData[0].Owner)
				resp := testNoError(t, route, url, "PUT", tt.Owner, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestGetMember(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount)
			tt := testGroupData[idx]
			t.Run(fmt.Sprintf("get member %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/members/%d", tt.GID, tt.Owner)
				resp := testNoError(t, route, url, "GET", tt.Owner, nil)
				expected := testData[idx]
				data := resp["data"].(map[string]any)
				equalStruct(t, expected, data, "role", "created_at")
				assert.NotEmpty(t, data["role"])
				assert.NotEmpty(t, data["created_at"])
			})
		}()
	}
	wg.Wait()
}

func TestGetMembers(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount)
			tt := testGroupData[idx]
			t.Run(fmt.Sprintf("get group members %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/members", tt.GID)
				resp := testNoError(t, route, url, "GET", uint(1e5+1), nil)
				var members []*m.MemberInfo
				jsonData, _ := json.Marshal(resp["data"])
				json.Unmarshal(jsonData, &members)
				for _, member := range members {
					var m map[string]any
					jsonData, _ := json.Marshal(member)
					json.Unmarshal(jsonData, &m)
					expected := testData[idx]
					equalStruct(t, expected, m, "role", "created_at")
					assert.NotEmpty(t, m["role"])
					assert.NotEmpty(t, m["created_at"])
				}
			})
		}()
	}
	wg.Wait()
}

func TestDeleteGroup(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("delete group %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d", tt.GID)
				resp := testNoError(t, route, url, "DELETE", tt.Owner, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestUpdateGroupInformation(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}

	subTest := func(column, value string) {
		defer wg.Done()
		idx := rand.IntN(testDataCount)
		tt := testGroupData[idx]
		t.Run(fmt.Sprintf("update group information %d", tt.GID), func(t *testing.T) {
			url := fmt.Sprintf("/api/v1/groups/%d", tt.GID)
			resp := testNoError(t, route, url, "PUT", tt.Owner, nil, map[string]string{
				"field": column, "value": value})
			assert.Nil(t, resp["data"])
		})
	}
	for i := range testDataCount {
		wg.Add(2)
		go subTest("name", fmt.Sprintf("new name%d", i))
		go subTest("desc", fmt.Sprintf("new desc%d", i))
	}
	wg.Wait()
}

func TestHandOverOwner(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleMember)
	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("hand over owner %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/owner/%d", tt.GID, testGroupData[0].Owner)
				resp := testNoError(t, route, url, "PUT", tt.Owner, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestModifyAdmin(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleMember)
	wg := &sync.WaitGroup{}

	subTest := func(tt *m.Group, status int) {
		defer wg.Done()
		t.Run(fmt.Sprintf("modify admin %d", tt.GID), func(t *testing.T) {
			url := fmt.Sprintf("/api/v1/groups/%d/admins/%d", tt.GID, testGroupData[0].Owner)
			resp := testNoError(t, route, url, "PUT", tt.Owner, nil, map[string]string{
				"role": strconv.FormatInt(int64(status), 10)})
			assert.Nil(t, resp["data"])
		})
	}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go subTest(tt, m.GroupRoleAdmin)
	}
	wg.Wait()
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go subTest(tt, m.GroupRoleMember)
	}
	wg.Wait()
}

func TestAdminResgin(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleAdmin)
	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("admin resign%d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/admins/me/resign", tt.GID)
				resp := testNoError(t, route, url, "PUT", testData[0].ID, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestLeaveGroup(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleAdmin)
	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("leave group %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/members/me", tt.GID)
				resp := testNoError(t, route, url, "DELETE", testGroupData[0].Owner, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestKick(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	genTestMembers(t, m.GroupRoleMember)
	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData[1:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("kick member %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/members/%d", tt.GID, testGroupData[0].Owner)
				resp := testNoError(t, route, url, "DELETE", tt.Owner, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestCreateAnnounce(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("craete announce %d", tt.GID), func(t *testing.T) {
				body, _ := json.Marshal(m.GroupAnnouncement{GroupID: tt.GID, Content: "this is an announce"})
				url := fmt.Sprintf("/api/v1/groups/%d/announces", tt.GID)
				resp := testNoError(t, route, url, "POST", tt.Owner, bytes.NewBuffer(body))
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

var testAnnounceData = make([]*m.GroupAnnouncement, 0, testDataCount*3)

func setupGroupAnnounce() {
	for i := range 2 {
		for _, g := range testGroupData {
			announce := &m.GroupAnnouncement{GroupID: g.GID, CreatedBy: g.Owner}
			announce.Content = fmt.Sprintf("this is an announce%d", i)
			testAnnounceData = append(testAnnounceData, announce)
		}
	}

	for _, announce := range testAnnounceData {
		if err := s.Group().ReleaseAnnounce(announce); err != nil {
			panic(err)
		}
	}
}

func TestViewAnnounce(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()
	setupGroupAnnounce()

	wg := &sync.WaitGroup{}
	for _, tt := range testGroupData {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("view announce %d", tt.GID), func(t *testing.T) {
				body, _ := json.Marshal(m.Cursor{PageSize: 15, LastID: 0, HasMore: true})
				url := fmt.Sprintf("/api/v1/groups/%d/announces", tt.GID)
				resp := testNoError(t, route, url, "GET", tt.Owner, bytes.NewBuffer(body))
				cursor := &model.Cursor{LastID: math.MaxUint64, PageSize: 15}
				equalStruct(t, cursor, resp["data"].(map[string]any)["cursor"].(map[string]any))

				base := int(tt.GID - testGroupData[0].GID)
				expected := make([]*m.GroupAnnouncement, len(testAnnounceData)/testDataCount)
				for i := len(expected) - 1; i >= 0; i-- {
					expected[i] = testAnnounceData[base]
					base += testDataCount
				}
				v := reflect.ValueOf(resp["data"].(map[string]any)["data"])
				assert.Equal(t, len(expected), v.Len())
				for i := range v.Len() {
					elem := v.Index(i).Elem()
					var m map[string]any
					jsonData, _ := json.Marshal(expected[i])
					json.Unmarshal(jsonData, &m)
					keys := elem.MapKeys()
					for _, k := range keys {
						val := elem.MapIndex(k).Interface()
						if k.String() == "created_by" {
							exp := testData[expected[i].CreatedBy-testData[0].ID].Username
							assert.Equal(t, exp, val)
							continue
						}
						assert.Equal(t, m[k.String()], val)
					}
				}
			})
		}()
	}
	wg.Wait()
}

func TestViewLatestAnnounce(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()
	setupGroupAnnounce()

	wg := &sync.WaitGroup{}
	for range testDataCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx := rand.IntN(testDataCount)
			tt := testGroupData[idx]
			t.Run(fmt.Sprintf("view latest announce %d", tt.GID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/announces/latest", tt.GID)
				resp := testNoError(t, route, url, "GET", tt.Owner, nil)

				expected := testAnnounceData[len(testAnnounceData)+idx-testDataCount]
				equalStruct(t, expected, resp["data"].(map[string]any), "created_by")
				assert.Equal(t, testData[idx].Username, resp["data"].(map[string]any)["created_by"])
			})
		}()
	}
	wg.Wait()
}

func TestDeleteAnnounce(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()
	setupGroupAnnounce()

	wg := &sync.WaitGroup{}
	for _, tt := range testAnnounceData {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("delete announce %d", tt.GroupID), func(t *testing.T) {
				url := fmt.Sprintf("/api/v1/groups/%d/announces/%d", tt.GroupID, tt.ID)
				resp := testNoError(t, route, url, "DELETE", tt.CreatedBy, nil)
				assert.Nil(t, resp["data"])
			})
		}()
	}
	wg.Wait()
}

func TestGetGroupList(t *testing.T) {
	clearGroupData()
	setupTestData()
	setupTestGroupData()

	for i := 1; i < len(testGroupData); i++ {
		g := testGroupData[i]
		ctx := m.MemberStatusContext{GID: g.GID, From: g.Owner, To: testGroupData[i-1].Owner, NewStatus: m.GroupRoleMember}
		if err := s.Group().CreateMember(&ctx); err != nil {
			panic(err)
		}
	}

	url := "/api/v1/groups"
	wg := &sync.WaitGroup{}
	for i, tt := range testGroupData[:len(testGroupData)-1] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("get group list %d", tt.Owner), func(t *testing.T) {
				resp := testNoError(t, route, url, "GET", tt.Owner, nil)
				expected := []m.SummaryGroupInfo{
					{GID: tt.GID, GroupName: tt.Name},
					{GID: testGroupData[i+1].GID, GroupName: testGroupData[i+1].Name},
				}
				var groups []m.SummaryGroupInfo
				jsonData, _ := json.Marshal(resp["data"])
				json.Unmarshal(jsonData, &groups)
				assert.Equal(t, expected, groups)
			})
		}()
	}
	wg.Wait()
}
