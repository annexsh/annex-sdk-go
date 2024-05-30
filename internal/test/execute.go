package test

import (
	"errors"
	"fmt"
	"time"

	testv1 "github.com/annexhq/annex-proto/gen/go/type/test/v1"
	"github.com/annexhq/annex/test"
	"go.temporal.io/sdk/workflow"

	"github.com/annexhq/annex-sdk-go/internal/temporal"
)

type TestExecutor func(ctx workflow.Context, payload *testv1.Payload) error

func ExecuteTest(ctx workflow.Context, wf func(ctx workflow.Context)) error {
	weInfo := workflow.GetInfo(ctx)
	testExecID, err := test.ParseTestWorkflowID(weInfo.WorkflowExecution.ID)
	if err != nil {
		return err
	}

	ctx = temporal.WorkflowContextWithTestLogConfig(ctx, temporal.TestLogConfig{
		TestExecID: testExecID,
	})

	startCh := workflow.GetSignalChannel(ctx, testv1.TestSignal_TEST_SIGNAL_START_TEST.String())
	if ok, _ := startCh.ReceiveWithTimeout(ctx, 30*time.Second, nil); !ok {
		return errors.New("did not receive a start test signal before timeout")
	}
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
