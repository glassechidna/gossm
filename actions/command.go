package actions

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/gobuffalo/buffalo"
	"github.com/gorilla/websocket"
	"sync"
)

type commandRequest struct {
	ShellType   string
	InstanceIds []string
	Tags        map[string]string
	Command     string
}

type commandResponse struct {
	CommandId   string
	InstanceIds []string
}

func reqToAwsInput(req *commandRequest) *ssm.SendCommandInput {
	docName := "AWS-RunShellScript"
	if req.ShellType == "powershell" {
		docName = "AWS-RunPowerShellScript"
	}

	targets := []*ssm.Target{}

	if len(req.InstanceIds) > 0 {
		targets = append(targets, &ssm.Target{
			Key:    aws.String("InstanceIds"),
			Values: aws.StringSlice(req.InstanceIds),
		})
	}

	for name, val := range req.Tags {
		val := val
		key := fmt.Sprintf("tag:%s", name)

		target := &ssm.Target{
			Key:    &key,
			Values: []*string{&val},
		}
		targets = append(targets, target)
	}

	return &ssm.SendCommandInput{
		DocumentName:   &docName,
		Targets:        targets,
		TimeoutSeconds: aws.Int64(300),
		CloudWatchOutputConfig: &ssm.CloudWatchOutputConfig{
			CloudWatchOutputEnabled: aws.Bool(true),
		},
		Parameters: map[string][]*string{
			"commands": aws.StringSlice([]string{req.Command}),
		},
	}
}

var theClient *gossm.Client
var clientOnce *sync.Once

func client() *gossm.Client {
	clientOnce.Do(func() {
		theClient = gossm.New(nil)
	})
	return theClient
}

func CommandHandler(c buffalo.Context) error {
	req := commandRequest{}
	if err := c.Bind(&req); err != nil {
		return err
	}

	input := reqToAwsInput(&req)
	resp, err := client().Doit(context.Background(), input)
	if err != nil {
		panic(err)
	}

	return c.Render(200, r.JSON(commandResponse{
		CommandId:   resp.CommandId,
		InstanceIds: resp.InstanceIds.InstanceIds,
	}))

	//conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	//if err != nil {
	//	return err
	//}
	//
	//s := dockerdev.DefaultStorage
	//trips, err := s.GetTrips()
	//if err != nil {
	//	return err
	//}
	//
	//go func() {
	//	ch, stop := s.NotifyTrips()
	//
	//	for _, trip := range trips {
	//		conn.WriteJSON(requestsResponse{Requests: []requestsRequest{
	//			tripToRR(trip),
	//		}})
	//	}
	//
	//	for {
	//		select {
	//		case trip := <-ch:
	//			err := conn.WriteJSON(requestsResponse{Requests: []requestsRequest{
	//				tripToRR(trip),
	//			}})
	//			if err != nil { // probably connection closed by browser
	//				stop()
	//				return
	//			}
	//		}
	//	}
	//}()
	//
	//return nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
