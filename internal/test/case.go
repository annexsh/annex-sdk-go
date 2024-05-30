package test

import (
	"context"
	"fmt"
	"time"

	"github.com/annexhq/annex/test"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/activity"

	"github.com/annexhq/annex-sdk-go/internal/temporal"
)

type resultKey struct{}

type CaseExecutor func(ctx context.Context, payload *common.Payload) (any, error)

type CaseResponse[T any] struct {
	Result   T
	Finished time.Time
	Duration time.Duration
}

func ExecuteCase(ctx context.Context, a func(ctx context.Context)) (*CaseResponse[any], error) {
	info := activity.GetInfo(ctx)
	weInfo := info.WorkflowExecution

	testExecID, err := test.ParseTestWorkflowID(weInfo.ID)
	if err != nil {
		return nil, err
	}
	caseExecID, err := test.ParseCaseActivityID(info.ActivityID)
	if err != nil {
		return nil, err
	}

	ctx = temporal.ContextWithTestLogConfig(ctx, temporal.TestLogConfig{
		TestExecID: testExecID,
		CaseExecID: &caseExecID,
	})

	resultCh := make(chan any, 1)
	ctx = context.WithValue(ctx, resultKey{}, resultCh)

	start := time.Now()
	res := &CaseResponse[any]{}

	if err := execWithRecover(func() {
		a(ctx)
	}); err != nil {
		return nil, fmt.Errorf("case execution failed: %w", err)
	}

	res.Finished = time.Now()
	res.Duration = res.Finished.Sub(start)

	select {
	case result := <-resultCh:
		res.Result = result
	default:
	}
	return res, nil
}

func ResultChanFromContext(ctx context.Context) chan<- any {
	val := ctx.Value(resultKey{})
	ch, ok := val.(chan any)
	if !ok {
		panic("context value is not a result channel")
	}
	return ch
}
