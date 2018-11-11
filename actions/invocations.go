package actions

import (
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/gobuffalo/buffalo"
)

type invocation struct {
	InstanceId string
	Status string
	Stdout string
	Stderr string
}

type invocationsResponse struct {
	Invocations map[string]invocation
}

func invocationsGet(c buffalo.Context) error {
	cmdId := c.Param("commandId")

	status, err := gossm.DefaultHistory.Command(cmdId)
	if err != nil {
		return err
	}

	ch := make(chan gossm.SsmMessage)
	outputs, _ := gossm.DefaultHistory.CommandOutputs(cmdId)
	go status.Stream(outputs, ch)

	invs := map[string]invocation{}

	for msg := range ch {
		if msg.Payload != nil {
			id := msg.Payload.InstanceId
			inv := invs[id]
			inv.InstanceId = id
			inv.Status = "InProgress"
			inv.Stdout += msg.Payload.StdoutChunk
			inv.Stderr += msg.Payload.StderrChunk
			invs[id] = inv
		}
	}

	return c.Render(200, r.JSON(invocationsResponse{Invocations: invs}))
}
