package actions

import (
	"github.com/glassechidna/gossm/pkg/awssess"
	"github.com/glassechidna/gossm/pkg/gossm"
	"sync"
)

type GossmSession struct {
	history *gossm.History
	client  *gossm.Client
}
var sessOnce *sync.Once
var gossmSession *GossmSession

func sess() *GossmSession {
	sessOnce.Do(func() {
		h, _ := gossm.NewHistory("foo.db")
		sess := awssess.AwsSession("", "")
		gossmSession = &GossmSession{
			history: h,
			client: gossm.New(sess, h),
		}
	})
	return gossmSession
}

