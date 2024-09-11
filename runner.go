package annex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	"connectrpc.com/connect"
	testsv1 "github.com/annexsh/annex-proto/go/gen/annex/tests/v1"
	"github.com/annexsh/annex-proto/go/gen/annex/tests/v1/testsv1connect"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/annexsh/annex-sdk-go/internal/temporal"
)

const (
	runnerContext = "default"         // TODO: configurable
	runnerGroup   = "default"         // TODO: configurable
	annexHostPort = "127.0.0.1:50051" // TODO: configurable
	connectAPIURL = "http://" + annexHostPort + "/connect"
)

type Runner struct {
	id              string
	client          client.Client
	base            worker.Worker
	testClient      testsv1connect.TestServiceClient
	ctx             context.Context
	registeredTests []registeredTest
}

func NewRunner() (*Runner, error) {
	ctx := context.Background()

	testClient := testsv1connect.NewTestServiceClient(
		&http.Client{Timeout: time.Minute},
		connectAPIURL,
	)

	if _, err := testClient.RegisterGroup(ctx, connect.NewRequest(&testsv1.RegisterGroupRequest{
		Context: runnerContext,
		Name:    runnerGroup,
	})); err != nil {
		return nil, err
	}

	temporalClient, err := temporal.NewClient(ctx, annexHostPort, runnerContext)
	if err != nil {
		return nil, err
	}

	taskQueue := getTaskQueue(runnerContext, runnerGroup)
	id := getRunnerIdentify(taskQueue)

	base := worker.New(temporalClient, taskQueue, worker.Options{
		DisableRegistrationAliasing: true,
		Interceptors: []interceptor.WorkerInterceptor{
			temporal.NewWorkerLogInterceptor(testClient),
		},
		Identity: id,
	})

	base.RegisterActivity(temporal.NewTestLogActivity(testClient))

	return &Runner{
		id:         id,
		client:     temporalClient,
		base:       base,
		testClient: testClient,
		ctx:        ctx,
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
	var defs []*testsv1.TestDefinition

	for _, reg := range w.registeredTests {
		w.base.RegisterWorkflowWithOptions(reg.test.workflow, workflow.RegisterOptions{
			Name: reg.name,
		})

		def := &testsv1.TestDefinition{
			Name:         reg.name,
			DefaultInput: nil,
		}

		if reg.defaultParam != nil {
			data, err := json.Marshal(reg.defaultParam)
			if err != nil {
				return err
			}
			def.DefaultInput = &testsv1.Payload{
				Data: data,
			}
		}

		defs = append(defs, def)
	}

	regRes, err := w.testClient.RegisterTests(w.ctx, connect.NewRequest(&testsv1.RegisterTestsRequest{
		Context:     runnerContext,
		Group:       runnerGroup,
		Definitions: defs,
	}))
	if err != nil {
		return err
	}

	testIDs := make([]string, len(regRes.Msg.Tests))
	for i, reg := range regRes.Msg.Tests {
		testIDs[i] = reg.Id
	}

	err = w.base.Run(worker.InterruptCh())
	return err
}

type tester interface {
	workflow(ctx workflow.Context, payload *testsv1.Payload) error
	paramType() (bool, reflect.Type)
}

type registeredTest struct {
	name         string
	test         tester
	defaultParam any
}

func getTaskQueue(context string, groupName string) string {
	return fmt.Sprintf("%s-%s", context, groupName)
}

func getRunnerIdentify(taskQueueName string) string {
	return fmt.Sprintf("%d@%s@%s", os.Getpid(), getHostName(), taskQueueName)
}

func getHostName() string {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "Unknown"
	}
	return hostName
}
