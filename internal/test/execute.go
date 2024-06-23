package test

import (
	"errors"
	"fmt"

	"github.com/annexsh/annex-proto/gen/go/annex/tests/v1"
	"github.com/annexsh/annex/test"
	"go.temporal.io/sdk/workflow"

	"github.com/annexsh/annex-sdk-go/internal/temporal"
)

type TestExecutor func(ctx workflow.Context, payload *testsv1.Payload) error

func ExecuteTest(ctx workflow.Context, wf func(ctx workflow.Context)) error {
	weInfo := workflow.GetInfo(ctx)
	testExecID, err := test.ParseTestWorkflowID(weInfo.WorkflowExecution.ID)
	if err != nil {
		return err
	}

	ctx = temporal.WorkflowContextWithTestLogConfig(ctx, temporal.TestLogConfig{
		TestExecID: testExecID,
	})

	return execWithRecover(func() {
		wf(ctx)
	})
}

func execWithRecover(wrapper func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(fmt.Sprintf("%+v", r))
		}
	}()
	wrapper()
	return err
}
