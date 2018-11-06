package gossm

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
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

func (i Invocations) InstanceIds(ec2Api ec2iface.EC2API) (*InstanceIds, error) {
	var allIds []string
	var faultyInstanceIds []string
	var liveInstanceIds []string
	var wrongInstanceIds []string

	for id, _ := range i {
		allIds = append(allIds, id)
	}

	var reservations []*ec2.Reservation
	descInput := &ec2.DescribeInstancesInput{InstanceIds: aws.StringSlice(allIds)}
	err := ec2Api.DescribeInstancesPages(descInput, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
		reservations = append(reservations, page.Reservations...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}

	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			invocation := i[*instance.InstanceId]
			docName := *invocation.DocumentName

			if instance.Platform != nil && *instance.Platform == "windows" && docName != "AWS-RunPowerShellScript" {
				wrongInstanceIds = append(wrongInstanceIds, *instance.InstanceId)
				continue
			} else if instance.Platform == nil /* linux */ && docName != "AWS-RunShellScript" {
				wrongInstanceIds = append(wrongInstanceIds, *instance.InstanceId)
				continue
			}

			if *instance.State.Name == ec2.InstanceStateNameRunning {
				liveInstanceIds = append(liveInstanceIds, *instance.InstanceId)
			}
		}
	}

	for _, instanceId := range allIds {
		if !stringInSlice(instanceId, liveInstanceIds) && !stringInSlice(instanceId, wrongInstanceIds) {
			faultyInstanceIds = append(faultyInstanceIds, instanceId)
		}
	}

	return &InstanceIds{
		InstanceIds:              allIds,
		FaultyInstanceIds:        faultyInstanceIds,
		WrongPlatformInstanceIds: wrongInstanceIds,
	}, nil
}

type InstanceIds struct {
	InstanceIds              []string
	FaultyInstanceIds        []string
	WrongPlatformInstanceIds []string
}
