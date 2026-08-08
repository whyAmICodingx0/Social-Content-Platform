package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	nonAlnum   = regexp.MustCompile(`[^a-z0-9]+`)
	dashRuns   = regexp.MustCompile(`-{2,}`)
	maxSlugLen = 80
)

func makeBaseSlug(title string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = dashRuns.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > maxSlugLen {
		s = s[:maxSlugLen]
		s = strings.Trim(s, "-")
	}
	if s == "" {
		return randomSlug()
	}
	return s, nil
}

func candidateSlug(base string, attempt int) (string, error) {
	switch {
	case attempt == 0:
		return base, nil
	case attempt < slugMaxAttempts:
		return fmt.Sprintf("%s-%d", base, attempt+1), nil
	default:
		return randomSlug()
	}
}

func randomSlug() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "post-" + hex.EncodeToString(b), nil
}

const slugMaxAttempts = 5
