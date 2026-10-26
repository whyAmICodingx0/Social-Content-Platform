package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// ---- 假的 session store 與使用者查詢(取代 Redis 與 PostgreSQL)----

type fakeSessions struct {
	userID string
	err    error
	calls  int
	gotSID string
}

func (f *fakeSessions) GetUserID(_ context.Context, sid string) (string, error) {
	f.calls++
	f.gotSID = sid
	return f.userID, f.err
}

type fakeUsers struct {
	user  *repository.User
	err   error
	gotID string
}

func (f *fakeUsers) GetByID(_ context.Context, id string) (*repository.User, error) {
	f.gotID = id
	return f.user, f.err
}

// ---- 測試情境 ----

const (
	testSID    = "sid-from-cookie"
	testUserID = "user-1"
)

// authCase 描述一種情境;Required 與 Optional 用同一組情境測,
// 各自的預期結果不同。
type authCase struct {
	name     string
	cookie   *http.Cookie // nil = 沒帶 sid cookie
	sessErr  error        // session store 回傳的錯誤(nil = 找到 session)
	usersErr error        // 使用者查詢回傳的錯誤(nil = 找到使用者)

	// Required 的預期
	wantStatus int
	wantCode   string
}

var sidCookie = &http.Cookie{Name: cookies.NameSID, Value: testSID}

var authCases = []authCase{
	{name: "沒有 sid cookie",
		wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthenticated},
	{name: "sid cookie 是空字串",
		cookie:     &http.Cookie{Name: cookies.NameSID, Value: ""},
		wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthenticated},
	{name: "session 不存在(過期或偽造)",
		cookie: sidCookie, sessErr: ErrSessionNotFound,
		wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthenticated},
	{name: "包裝過的 ErrSessionNotFound 也算憑證無效",
		cookie: sidCookie, sessErr: fmt.Errorf("lookup: %w", ErrSessionNotFound),
		wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthenticated},
	{name: "Redis 連不上(與 store.SessionStore 相同的包裝方式)",
		cookie: sidCookie, sessErr: fmt.Errorf("%w: dial tcp: connection refused", ErrStoreUnavailable),
		wantStatus: http.StatusServiceUnavailable, wantCode: api.CodeServiceUnavailable},
	{name: "未預期的錯誤也不放行(fail closed)",
		cookie: sidCookie, sessErr: context.DeadlineExceeded,
		wantStatus: http.StatusServiceUnavailable, wantCode: api.CodeServiceUnavailable},
	{name: "session 有效但使用者已刪除",
		cookie: sidCookie, usersErr: repository.ErrNotFound,
		wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthenticated},
	{name: "session 有效但資料庫錯誤",
		cookie: sidCookie, usersErr: errors.New("db: connection reset"),
		wantStatus: http.StatusServiceUnavailable, wantCode: api.CodeServiceUnavailable},
	{name: "一切正常",
		cookie:     sidCookie,
		wantStatus: http.StatusOK},
}

// result 是一次請求的完整結果。
type result struct {
	rec      *httptest.ResponseRecorder
	reached  bool             // 後面的 handler 有沒有被執行
	user     *repository.User // 後面的 handler 透過 CurrentUser 拿到的使用者
	loggedIn bool             // CurrentUser 的第二個回傳值
	sessions *fakeSessions
	users    *fakeUsers
}

func runAuth(tc authCase, pick func(*Auth) gin.HandlerFunc) result {
	sessions := &fakeSessions{userID: testUserID, err: tc.sessErr}
	users := &fakeUsers{user: &repository.User{ID: testUserID}, err: tc.usersErr}
	if tc.usersErr != nil {
		users.user = nil
	}
	a := &Auth{Store: sessions, Users: users}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if tc.cookie != nil {
		req.AddCookie(tc.cookie)
	}

	res := result{sessions: sessions, users: users}
	res.rec, res.reached = serve(pick(a), req, func(c *gin.Context) {
		res.user, res.loggedIn = CurrentUser(c)
	})
	return res
}

func TestRequired(t *testing.T) {
	for _, tc := range authCases {
		t.Run(tc.name, func(t *testing.T) {
			res := runAuth(tc, (*Auth).Required)

			if res.rec.Code != tc.wantStatus {
				t.Fatalf("狀態碼 = %d,預期 %d(body = %q)", res.rec.Code, tc.wantStatus, res.rec.Body.String())
			}

			if tc.wantStatus != http.StatusOK {
				// 被擋下:後面的 handler 絕對不能執行
				if res.reached {
					t.Fatal("請求被拒絕,但後面的 handler 仍然執行了")
				}
				if got := errorCode(t, res.rec); got != tc.wantCode {
					t.Fatalf("error.code = %q,預期 %q", got, tc.wantCode)
				}
				return
			}

			// 放行:handler 拿得到正確的使用者
			if !res.reached || !res.loggedIn || res.user == nil || res.user.ID != testUserID {
				t.Fatalf("handler 沒拿到使用者:reached=%v loggedIn=%v user=%+v",
					res.reached, res.loggedIn, res.user)
			}
			// 查的是 cookie 裡的那個 session、那個 session 指向的使用者
			if res.sessions.gotSID != testSID {
				t.Fatalf("查詢的 session id = %q,預期 cookie 裡的 %q", res.sessions.gotSID, testSID)
			}
			if res.users.gotID != testUserID {
				t.Fatalf("查詢的 user id = %q,預期 session 裡的 %q", res.users.gotID, testUserID)
			}
		})
	}
}

func TestOptional(t *testing.T) {
	for _, tc := range authCases {
		t.Run(tc.name, func(t *testing.T) {
			res := runAuth(tc, (*Auth).Optional)

			// 任何情況都放行:失敗一律降級為匿名,絕不回 401 / 503(決策 #15)
			if res.rec.Code != http.StatusOK || !res.reached {
				t.Fatalf("optional auth 必須放行:狀態碼 = %d,reached = %v(body = %q)",
					res.rec.Code, res.reached, res.rec.Body.String())
			}

			wantLoggedIn := tc.wantStatus == http.StatusOK
			if res.loggedIn != wantLoggedIn {
				t.Fatalf("CurrentUser 是否登入 = %v,預期 %v", res.loggedIn, wantLoggedIn)
			}
			if wantLoggedIn && (res.user == nil || res.user.ID != testUserID) {
				t.Fatalf("CurrentUser = %+v,預期 ID %q", res.user, testUserID)
			}
		})
	}
}

// 沒帶 cookie 時不應該去查 session store(匿名訪客不產生 Redis 查詢)。
func TestAuth_NoCookieSkipsStore(t *testing.T) {
	for _, pick := range []func(*Auth) gin.HandlerFunc{(*Auth).Required, (*Auth).Optional} {
		res := runAuth(authCase{}, pick) // 零值 = 沒帶 cookie
		if res.sessions.calls != 0 {
			t.Fatalf("沒有 cookie 卻查了 session store %d 次", res.sessions.calls)
		}
	}
}

// spec 4.11:401 不區分原因。三種原因的回應必須一模一樣,
// 否則攻擊者能從回應差異判斷「這個 session id 曾經存在」或「這個帳號被刪了」。
func TestRequired_401DoesNotRevealReason(t *testing.T) {
	var first string
	for _, tc := range authCases {
		if tc.wantStatus != http.StatusUnauthorized {
			continue
		}
		body := runAuth(tc, (*Auth).Required).rec.Body.String()
		if first == "" {
			first = body
			continue
		}
		if body != first {
			t.Fatalf("「%s」的 401 回應與其他原因不同:\n  %s\n  %s", tc.name, body, first)
		}
	}
}
