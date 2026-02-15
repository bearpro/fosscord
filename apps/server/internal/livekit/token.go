package livekit

import (
	"errors"
	"strings"

	livekitauth "github.com/livekit/protocol/auth"
)

type TokenIssuer struct {
	apiKey    string
	apiSecret string
}

type VoiceTokenInput struct {
	RoomName string
	Identity string
	Name     string
	Metadata string
}

func NewTokenIssuer(apiKey, apiSecret string) TokenIssuer {
	return TokenIssuer{
		apiKey:    strings.TrimSpace(apiKey),
		apiSecret: strings.TrimSpace(apiSecret),
	}
}

func (i TokenIssuer) Enabled() bool {
	return i.apiKey != "" && i.apiSecret != ""
}

func (i TokenIssuer) IssueVoiceToken(input VoiceTokenInput) (string, error) {
	if !i.Enabled() {
		return "", errors.New("livekit credentials are not configured")
	}

	token := livekitauth.NewAccessToken(i.apiKey, i.apiSecret)
	token.SetIdentity(input.Identity)
	token.SetName(input.Name)
	token.SetMetadata(input.Metadata)
	token.SetVideoGrant(&livekitauth.VideoGrant{
		RoomJoin:       true,
		Room:           input.RoomName,
		CanPublish:     boolPointer(true),
		CanSubscribe:   boolPointer(true),
		CanPublishData: boolPointer(true),
	})

	return token.ToJWT()
}

func boolPointer(value bool) *bool {
	return &value
}
