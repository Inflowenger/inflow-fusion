package nats

import (
	"strings"

	"github.com/nats-io/jwt/v2"
)

const (
	NATS_DEFAULT_INBOX = "_INBOX"
)

func GetInboxPrefix(userClaim *jwt.UserClaims) string {
	if tags := userClaim.GetTags(); len(tags) > 0 {
		for _, v := range tags {
			if strings.HasPrefix(v, NATS_DEFAULT_INBOX) {
				return v
			}
		}
	}
	return NATS_DEFAULT_INBOX
}
