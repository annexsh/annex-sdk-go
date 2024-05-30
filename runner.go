package annex

import (
	"context"
	"encoding/json"
	"reflect"

	testservicev1 "github.com/annexhq/annex-proto/gen/go/rpc/testservice/v1"
	testv1 "github.com/annexhq/annex-proto/gen/go/type/test/v1"
	"github.com/denisbrodbeck/machineid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/annexhq/annex-sdk-go/internal/temporal"
)

const taskQueue = "default" // TODO: configurable

type Runner struct {
	id              string
	client          client.Client
	base            worker.Worker
	annex           testservicev1.TestServiceClient
	ctx             context.Context
	registeredTests []registeredTest
}

func NewRunner() (*Runner, error) {
	ctx := context.Background()

	annexAddr := "127.0.0.1:50051" // TODO: configurable

	conn, err := grpc.DialContext(ctx, annexAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	annexClient := testservicev1.NewTestServiceClient(conn)

	namespace := "default"

	temporalClient, err := temporal.NewClient(ctx, annexAddr, namespace)
	if err != nil {
		return nil, err
	}

	id, err := machineid.ProtectedID("annex-runner")
	if err != nil {
		return nil, err
	}

	base := worker.New(temporalClient, taskQueue, worker.Options{
		DisableRegistrationAliasing: true,
		Interceptors: []interceptor.WorkerInterceptor{
			temporal.NewWorkerLogInterceptor(annexClient),
		},
		Identity: id,
	})

	base.RegisterActivity(temporal.NewTestLogActivity(annexClient))

	return &Runner{
		id:     id,
		client: temporalClient,
		base:   base,
		annex:  annexClient,
		ctx:    ctx,
	}, nil
}

func RegisterTest(runner *Runner, name string, test func(t TestT)) {
	runner.registeredTests = append(runner.registeredTests, registeredTest{
		name: name,
		test: &simpleTest{test: test},
	})
}

func RegisterInputTest[P any](runner *Runner, name string, test func(ctx TestT, param P)) {
	runner.registeredTests = append(runner.registeredTests, registeredTest{
		name:         name,
		test:         &paramTest[P]{test: test},
		defaultParam: *new(P),
	})
}

func RegisterCase(runner *Runner, caseFn func(t CaseT)) {
	c := simpleCase{caseFn: caseFn}
	runner.base.RegisterActivityWithOptions(c.activity, activity.RegisterOptions{
		Name: c.name(),
	})
}

func RegisterInputCase[P any](runner *Runner, caseFn func(t CaseT, param P)) {
	c := paramCase[P]{caseFn: caseFn}
	runner.base.RegisterActivityWithOptions(c.activity, activity.RegisterOptions{
		Name: c.name(),
	})
}

func (w *Runner) Run() error {
	var defs []*testv1.TestDefinition

	for _, reg := range w.registeredTests {
		w.base.RegisterWorkflowWithOptions(reg.test.workflow, workflow.RegisterOptions{
			Name: reg.name,
		})

		def := &testv1.TestDefinition{
			Project:        taskQueue,
			Name:           reg.name,
			DefaultPayload: nil,
		}

		if reg.defaultParam != nil {
			data, err := json.Marshal(reg.defaultParam)
			if err != nil {
				return err
			}
			def.DefaultPayload = &testv1.Payload{
				Data:   data,
				IsZero: true,
			}
		}

		defs = append(defs, def)
	}

	regRes, err := w.annex.RegisterTests(w.ctx, &testservicev1.RegisterTestsRequest{
		RunnerId:    w.id,
		Definitions: defs,
	})
	if err != nil {
		return err
	}

	testIDs := make([]string, len(regRes.Registered))
	for i, reg := range regRes.Registered {
		testIDs[i] = reg.Id
	}

	err = w.base.Run(worker.InterruptCh())
	return err
}

type tester interface {
	workflow(ctx workflow.Context, payload *testv1.Payload) error
	paramType() (bool, reflect.Type)
}

type registeredTest struct {
	name         string
	test         tester
	defaultParam any
}
