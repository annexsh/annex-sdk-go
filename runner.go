package annex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/MakeNowJust/heredoc/v2"
	testsv1 "github.com/annexsh/annex-proto/go/gen/annex/tests/v1"
	"github.com/annexsh/annex-proto/go/gen/annex/tests/v1/testsv1connect"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/annexsh/annex-sdk-go/internal/temporal"
)

type TestSuiteRunnerConfig struct {
	HostPort      string
	Context       string
	TestSuiteName string
	TestSuiteDesc string
}

type TestSuiteRunner struct {
	ctx             context.Context
	id              string
	context         string
	suiteID         string
	client          client.Client
	worker          worker.Worker
	testClient      testsv1connect.TestServiceClient
	registeredTests []registeredTest
}

func NewTestSuiteRunner(cfg TestSuiteRunnerConfig) (*TestSuiteRunner, error) {
	ctx := context.Background()

	httpc := &http.Client{Timeout: time.Minute}

	connectURL := "http://" + cfg.HostPort + "/connect"
	testClient := testsv1connect.NewTestServiceClient(httpc, connectURL)

	temporalClient, err := temporal.NewClient(ctx, cfg.HostPort, cfg.Context)
	if err != nil {
		return nil, err
	}

	suiteReq := &testsv1.RegisterTestSuiteRequest{
		Context: cfg.Context,
		Name:    cfg.TestSuiteName,
	}
	if strings.TrimSpace(cfg.TestSuiteDesc) != "" {
		desc := heredoc.Doc(cfg.TestSuiteDesc)
		suiteReq.Description = &desc
	}
	suiteRes, err := testClient.RegisterTestSuite(ctx, connect.NewRequest(suiteReq))
	if err != nil {
		return nil, err
	}

	taskQueue := getTaskQueue(cfg.Context, suiteRes.Msg.Id)
	id := getRunnerIdentify(taskQueue)

	wrk := worker.New(temporalClient, taskQueue, worker.Options{
		DisableRegistrationAliasing: true,
		Interceptors: []interceptor.WorkerInterceptor{
			temporal.NewWorkerLogInterceptor(testClient),
		},
		Identity: id,
	})

	wrk.RegisterActivity(temporal.NewTestLogActivity(testClient))

	return &TestSuiteRunner{
		ctx:        ctx,
		id:         id,
		context:    cfg.Context,
		suiteID:    suiteRes.Msg.Id,
		client:     temporalClient,
		worker:     wrk,
		testClient: testClient,
	}, nil
}

func RegisterTest(runner *TestSuiteRunner, name string, test func(t TestT)) {
	runner.registeredTests = append(runner.registeredTests, registeredTest{
		name: name,
		test: &simpleTest{test: test},
	})
}

func RegisterInputTest[P any](runner *TestSuiteRunner, name string, test func(t TestT, param P)) {
	runner.registeredTests = append(runner.registeredTests, registeredTest{
		name:         name,
		test:         &paramTest[P]{test: test},
		defaultParam: *new(P),
	})
}

func RegisterCase(runner *TestSuiteRunner, caseFn func(t CaseT)) {
	c := simpleCase{caseFn: caseFn}
	runner.worker.RegisterActivityWithOptions(c.activity, activity.RegisterOptions{
		Name: c.name(),
	})
}

func RegisterInputCase[P any](runner *TestSuiteRunner, caseFn func(t CaseT, param P)) {
	c := paramCase[P]{caseFn: caseFn}
	runner.worker.RegisterActivityWithOptions(c.activity, activity.RegisterOptions{
		Name: c.name(),
	})
}

func (w *TestSuiteRunner) Run() error {
	var defs []*testsv1.TestDefinition

	for _, reg := range w.registeredTests {
		w.worker.RegisterWorkflowWithOptions(reg.test.workflow, workflow.RegisterOptions{
			Name: reg.name,
		})

		def := &testsv1.TestDefinition{
			Name:         reg.name,
			DefaultInput: nil,
		}

		if reg.defaultParam != nil {
			p, err := converter.NewJSONPayloadConverter().ToPayload(reg.defaultParam)
			if err != nil {
				return err
			}
			def.DefaultInput = &testsv1.Payload{
				Data:     p.Data,
				Metadata: p.Metadata,
			}
		}

		defs = append(defs, def)
	}

	version, err := getTestSuiteVersion(defs)
	if err != nil {
		return err
	}

	stream := w.testClient.RegisterTests(w.ctx)

	for _, def := range defs {
		if err = stream.Send(&testsv1.RegisterTestsRequest{
			Context:     w.context,
			TestSuiteId: w.suiteID,
			Definition:  def,
			Version:     version,
			RunnerId:    w.id,
		}); err != nil {
			return err
		}
	}

	if _, err = stream.CloseAndReceive(); err != nil {
		return err
	}

	return w.worker.Run(worker.InterruptCh())
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

func getTaskQueue(context string, suiteID string) string {
	return fmt.Sprintf("%s-%s", context, suiteID)
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

func getTestSuiteVersion(defs []*testsv1.TestDefinition) (string, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err := encoder.Encode(defs); err != nil {
		return "", err
	}

	hash := sha256.Sum256(buffer.Bytes())
	return hex.EncodeToString(hash[:]), nil
}
