package gossm

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
)

func MakeTargets(tagPairs, instanceIds []string) []*ssm.Target {
	targets := []*ssm.Target{}

	for _, pair := range tagPairs {
		splitted := strings.SplitN(pair, "=", 2)

		tag := splitted[0]
		val := splitted[1]
		key := fmt.Sprintf("tag:%s", tag)

		target := &ssm.Target{
			Key:    &key,
			Values: []*string{&val},
		}
		targets = append(targets, target)
	}

	if len(instanceIds) > 0 {
		target := &ssm.Target{
			Key:    aws.String("InstanceIds"),
			Values: aws.StringSlice(instanceIds),
		}
		targets = append(targets, target)
	}

	return targets
}
