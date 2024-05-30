package test

import (
	"github.com/annexhq/annex/test"
	"go.temporal.io/sdk/workflow"
)

type Pending struct {
	id     test.CaseExecutionID
	name   string
	future workflow.Future
	err    error
}

func (p *Pending) IsReady() bool {
	return p.err != nil || p.IsReady()
}

func NewPending(id test.CaseExecutionID, name string, base workflow.Future) *Pending {
	return &Pending{
		id:     id,
		name:   name,
		future: base,
	}
}

func NewPendingError(id test.CaseExecutionID, name string, err error) *Pending {
	return &Pending{
		id:   id,
		name: name,
		err:  err,
	}
}

func GetPendingFuture(pending *Pending) workflow.Future {
	return pending.future
}

func GetPendingError(pending *Pending) error {
	return pending.err
}
