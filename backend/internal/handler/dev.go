package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

type DevHandler struct {
	Users *repository.UserRepository
}

type echoRequest struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// Echo:驗證 BindStrict 的行為。
func (h *DevHandler) Echo(c *gin.Context) {
	var req echoRequest
	if !api.BindStrict(c, &req) {
		return
	}
	api.OK(c, req)
}

// GetUser:驗證 repository 的 deleted_at 封裝。
// 注意:id 亂打非 UUID 會回 500(dev 工具不做格式驗證;
// 正式端點在任務 H 會先驗格式)。
func (h *DevHandler) GetUser(c *gin.Context) {
	u, err := h.Users.GetByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		api.Fail(c, http.StatusInternalServerError, api.CodeInternalError, "unexpected error")
		return
	}
	api.OK(c, gin.H{"id": u.ID, "username": u.Username, "email": u.Email})
}

// WhoAmI:驗證 optional auth(帶無效 sid 也不該 401)。
func (h *DevHandler) WhoAmI(c *gin.Context) {
	if u, ok := middleware.CurrentUser(c); ok {
		api.OK(c, gin.H{"authenticated": true, "username": u.Username})
		return
	}
	api.OK(c, gin.H{"authenticated": false})
}

// OnboardingPage:dev 專用的極簡 onboarding 頁。
// 真前端(Vue)完成後刪除;它存在的意義是讓任務 F 能全程用瀏覽器驗收。
func (h *DevHandler) OnboardingPage(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, onboardingHTML)
}

const onboardingHTML = `<!doctype html>
<meta charset="utf-8">
<title>Dev Onboarding</title>
<style>body{font-family:sans-serif;max-width:480px;margin:40px auto}input{display:block;width:100%;margin:8px 0;padding:8px}</style>
<h2>選擇你的 username</h2>
<input id="u" placeholder="username(3-30 字,小寫英數與底線)">
<input id="d" placeholder="display name(選填)">
<button onclick="signup()">送出</button>
<button onclick="logout()">測試登出</button>
<pre id="out"></pre>
<script>
async function signup(){
  const body = { username: document.getElementById('u').value };
  const d = document.getElementById('d').value;
  if (d) body.display_name = d;
  const res = await fetch('/api/v1/auth/signup', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body)
  });
  show(res.status, await res.json());
  if (res.ok) document.getElementById('out').textContent +=
    '\n\n登入成功!開新分頁到 http://localhost:8080/api/v1/me 看看';
}
async function logout(){
  const res = await fetch('/api/v1/auth/logout', {method: 'POST'});
  show(res.status, res.status === 204 ? '(no content)' : await res.json());
}
function show(status, data){
  document.getElementById('out').textContent =
    'HTTP ' + status + '\n' + JSON.stringify(data, null, 2);
}
</script>`
