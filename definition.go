package annex

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/annexsh/annex-proto/gen/go/annex/tests/v1"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/workflow"

	"github.com/annexsh/annex-sdk-go/internal/name"
	"github.com/annexsh/annex-sdk-go/internal/test"
	"github.com/annexsh/annex-sdk-go/internal/testing"
)

type simpleTest struct {
	test func(t TestT)
}

func (f *simpleTest) workflow(ctx workflow.Context, payload *testsv1.Payload) error {
	if f.test == nil {
		return errors.New("flow cannot be nil")
	}

	if payload != nil {
		return errors.New("unexpected payload received: test does not have a parameter defined")
	}

	return test.ExecuteTest(ctx, func(ctx workflow.Context) {
		f.test(testing.NewT(ctx))
	})
}

func (f *simpleTest) paramType() (bool, reflect.Type) {
	return false, nil
}

type paramTest[P any] struct {
	test func(t TestT, param P)
}

func (f *paramTest[P]) workflow(ctx workflow.Context, payload *testsv1.Payload) error {
	if f.test == nil {
		return errors.New("test cannot be nil")
	}

	param, err := test.DecodeParam[P](payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal test param: %w", err)
	}

	return test.ExecuteTest(ctx, func(ctx workflow.Context) {
		f.test(testing.NewT(ctx), param)
	})
}

func (f *paramTest[P]) paramType() (bool, reflect.Type) {
	var zeroParam P
	return true, reflect.TypeOf(zeroParam)
}

type simpleCase struct {
	caseFn func(t CaseT)
}

func (c *simpleCase) activity(ctx context.Context, payload *common.Payload) (any, error) {
	if c.caseFn == nil {
		return nil, errors.New("case cannot be nil")
	}

	if payload != nil {
		return nil, errors.New("unexpected payload received: case does not have a parameter defined")
	}

	return test.ExecuteCase(ctx, func(ctx context.Context) {
		c.caseFn(testing.NewCaseT(ctx))
	})
}

func (c *simpleCase) name() string {
	return name.FuncName(c.caseFn)
}

type paramCase[P any] struct {
	caseFn func(t CaseT, param P)
}

func (c *paramCase[P]) activity(ctx context.Context, payload *common.Payload) (any, error) {
	if c.caseFn == nil {
		return nil, errors.New("case cannot be nil")
	}

	param, err := test.DecodeTemporalParam[P](payload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal case param: %w", err)
	}

	executeCase, err := test.ExecuteCase(ctx, func(ctx context.Context) {
		c.caseFn(testing.NewCaseT(ctx), param)
	})
	return executeCase, err
}

func (c *paramCase[P]) name() string {
	return name.FuncName(c.caseFn)
}

func getWorkflowT(t TestT) testing.WorkflowT {
	if casted, ok := t.(testing.WorkflowT); ok {
		return casted
	}
	panic("not a valid test")
}
