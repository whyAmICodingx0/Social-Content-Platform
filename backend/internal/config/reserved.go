package config

// 保留 username 清單（API Spec v1.2 5.3）。
// 判斷時機在正規化（轉小寫）之後，所以這裡全小寫即可。
var reservedUsernames = map[string]struct{}{
	"me": {}, "admin": {}, "administrator": {}, "api": {}, "auth": {},
	"login": {}, "logout": {}, "signup": {}, "signin": {}, "signout": {},
	"onboarding": {}, "users": {}, "user": {}, "posts": {}, "post": {},
	"tags": {}, "tag": {}, "settings": {}, "account": {}, "accounts": {},
	"profile": {}, "support": {}, "help": {}, "system": {}, "moderator": {},
	"mod": {}, "root": {}, "null": {}, "undefined": {}, "official": {},
	"security": {}, "abuse": {}, "about": {}, "terms": {}, "privacy": {},
	"search": {}, "explore": {}, "new": {}, "edit": {}, "delete": {},
	"static": {}, "assets": {}, "www": {}, "mail": {}, "feed": {}, "home": {},
}

func IsReservedUsername(u string) bool {
	_, ok := reservedUsernames[u]
	return ok
}
