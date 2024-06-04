package annex

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/converter"
	temporalsdk "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/annexsh/annex-sdk-go/internal/name"
	"github.com/annexsh/annex-sdk-go/internal/test"
)

type CaseFunc func(ctx context.Context)

type InputCaseFunc[P any] func(t CaseT, param P)

type CaseConfig struct {
	Case  any
	Input any
}

type startCaseOptions struct {
	input any
}

type StartCaseOption func(opts *startCaseOptions)

func WithInput(input any) StartCaseOption {
	return func(opts *startCaseOptions) {
		opts.input = input
	}
}

func StartCase(t TestT, caseFunc any, opts ...StartCaseOption) *test.Pending {
	var options startCaseOptions
	for _, opt := range opts {
		opt(&options)
	}

	wt := getWorkflowT(t)
	ctx := wt.WorkflowContext()
	execID := wt.NextCaseExecutionID()

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ActivityID:             execID.ActivityID(),
		ScheduleToCloseTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
		RetryPolicy: &temporalsdk.RetryPolicy{
			MaximumAttempts: 1,
		},
	})

	activityName := name.FuncName(caseFunc)
	caseName := activityName
	nameSplit := strings.Split(activityName, ".")
	if len(nameSplit) > 1 {
		caseName = nameSplit[len(nameSplit)-1]
	}

	var workflowFuture workflow.Future
	if options.input == nil {
		workflowFuture = workflow.ExecuteActivity(ctx, activityName)
	} else {
		payload, err := converter.GetDefaultDataConverter().ToPayload(options.input)
		if err != nil {
			return test.NewPendingError(execID, activityName, fmt.Errorf("failed to marshal case param: %w", err))
		}

		workflowFuture = workflow.ExecuteActivity(ctx, activityName, payload)
	}

	return test.NewPending(execID, caseName, workflowFuture)
}
