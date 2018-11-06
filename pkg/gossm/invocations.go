package gossm

import (
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

type Invocations map[string]*ssm.CommandInvocation

func (i Invocations) AllComplete() bool {
	allDone := true
	for _, inv := range i {
		allDone = allDone && *inv.Status != ssm.CommandInvocationStatusInProgress
	}
	return allDone
}

func (i Invocations) CompletedSince(prev Invocations) Invocations {
	ret := Invocations{}

	for id, val := range i {
		val := val
		prevVal, found := prev[id]
		if *val.Status != ssm.CommandStatusInProgress && (!found || *prevVal.Status != *val.Status) {
			ret[id] = val
		}
	}

	return ret
}

func (i Invocations) AddFromSlice(slice []*ssm.CommandInvocation) {
	for _, inv := range slice {
		i[*inv.InstanceId] = inv
	}
}

func (i Invocations) AddFromSSM(ssmApi ssmiface.SSMAPI, commandId string) error {
	listInput := &ssm.ListCommandInvocationsInput{CommandId: &commandId}
	return ssmApi.ListCommandInvocationsPages(listInput, func(page *ssm.ListCommandInvocationsOutput, lastPage bool) bool {
		for _, inv := range page.CommandInvocations {
			i[*inv.InstanceId] = inv
		}
		return !lastPage
	})
}

func (i Invocations) InstanceIds() []string {
	var ids []string

	for key := range i {
		ids = append(ids, key)
	}

	return ids
}
