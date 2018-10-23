package gossm

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"log"
	"strings"
	"text/template"
)

func MakeTargets(tagPairs, instanceIds []string) []*ssm.Target {
	targets := []*ssm.Target{}

	for _, pair := range tagPairs {
		splitted := strings.SplitN(pair, "=", 2)

		tag := splitted[0]
		val := splitted[1]
		key := fmt.Sprintf("tag:%s", tag)

		target := &ssm.Target{
			Key: &key,
			Values: []*string{&val},
		}
		targets = append(targets, target)
	}

	if len(instanceIds) > 0 {
		target := &ssm.Target{
			Key: aws.String("InstanceIds"),
			Values: aws.StringSlice(instanceIds),
		}
		targets = append(targets, target)
	}

	return targets
}

func RealBucketName(sess *session.Session, input string) string {
	tmpl, err := template.New("bucket").Parse(input)
	if err != nil { log.Panicf(err.Error()) }

	region := *sess.Config.Region
	api := sts.New(sess)
	identity, err := api.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil { log.Panicf(err.Error()) }

	buf := bytes.Buffer{}
	err = tmpl.Execute(&buf, map[string]string{
		"Region":    region,
		"AccountId": *identity.Account,
	})
	if err != nil { log.Panicf(err.Error()) }

	return buf.String()
}
