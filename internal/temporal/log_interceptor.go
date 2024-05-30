package temporal

import (
	"context"

	"go.temporal.io/sdk/interceptor"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

type workerTestLogInterceptor struct {
	interceptor.WorkerInterceptorBase
	publisher LogPublisher
}

func NewWorkerLogInterceptor(publisher LogPublisher) interceptor.WorkerInterceptor {
	return &workerTestLogInterceptor{
		publisher: publisher,
	}
}

func (i *workerTestLogInterceptor) InterceptActivity(_ context.Context, next interceptor.ActivityInboundInterceptor) interceptor.ActivityInboundInterceptor {
	in := &activityInboundLogInterceptor{
		root:      i,
		publisher: i.publisher,
	}
	in.Next = next
	return in
}

func (i *workerTestLogInterceptor) InterceptWorkflow(_ workflow.Context, next interceptor.WorkflowInboundInterceptor) interceptor.WorkflowInboundInterceptor {
	in := &workflowInboundLogInterceptor{root: i}
	in.Next = next
	return in
}

type activityInboundLogInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
	root      *workerTestLogInterceptor
	publisher LogPublisher
}

func (i *activityInboundLogInterceptor) Init(outbound interceptor.ActivityOutboundInterceptor) error {
	in := &activityOutboundInterceptor{
		root:      i.root,
		publisher: i.publisher,
	}
	in.Next = outbound
	return i.Next.Init(in)
}

type activityOutboundInterceptor struct {
	interceptor.ActivityOutboundInterceptorBase
	root      *workerTestLogInterceptor
	publisher LogPublisher
}

func (i *activityOutboundInterceptor) GetLogger(ctx context.Context) tlog.Logger {
	cfg, ok := TestLogConfigFromContext(ctx)
	if ok {
		if cfg.CaseExecID == nil {
			return NewTestActivityLogger(i.publisher, cfg.TestExecID)
		}
		return NewTestActivityLogger(i.publisher, cfg.TestExecID, WithCaseExecID(*cfg.CaseExecID))
	}
	return i.Next.GetLogger(ctx)
}

type workflowInboundLogInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
	root *workerTestLogInterceptor
}

func (i *workflowInboundLogInterceptor) Init(outbound interceptor.WorkflowOutboundInterceptor) error {
	in := &workflowOutboundLogInterceptor{root: i.root}
	in.Next = outbound
	return i.Next.Init(in)
}

type workflowOutboundLogInterceptor struct {
	interceptor.WorkflowOutboundInterceptorBase
	root *workerTestLogInterceptor
}

func (i *workflowOutboundLogInterceptor) GetLogger(ctx workflow.Context) tlog.Logger {
	cfg, ok := TestLogConfigFromWorkflowContext(ctx)
	if ok {
		return NewTestWorkflowLogger(ctx, cfg.TestExecID)
	}
	return i.Next.GetLogger(ctx)
}
