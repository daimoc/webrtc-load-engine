package livekit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTokenFor_Signed(t *testing.T) {
	cfg := Config{Room: "load008", APIKey: "devkey", APISecret: "s3cret", TokenTTL: 2 * time.Hour}
	now := time.Unix(1760000000, 0)
	tok, err := tokenFor(cfg, "peer-1", now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token %q has %d parts", tok, len(parts))
	}
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if got := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); got != parts[2] {
		t.Error("signature does not verify with the secret")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	if err := json.Unmarshal(payload, &c); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"iss": "devkey", "sub": "peer-1", "name": "peer-1", "jti": "peer-1",
		"nbf": float64(now.Add(-10 * time.Second).Unix()), "exp": float64(now.Add(2 * time.Hour).Unix()),
	}
	for k, v := range want {
		if c[k] != v {
			t.Errorf("claim %s = %v, want %v", k, c[k], v)
		}
	}
	video, _ := c["video"].(map[string]any)
	wantVideo := map[string]any{"room": "load008", "roomJoin": true, "canPublish": true, "canSubscribe": true, "canPublishData": false}
	for k, v := range wantVideo {
		got, present := video[k]
		if !present || got != v {
			t.Errorf("video.%s = %v (present %v), want %v", k, got, present, v)
		}
	}
}

func TestTokenFor_ReadyMade(t *testing.T) {
	tok, err := tokenFor(Config{Token: "ready.made.token"}, "peer-1", time.Now())
	if err != nil || tok != "ready.made.token" {
		t.Errorf("tokenFor = %q, %v", tok, err)
	}
}
