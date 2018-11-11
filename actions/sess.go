package actions

import (
	"github.com/glassechidna/gossm/pkg/awssess"
	"github.com/glassechidna/gossm/pkg/gossm"
	"sync"
)

type GossmSession struct {
	client  *gossm.Client
}

var sessOnce *sync.Once
var gossmSession *GossmSession

func sess() *GossmSession {
	sessOnce.Do(func() {
		sess := awssess.AwsSession("", "")
		gossmSession = &GossmSession{
			client:  gossm.New(sess, gossm.DefaultHistory),
		}
	})
	return gossmSession
}
