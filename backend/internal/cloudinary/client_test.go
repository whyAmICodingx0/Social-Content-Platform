package cloudinary

import "testing"

// 測試向量取自 Cloudinary 官方文件「Generating authentication signatures」。
// 若演算法寫錯（排序、串接、secret 位置），這條一定會失敗 —— 不會假通過。
func TestSign_OfficialExample(t *testing.T) {
	params := map[string]string{
		"eager":     "w_400,h_300,c_pad|w_260,h_200,c_crop",
		"public_id": "sample_image",
		"timestamp": "1315060510",
	}
	got := signWithSecret(params, "abcd")
	want := "bfd09f95f331f558cbd1320e67aa8d488770583e"
	if got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

func TestStringToSign_SortsKeys(t *testing.T) {
	got := StringToSign(map[string]string{
		"timestamp":     "1",
		"upload_preset": "p",
		"asset_folder":  "a",
		"public_id":     "x",
		"overwrite":     "false",
	})
	want := "asset_folder=a&overwrite=false&public_id=x&timestamp=1&upload_preset=p"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
