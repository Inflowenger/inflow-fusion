package InfraSpaces

import (
	"encoding/base64"

	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
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



