package service

import "strings"

const maxTagsPerPost = 5

func normalizeTags(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))

	for _, t := range raw {
		s := strings.ToLower(strings.TrimSpace(t))
		s = nonAlnum.ReplaceAllString(s, "-")
		s = dashRuns.ReplaceAllString(s, "-")
		s = strings.Trim(s, "-")
		if s == "" {
			continue
		}
		if len(s) > 50 {
			s = s[:50]
			s = strings.Trim(s, "-")
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	if len(out) > maxTagsPerPost {
		return nil, &ValidationError{
			Field: "tags", Message: "at most 5 tags allowed",
		}
	}
	return out, nil
}
