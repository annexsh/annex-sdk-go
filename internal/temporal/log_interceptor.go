package temporal

import (
	"context"

	"github.com/annexsh/annex/log"
	"go.temporal.io/sdk/interceptor"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

type workerTestLogInterceptor struct {
	interceptor.WorkerInterceptorBase
	logger    log.Logger
	publisher LogPublisher
}

func NewWorkerLogInterceptor(logger log.Logger, publisher LogPublisher) interceptor.WorkerInterceptor {
	return &workerTestLogInterceptor{
		logger:    logger,
		publisher: publisher,
	}
}

func (i *workerTestLogInterceptor) InterceptActivity(_ context.Context, next interceptor.ActivityInboundInterceptor) interceptor.ActivityInboundInterceptor {
	in := &activityInboundLogInterceptor{
		root:      i,
		logger:    i.logger,
		publisher: i.publisher,
	}
	in.Next = next
	return in
}

func (i *workerTestLogInterceptor) InterceptWorkflow(_ workflow.Context, next interceptor.WorkflowInboundInterceptor) interceptor.WorkflowInboundInterceptor {
	in := &workflowInboundLogInterceptor{root: i, logger: i.logger}
	in.Next = next
	return in
}

type activityInboundLogInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
	root      *workerTestLogInterceptor
	logger    log.Logger
	publisher LogPublisher
}

func (i *activityInboundLogInterceptor) Init(outbound interceptor.ActivityOutboundInterceptor) error {
	in := &activityOutboundInterceptor{
		root:      i.root,
		logger:    i.logger,
		publisher: i.publisher,
	}
	in.Next = outbound
	return i.Next.Init(in)
}

type activityOutboundInterceptor struct {
	interceptor.ActivityOutboundInterceptorBase
	root      *workerTestLogInterceptor
	logger    log.Logger
	publisher LogPublisher
}

func (i *activityOutboundInterceptor) GetLogger(ctx context.Context) tlog.Logger {
	cfg, ok := TestLogConfigFromContext(ctx)
	if ok {
		if cfg.CaseExecID == nil {
			return NewTestActivityLogger(i.logger, i.publisher, cfg.TestExecID)
		}
		return NewTestActivityLogger(i.logger, i.publisher, cfg.TestExecID, WithCaseExecID(*cfg.CaseExecID))
	}
	return i.Next.GetLogger(ctx)
}

type workflowInboundLogInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
	root   *workerTestLogInterceptor
	logger log.Logger
}

func (i *workflowInboundLogInterceptor) Init(outbound interceptor.WorkflowOutboundInterceptor) error {
	in := &workflowOutboundLogInterceptor{root: i.root, logger: i.logger}
	in.Next = outbound
	return i.Next.Init(in)
}

type workflowOutboundLogInterceptor struct {
	interceptor.WorkflowOutboundInterceptorBase
	root   *workerTestLogInterceptor
	logger log.Logger
}

func (i *workflowOutboundLogInterceptor) GetLogger(ctx workflow.Context) tlog.Logger {
	cfg, ok := TestLogConfigFromWorkflowContext(ctx)
	if ok {
		return NewTestWorkflowLogger(ctx, i.logger, cfg.TestExecID)
	}
	return i.Next.GetLogger(ctx)
}
