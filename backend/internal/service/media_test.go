package service

import (
	"testing"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cloudinary"
)

func f64(v float64) *float64 { return &v }

func TestCheckResource(t *testing.T) {
	img := purposeSpecs["post_image"]
	vid := purposeSpecs["post_video"]

	cases := []struct {
		name   string
		spec   purposeSpec
		res    cloudinary.Resource
		reject bool
	}{
		{"image ok", img, cloudinary.Resource{ResourceType: "image", Format: "jpg", Bytes: 1 << 20}, false},
		{"image exactly at limit", img, cloudinary.Resource{ResourceType: "image", Format: "png", Bytes: 5 << 20}, false},
		{"image one byte over", img, cloudinary.Resource{ResourceType: "image", Format: "png", Bytes: 5<<20 + 1}, true},
		{"svg rejected", img, cloudinary.Resource{ResourceType: "image", Format: "svg", Bytes: 1000}, true},
		{"gif rejected", img, cloudinary.Resource{ResourceType: "image", Format: "gif", Bytes: 1000}, true},
		{"uppercase format ok", img, cloudinary.Resource{ResourceType: "image", Format: "WEBP", Bytes: 1000}, false},
		{"zero bytes", img, cloudinary.Resource{ResourceType: "image", Format: "jpg", Bytes: 0}, true},
		{"wrong resource type", img, cloudinary.Resource{ResourceType: "video", Format: "mp4", Bytes: 1000}, true},
		{"video ok", vid, cloudinary.Resource{ResourceType: "video", Format: "mp4", Bytes: 1 << 20, Duration: f64(42)}, false},
		{"video exactly 60s", vid, cloudinary.Resource{ResourceType: "video", Format: "mov", Bytes: 1 << 20, Duration: f64(60)}, false},
		{"video 61s", vid, cloudinary.Resource{ResourceType: "video", Format: "mp4", Bytes: 1 << 20, Duration: f64(61)}, true},
		{"video missing duration fails closed", vid, cloudinary.Resource{ResourceType: "video", Format: "mp4", Bytes: 1 << 20}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.res
			got := checkResource(tc.spec, &res)
			if (got != nil) != tc.reject {
				t.Fatalf("reject = %v, want %v (err=%v)", got != nil, tc.reject, got)
			}
		})
	}
}

func TestOwnedBy(t *testing.T) {
	in := ReportInput{Purpose: "post_image", AssetID: "a", PublicID: "pi_x"}
	ok := cloudinary.Resource{AssetFolder: "inkwell/u1", AssetID: "a", PublicID: "pi_x"}

	if !ownedBy(&ok, "u1", in) {
		t.Fatal("own upload should be owned")
	}

	other := ok
	other.AssetFolder = "inkwell/u2"
	if ownedBy(&other, "u1", in) {
		t.Fatal("another user's folder must not be owned")
	}

	wrongAsset := ok
	wrongAsset.AssetID = "b"
	if ownedBy(&wrongAsset, "u1", in) {
		t.Fatal("asset id mismatch must not be owned")
	}

	// 前綴陷阱：inkwell/u1 不可匹配 inkwell/u10 —— 這是用「相等」而非 HasPrefix 的理由
	prefixTrap := ok
	prefixTrap.AssetFolder = "inkwell/u10"
	if ownedBy(&prefixTrap, "u1", in) {
		t.Fatal("folder must match exactly, not by prefix")
	}
}

func TestNewPublicID(t *testing.T) {
	id, err := newPublicID("pi")
	if err != nil {
		t.Fatal(err)
	}
	if !publicIDPattern.MatchString(id) {
		t.Fatalf("generated id %q does not match publicIDPattern", id)
	}
}
