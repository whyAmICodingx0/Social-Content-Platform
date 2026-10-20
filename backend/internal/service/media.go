package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cloudinary"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/store"
)

const (
	dailyUploadQuota = 50 // 決策 #137
	destroyTimeout   = 5 * time.Second
)

var (
	ErrQuotaExceeded = errors.New("service: upload quota exceeded")
	// ErrUploadNotFound 涵蓋「不存在」與「不屬於本人」——刻意不區分，不洩漏他人資源存在
	ErrUploadNotFound      = errors.New("service: upload not found")
	ErrProviderUnavailable = errors.New("service: media provider unavailable")
)

// UploadRejectedError：檔案存在且屬於本人，但違反政策（大小、格式、長度）。
// Reason 會回給前端，所以只放可公開的描述。
type UploadRejectedError struct{ Reason string }

func (e *UploadRejectedError) Error() string { return "service: upload rejected: " + e.Reason }

// purposeSpec：每個 purpose 的完整政策。
//
// 決策 #129：資料庫不存 kind，resourceType 由這張表推導。
type purposeSpec struct {
	tag                 string // public_id 前綴（決策 #135）
	preset              string
	resourceType        string
	maxBytes            int64 // 檢查的是實際存下的大小（決策 #139）
	formats             map[string]bool
	transformation      string // 決策 #119：固定變體
	ext                 string
	thumbTransformation string
	maxDuration         float64 // 0 = 不檢查
}

var (
	imageFormats = map[string]bool{"jpg": true, "jpeg": true, "png": true, "webp": true}
	videoFormats = map[string]bool{"mp4": true, "webm": true, "mov": true}
)

var purposeSpecs = map[string]purposeSpec{
	"avatar": {
		tag: "av", preset: "inkwell_avatar", resourceType: "image",
		maxBytes: 2 << 20, formats: imageFormats,
		transformation: "c_fill,g_face,w_512,h_512,f_auto,q_auto",
	},
	"post_image": {
		tag: "pi", preset: "inkwell_post_image", resourceType: "image",
		maxBytes: 5 << 20, formats: imageFormats,
		transformation: "c_limit,w_1600,f_auto,q_auto",
	},
	"post_video": {
		tag: "pv", preset: "inkwell_post_video", resourceType: "video",
		maxBytes: 30 << 20, formats: videoFormats,
		transformation: "f_mp4,q_auto", ext: "mp4",
		thumbTransformation: "so_1,c_limit,w_800,f_jpg,q_auto",
		maxDuration:         60,
	},
}

var (
	assetIDPattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
	publicIDPattern = regexp.MustCompile(`^(av|pi|pv)_[A-Za-z0-9_-]{22}$`)
)

type MediaService struct {
	Media      *repository.MediaRepository
	Cloudinary *cloudinary.Client
	Quota      *store.UploadQuotaStore
}

// ---------- 簽章 ----------

type UploadTicket struct {
	UploadURL    string
	ResourceType string
	MaxBytes     int64
	// Fields：前端必須原封不動全部送給 Cloudinary（M-0 §0.13）
	Fields map[string]string
}

func assetFolderFor(userID string) string { return "inkwell/" + userID }

// newPublicID 產生 "{tag}_" + 22 字元 base64url（128 bit CSPRNG）。
func newPublicID(tag string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return tag + "_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// IssueTicket 發出一張「只能用一次」的上傳簽章（決策 #135）。
func (s *MediaService) IssueTicket(ctx context.Context, userID, purpose string) (*UploadTicket, error) {
	spec, ok := purposeSpecs[purpose]
	if !ok {
		return nil, &ValidationError{Field: "purpose", Message: "must be avatar, post_image or post_video"}
	}
	if !s.Cloudinary.Configured() {
		return nil, ErrProviderUnavailable
	}

	// 配額（決策 #137）。Redis 不可用時放行並記 log ——
	// 配額是保護措施而非安全邊界。
	n, err := s.Quota.Incr(ctx, userID, time.Now())
	switch {
	case err != nil:
		log.Printf("media: upload quota unavailable, allowing (user=%s): %v", userID, err)
	case n > dailyUploadQuota:
		return nil, ErrQuotaExceeded
	}

	publicID, err := newPublicID(spec.tag)
	if err != nil {
		return nil, err
	}

	// ⚠️ 簽進去的每一個參數，前端都必須原封不動送出。
	// - asset_folder：dynamic folder mode 的正確參數（決策 #134），絕不用 folder
	// - public_id + overwrite=false：讓這個簽章只能產生一個檔案（決策 #135）
	params := map[string]string{
		"asset_folder":  assetFolderFor(userID),
		"overwrite":     "false",
		"public_id":     publicID,
		"timestamp":     strconv.FormatInt(time.Now().Unix(), 10),
		"upload_preset": spec.preset,
	}

	fields := make(map[string]string, len(params)+2)
	for k, v := range params {
		fields[k] = v
	}
	fields["api_key"] = s.Cloudinary.APIKey
	fields["signature"] = s.Cloudinary.Sign(params)
	// api_secret 只用於計算簽章，絕不放進回應

	return &UploadTicket{
		UploadURL:    s.Cloudinary.UploadURL(spec.resourceType),
		ResourceType: spec.resourceType,
		MaxBytes:     spec.maxBytes,
		Fields:       fields,
	}, nil
}

// ---------- 回報 ----------

type ReportInput struct {
	Purpose  string
	AssetID  string
	PublicID string
}

// Report 驗證上傳結果並建立 media 列。
// 回傳 (media, 是否為既有, error)：既有 → handler 回 200，新建 → 201。
func (s *MediaService) Report(ctx context.Context, userID string, in ReportInput) (*repository.Media, bool, error) {
	spec, ok := purposeSpecs[in.Purpose]
	if !ok {
		return nil, false, &ValidationError{Field: "purpose", Message: "must be avatar, post_image or post_video"}
	}
	if !assetIDPattern.MatchString(in.AssetID) {
		return nil, false, &ValidationError{Field: "asset_id", Message: "invalid format"}
	}
	// 格式不符就不呼叫 Admin API（省額度，也不讓任意字串進 URL path）
	if !publicIDPattern.MatchString(in.PublicID) || !strings.HasPrefix(in.PublicID, spec.tag+"_") {
		return nil, false, &ValidationError{Field: "public_id", Message: "invalid format or does not match purpose"}
	}

	// 冪等快路徑（決策 #143）：已建立過就不再呼叫 Admin API
	existing, err := s.Media.GetByAssetID(ctx, in.AssetID)
	switch {
	case err == nil:
		if existing.OwnerID != userID {
			return nil, false, ErrUploadNotFound // 不洩漏他人資源存在
		}
		return existing, true, nil
	case !errors.Is(err, repository.ErrNotFound):
		return nil, false, err
	}

	if !s.Cloudinary.Configured() {
		return nil, false, ErrProviderUnavailable
	}

	res, err := s.Cloudinary.GetResource(ctx, spec.resourceType, in.PublicID)
	if errors.Is(err, cloudinary.ErrNotFound) {
		return nil, false, ErrUploadNotFound
	}
	if err != nil {
		log.Printf("media: admin api lookup failed (public_id=%s): %v", in.PublicID, err)
		return nil, false, ErrProviderUnavailable
	}

	// 所有權（決策 #134）。不屬於本人 → 一律當作找不到，且**絕不刪除**。
	if !ownedBy(res, userID, in) {
		return nil, false, ErrUploadNotFound
	}

	// 政策檢查。走到這裡已確認是本人的檔案，違規就刪掉（決策 #140）。
	if rej := checkResource(spec, res); rej != nil {
		s.destroyRejected(ctx, spec.resourceType, in.PublicID)
		return nil, false, rej
	}

	m := &repository.Media{
		OwnerID:          userID,
		ProviderAssetID:  res.AssetID,
		ProviderPublicID: res.PublicID,
		Purpose:          in.Purpose,
		Status:           "ready", // 決策 #118：P5 同步反查成功即 ready
		URL:              s.Cloudinary.DeliveryURL(spec.resourceType, spec.transformation, res.PublicID, spec.ext),
		Format:           strings.ToLower(res.Format),
		Bytes:            res.Bytes,
		Width:            positiveOrNil(res.Width),
		Height:           positiveOrNil(res.Height),
	}
	if spec.resourceType == "video" {
		thumb := s.Cloudinary.DeliveryURL("video", spec.thumbTransformation, res.PublicID, "jpg")
		m.ThumbnailURL = &thumb
		m.Duration = res.Duration // checkResource 已保證非 nil
	}

	created, err := s.Media.Create(ctx, m)
	if errors.Is(err, repository.ErrMediaDuplicate) {
		// 兩個並行的回報撞在一起（決策 #117）：收斂到既有那一筆
		again, ferr := s.Media.GetByAssetID(ctx, in.AssetID)
		if ferr != nil {
			return nil, false, ferr
		}
		if again.OwnerID != userID {
			return nil, false, ErrUploadNotFound
		}
		return again, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return created, false, nil
}

// ownedBy：Admin API 回傳的資源是否確實是本人這次上傳的那一個。
// 純函式，可單獨測試。
func ownedBy(res *cloudinary.Resource, userID string, in ReportInput) bool {
	return res.AssetFolder == assetFolderFor(userID) &&
		res.AssetID == in.AssetID &&
		res.PublicID == in.PublicID
}

// checkResource：政策檢查。純函式，可單獨測試。
func checkResource(spec purposeSpec, res *cloudinary.Resource) *UploadRejectedError {
	if res.ResourceType != spec.resourceType {
		return &UploadRejectedError{Reason: "unexpected resource type"}
	}
	if !spec.formats[strings.ToLower(res.Format)] {
		return &UploadRejectedError{Reason: "file format not allowed"}
	}
	if res.Bytes <= 0 {
		return &UploadRejectedError{Reason: "invalid file"}
	}
	if res.Bytes > spec.maxBytes {
		return &UploadRejectedError{Reason: "file too large"}
	}
	if spec.maxDuration > 0 {
		// 缺值一律拒絕（fail closed，決策 #138）——寧可明確失敗，不可放行未知長度
		if res.Duration == nil {
			return &UploadRejectedError{Reason: "video duration unavailable"}
		}
		if *res.Duration > spec.maxDuration {
			return &UploadRejectedError{Reason: "video longer than 60 seconds"}
		}
	}
	return nil
}

// destroyRejected 刪除違規的上傳（決策 #140）。
// 同決策 #89：同步呼叫、WithoutCancel + timeout、失敗只記 log。
func (s *MediaService) destroyRejected(ctx context.Context, resourceType, publicID string) {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), destroyTimeout)
	defer cancel()
	if err := s.Cloudinary.Destroy(dctx, resourceType, publicID); err != nil {
		log.Printf("media: destroy rejected upload failed (public_id=%s): %v", publicID, err)
	}
}

func positiveOrNil(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}
