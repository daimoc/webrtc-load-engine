package livekit

import (
	"time"

	"webrtc-load-engine/src/platform/internal/jwt"
)

// tokenClockSkew backdates nbf so that a server clock slightly behind ours
// still accepts a fresh token.
const tokenClockSkew = 10 * time.Second

// videoGrant is the "video" claim of a LiveKit access token. canPublishData
// is written even when false: LiveKit reads a missing one as true.
type videoGrant struct {
	Room           string `json:"room"`
	RoomJoin       bool   `json:"roomJoin"`
	CanPublish     bool   `json:"canPublish"`
	CanSubscribe   bool   `json:"canSubscribe"`
	CanPublishData bool   `json:"canPublishData"`
}

type tokenClaims struct {
	Issuer    string     `json:"iss"`
	Subject   string     `json:"sub"`
	Name      string     `json:"name"`
	ID        string     `json:"jti"`
	NotBefore int64      `json:"nbf"`
	Expires   int64      `json:"exp"`
	Video     videoGrant `json:"video"`
}

// tokenFor returns the access token of bot peerID (research R3): the
// ready-made token, or one signed with the API key and secret whose
// identity is the peer ID.
func tokenFor(c Config, peerID string, now time.Time) (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}
	return jwt.Sign(tokenClaims{
		Issuer:    c.APIKey,
		Subject:   peerID,
		Name:      peerID,
		ID:        peerID,
		NotBefore: now.Add(-tokenClockSkew).Unix(),
		Expires:   now.Add(c.TokenTTL).Unix(),
		Video:     videoGrant{Room: c.Room, RoomJoin: true, CanPublish: true, CanSubscribe: true},
	}, []byte(c.APISecret))
}
