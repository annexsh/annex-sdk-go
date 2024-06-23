package temporal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/annexsh/annex-proto/gen/go/annex/tests/v1"
	"github.com/annexsh/annex/test"
	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	tlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const logRequestTimeout = 5 * time.Second

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Logger struct {
	logger        *log.Logger
	globalKeyvals string
}

func NewLogger() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Debug(msg string, keyvals ...any) {
	l.log(LevelDebug, msg, keyvals)
}

func (l *Logger) Info(msg string, keyvals ...any) {
	l.log(LevelInfo, msg, keyvals)
}

func (l *Logger) Warn(msg string, keyvals ...any) {
	l.log(LevelWarn, msg, keyvals)
}

func (l *Logger) Error(msg string, keyvals ...any) {
	l.log(LevelError, msg, keyvals)
}

// With returns new logger the prepend every log entry with keyvals.
func (l *Logger) With(keyvals ...any) tlog.Logger {
	logger := &Logger{
		logger: l.logger,
	}
	if l.globalKeyvals != "" {
		logger.globalKeyvals = l.globalKeyvals + " "
	}
	logger.globalKeyvals += strings.TrimSuffix(fmt.Sprintln(keyvals...), "\n")
	return logger
}

func (l *Logger) log(level Level, msg string, keyvals ...any) {
	// To avoid extra space when globalKeyvals is not specified.
	if l.globalKeyvals == "" {
		l.logger.Println(append([]any{level, msg}, keyvals...)...)
	} else {
		l.logger.Println(append([]any{level, msg, l.globalKeyvals}, keyvals...)...)
	}
}

var _ tlog.Logger = (*TestActivityLogger)(nil)

type LogPublisher interface {
	PublishTestExecutionLog(
		ctx context.Context,
		req *connect.Request[testsv1.PublishTestExecutionLogRequest],
	) (*connect.Response[testsv1.PublishTestExecutionLogResponse], error)
}

type CaseOption func(logger *TestActivityLogger)

func WithCaseExecID(caseExecID test.CaseExecutionID) CaseOption {
	return func(logger *TestActivityLogger) {
		logger.caseExecID = &caseExecID
	}
}

type TestActivityLogger struct {
	*Logger
	globalKeyvals string
	pub           LogPublisher
	testExecID    test.TestExecutionID
	caseExecID    *test.CaseExecutionID
}

func NewTestActivityLogger(pub LogPublisher, testExecID test.TestExecutionID, opts ...CaseOption) *TestActivityLogger {
	logger := &TestActivityLogger{
		Logger:     NewLogger(),
		pub:        pub,
		testExecID: testExecID,
	}
	for _, opt := range opts {
		opt(logger)
	}
	return logger
}

func (l *TestActivityLogger) Debug(msg string, keyvals ...any) {
	l.log(LevelDebug, msg, keyvals)
}

func (l *TestActivityLogger) Info(msg string, keyvals ...any) {
	l.log(LevelInfo, msg, keyvals)
}

func (l *TestActivityLogger) Warn(msg string, keyvals ...any) {
	l.log(LevelWarn, msg, keyvals)
}

func (l *TestActivityLogger) Error(msg string, keyvals ...any) {
	l.log(LevelError, msg, keyvals)
}

func (l *TestActivityLogger) log(level Level, msg string, keyvals []any) {
	if !l.testExecID.IsEmpty() {
		req := &testsv1.PublishTestExecutionLogRequest{
			TestExecutionId: l.testExecID.String(),
			CaseExecutionId: nil,
			Level:           string(level),
			Message:         msg,
			CreateTime:      timestamppb.Now(),
		}

		if l.caseExecID != nil {
			cid32 := l.caseExecID.Int32()
			req.CaseExecutionId = &cid32
		}

		ctx, cancel := context.WithTimeout(context.Background(), logRequestTimeout)
		defer cancel()
		if _, err := l.pub.PublishTestExecutionLog(ctx, connect.NewRequest(req)); err != nil {
			kv := append(keyvals, "error", err)
			l.Logger.Error("failed to publish log", kv...)
		}
	}

	l.Logger.log(level, msg, keyvals...)
}

type TestLogRequest struct {
	Level           Level
	Message         string
	KeyVals         []any
	GlobalKeyVals   string
	TestExecutionID test.TestExecutionID
}

type TestLogResponse struct {
	LogID uuid.UUID
}

// TestLogActivity is used to publish test logs from a workflow. This must be
// executed as a local activity to minimize the number of workflow events.
type TestLogActivity struct {
	pub LogPublisher
}

func NewTestLogActivity(pub LogPublisher) *TestLogActivity {
	return &TestLogActivity{
		pub: pub,
	}
}

func (t *TestLogActivity) Publish(ctx context.Context, req TestLogRequest) (*TestLogResponse, error) {
	ctx = ContextWithTestLogConfig(ctx, TestLogConfig{
		TestExecID: req.TestExecutionID,
	})
	logger := activity.GetLogger(ctx)

	keyVals := req.GlobalKeyVals + strings.TrimSuffix(fmt.Sprintln(req.KeyVals...), "\n")

	pubReq := &testsv1.PublishTestExecutionLogRequest{
		TestExecutionId: req.TestExecutionID.String(),
		Level:           string(req.Level),
		Message:         req.Message + " " + keyVals,
		CreateTime:      timestamppb.New(time.Now().UTC()),
	}

	res, err := t.pub.PublishTestExecutionLog(ctx, connect.NewRequest(pubReq))
	if err != nil {
		kv := append(req.KeyVals, "error", err)
		logger.Error("failed to publish test log", kv...)
	}

	logID, err := uuid.Parse(res.Msg.LogId)
	if err != nil {
		return nil, err
	}

	// Return log id so that it is saved in workflow history
	return &TestLogResponse{
		LogID: logID,
	}, nil
}

type TestWorkflowLogger struct {
	*Logger
	ctx        workflow.Context
	testExecID test.TestExecutionID
}

func NewTestWorkflowLogger(ctx workflow.Context, testExecID test.TestExecutionID) *TestWorkflowLogger {
	return &TestWorkflowLogger{
		Logger: NewLogger(),
		ctx: workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
			// local activity requires short start to close timeout
			StartToCloseTimeout: logRequestTimeout + (100 * time.Millisecond),
			// no retries since operation is non-critical
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		}),
		testExecID: testExecID,
	}
}

func (l *TestWorkflowLogger) Debug(msg string, keyvals ...any) {
	l.log(LevelDebug, msg, keyvals...)
}

func (l *TestWorkflowLogger) Info(msg string, keyvals ...any) {
	l.log(LevelInfo, msg, keyvals)
}

func (l *TestWorkflowLogger) Warn(msg string, keyvals ...any) {
	l.log(LevelWarn, msg, keyvals)
}

func (l *TestWorkflowLogger) Error(msg string, keyvals ...any) {
	l.log(LevelError, msg, keyvals)
}

func (l *TestWorkflowLogger) log(level Level, msg string, keyvals ...any) {
	keyvals = append(keyvals, l.globalKeyvals)
	request := TestLogRequest{
		Level:           level,
		Message:         msg,
		KeyVals:         keyvals,
		TestExecutionID: l.testExecID,
	}
	var logActivity TestLogActivity
	if err := workflow.ExecuteLocalActivity(l.ctx, logActivity.Publish, request).Get(l.ctx, nil); err != nil {
		keyvals = append(keyvals, "error", err)
		l.Logger.Error("failed to execute logger local activity", keyvals...)
	}
}
