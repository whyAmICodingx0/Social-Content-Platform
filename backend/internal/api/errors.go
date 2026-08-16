package api

// 錯誤碼合約(spec 3.2)。這些字串是前後端合約的一部分:
// 前端以 code 判斷分支,所以字串一個字都不能隨意改。
const (
	CodeValidationError      = "VALIDATION_ERROR"
	CodeUnauthenticated      = "UNAUTHENTICATED"
	CodeForbidden            = "FORBIDDEN"
	CodeNotFound             = "NOT_FOUND"
	CodeUsernameTaken        = "USERNAME_TAKEN"
	CodeEmailTaken           = "EMAIL_TAKEN"
	CodeSlugConflict         = "SLUG_CONFLICT"
	CodeConflict             = "CONFLICT"
	CodeUnsupportedMediaType = "UNSUPPORTED_MEDIA_TYPE"
	CodeInternalError        = "INTERNAL_ERROR"
	CodeServiceUnavailable   = "SERVICE_UNAVAILABLE"
	// Phase 3
	CodeCannotMessageSelf = "CANNOT_MESSAGE_SELF"
	CodeInvalidCursor     = "INVALID_CURSOR" // P3-3 使用，一併定義避免重複改檔
)
