package awssess

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/glassechidna/awscredcache"
	"github.com/pquerna/otp/totp"
	"time"
)

func AwsSession(profile, region string) *session.Session {
	provider := awscredcache.NewAwsCacheCredProvider(profile)
	provider.MfaCodeProvider = func(mfaSecret string) (string, error) {
		if len(mfaSecret) == 0 {
			return stscreds.StdinTokenProvider()
		} else {
			return totp.GenerateCode(mfaSecret, time.Now())
		}
	}

	chain := provider.WrapInChain()
	creds := credentials.NewCredentials(chain)

	sessOpts := session.Options{
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		Config:                  aws.Config{Credentials: creds},
	}

	if len(profile) > 0 {
		sessOpts.Profile = profile
	}

	sess, _ := session.NewSessionWithOptions(sessOpts)
	//sess.Config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)

	if len(region) > 0 {
		sess.Config.Region = &region
	}

	return sess
}
