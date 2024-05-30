package temporal

import (
	"context"

	"github.com/annexhq/annex/test"
	"go.temporal.io/sdk/workflow"
)

type testLogConfigKey struct{}

type TestLogConfig struct {
	TestExecID test.TestExecutionID
	CaseExecID *test.CaseExecutionID
}

func ContextWithTestLogConfig(ctx context.Context, config TestLogConfig) context.Context {
	return context.WithValue(ctx, testLogConfigKey{}, config)
}

func TestLogConfigFromContext(ctx context.Context) (TestLogConfig, bool) {
	return getContextVal[TestLogConfig](ctx, testLogConfigKey{})
}

func WorkflowContextWithTestLogConfig(ctx workflow.Context, config TestLogConfig) workflow.Context {
	return workflow.WithValue(ctx, testLogConfigKey{}, config)
}

func TestLogConfigFromWorkflowContext(ctx workflow.Context) (TestLogConfig, bool) {
	return getContextVal[TestLogConfig](ctx, testLogConfigKey{})
}

type valuer interface {
	Value(key any) any
}

func getContextVal[T any](ctx valuer, key any) (T, bool) {
	var ret T
	val := ctx.Value(key)
	if val == nil {
		return ret, false
	}
	ret, ok := val.(T)
	return ret, ok
}
