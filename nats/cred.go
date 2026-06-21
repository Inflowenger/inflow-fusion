package nats

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

const (
	NATS_DEFAULT_INBOX = "_INBOX"
)

func CreateUserCredential(accountSeed string, user models.UserCredGenInput) (*models.Cred, error) {
	userKP, _ := nkeys.CreateUser()
	userPub, _ := userKP.PublicKey()

	userClaim := jwt.NewUserClaims(userPub)
	userClaim.Name = user.Name
	userClaim.Permissions.Pub = user.Pub
	userClaim.Permissions.Sub = user.Sub
	userClaim.Tags = user.Tags
	accKey, err := nkeys.FromSeed([]byte(accountSeed))
	if err != nil {
		return nil, err
	}
	userJWT, err := userClaim.Encode(accKey)
	if err != nil {
		return nil, err
	}
	userSeed, _ := userKP.Seed()
	creds, err := jwt.FormatUserConfig(userJWT, userSeed)
	if err != nil {
		return nil, err
	}
	return &models.Cred{Jwt: userJWT, USeed: string(userSeed), UPub: userPub, Base64Cred: base64.StdEncoding.EncodeToString(creds), Raw: string(creds)}, nil

}

func GetInboxConfigWithPluginId(pluginUid string) string {
	return fmt.Sprintf("_INBOX.%s", etc.UuidLastPart(pluginUid))
}

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
