package testing

import (
	"context"
	"fmt"

	"github.com/annexsh/annex/test"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

type Logger interface {
	Debug(msg string, keyvals ...any)
	Info(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
	Error(msg string, keyvals ...any)
}

type WorkflowT interface {
	WorkflowContext() workflow.Context
	NextCaseExecutionID() test.CaseExecutionID
}

type TestT struct {
	*CollectT
	ctx     workflow.Context
	current test.CaseExecutionID
}

func NewT(ctx workflow.Context) *TestT {
	return &TestT{
		CollectT: new(CollectT),
		ctx:      ctx,
		current:  0,
	}
}

func (t *TestT) WorkflowContext() workflow.Context {
	return t.ctx
}

func (t *TestT) Logger() Logger {
	return workflow.GetLogger(t.ctx)
}

func (t *TestT) NextCaseExecutionID() test.CaseExecutionID {
	t.current++
	return t.current
}

type CaseT struct {
	*CollectT
	ctx context.Context
}

func NewCaseT(ctx context.Context) *CaseT {
	return &CaseT{
		CollectT: new(CollectT),
		ctx:      ctx,
	}
}

func (t *CaseT) Context() context.Context {
	return t.ctx
}

func (t *CaseT) Logger() Logger {
	return activity.GetLogger(t.ctx)
}

type tHelper interface {
	Helper()
}

// CollectT implements the TestingT interface and collects all errors.
type CollectT struct {
	errors []error
}

// Errorf collects the error.
func (c *CollectT) Errorf(format string, args ...interface{}) {
	c.errors = append(c.errors, fmt.Errorf(format, args...))
}

// FailNow panics.
func (c *CollectT) FailNow() {
	panic("Assertion failed")
}

// Reset clears the collected errors.
func (c *CollectT) Reset() {
	c.errors = nil
}

// Copy copies the collected errors to the supplied t.
func (c *CollectT) Copy(t assert.TestingT) {
	if tt, ok := t.(tHelper); ok {
		tt.Helper()
	}
	for _, err := range c.errors {
		t.Errorf("%v", err)
	}
}
