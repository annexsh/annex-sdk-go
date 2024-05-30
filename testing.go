package annex

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/annexhq/annex-sdk-go/internal/test"
	"github.com/annexhq/annex-sdk-go/internal/testing"
)

type TestT interface {
	require.TestingT
	Logger() testing.Logger
}

type CaseT interface {
	require.TestingT
	Context() context.Context
	Logger() testing.Logger
}

func SetResult(t CaseT, result any) {
	defer func() {
		if r := recover(); r != nil {
			if msg, ok := r.(string); ok && msg == "panic: send on closed channel" {
				panic("case result cannot be set more than once")
			}
			panic(r)
		}
	}()
	ch := test.ResultChanFromContext(t.Context())
	ch <- result
	close(ch)
}

func RequireSuccess(t TestT, pending *test.Pending) {
	err := test.GetPendingError(pending)
	require.NoError(t, err)
	ctx := getWorkflowT(t).WorkflowContext()
	err = test.GetPendingFuture(pending).Get(ctx, nil)
	require.NoError(t, err)
}

func RequireSuccessResult[R any](t TestT, pending *test.Pending) R {
	var res test.CaseResponse[R]
	err := test.GetPendingError(pending)
	require.NoError(t, err)
	ctx := getWorkflowT(t).WorkflowContext()
	err = test.GetPendingFuture(pending).Get(ctx, &res)
	require.NoError(t, err)
	return res.Result
}
