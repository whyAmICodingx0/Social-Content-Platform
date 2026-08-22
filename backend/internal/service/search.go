package service

import (
	"strings"
	"unicode/utf8"
)

const (
	minSearchLen = 2   // 決策 #97：維持 2 以支援中文兩字詞
	maxSearchLen = 100
)

// SearchTerms 是查詢字串處理的結果。
//
// 兩個欄位刻意分開：
//   Pattern —— 給 ILIKE 用，已 escape 並加上 %
//   Query   —— 給 word_similarity 用，trim 後但**未 escape**
//
// ⚠️ 兩者不可互換。若把 escape 後的字串餵給 word_similarity，
// 使用者搜「50%」會變成對「50\%」算 trigram，排序失準。
type SearchTerms struct {
	Pattern string
	Query   string
}

// likeEscaper 處理 LIKE 的特殊字元。
//
// ⚠️ 順序重要：反斜線必須先處理，否則後面補上的跳脫字元會被二次跳脫。
// strings.NewReplacer 是「單次掃描、不重複替換」，所以這裡是安全的，
// 但把反斜線放第一個仍是慣例（換成手寫迴圈時才不會踩到）。
//
// 用反引號 raw string 避免二次轉義看錯 —— `\\` 在 raw string 裡就是兩個反斜線。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// ParseSearchQuery 驗證並處理查詢字串（決策 #96）。
//
// 不 escape 的後果：使用者搜「%」會匹配全部文章、搜「_」會匹配任一字元。
// 這不是安全漏洞（參數化查詢仍防 SQL injection），但是明顯的行為錯誤。
//
// ⚠️ 測試 escape 時必須用 ≥2 字元的值（例如 "50%"）——
// 單一字元會先被下方的長度檢查擋成 400，escape 根本不會被執行到。
func ParseSearchQuery(raw string) (*SearchTerms, error) {
	q := strings.TrimSpace(raw)

	// 用 rune 計數，不是 byte —— 一個中文字 3 bytes，
	// 用 len() 會讓「程式」被當成 6 而通過、「台灣人」被當成 9 卻算成超長。
	// 這與決策 #46（留言長度）是同一個坑。
	n := utf8.RuneCountInString(q)
	if n < minSearchLen || n > maxSearchLen {
		return nil, &ValidationError{
			Field:   "q",
			Message: "must be 2-100 characters",
		}
	}

	return &SearchTerms{
		Pattern: "%" + likeEscaper.Replace(q) + "%",
		Query:   q, // 未 escape，供 word_similarity 使用
	}, nil
}