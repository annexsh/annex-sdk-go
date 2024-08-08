package test

import (
	"github.com/annexsh/annex-proto/go/gen/annex/tests/v1"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

func DecodeParam[P any](payload *testsv1.Payload) (P, error) {
	converted := convertAnnexPayload(payload)
	dc := converter.GetDefaultDataConverter()
	var param P
	if err := dc.FromPayload(converted, &param); err != nil {
		return param, err
	}
	return param, nil
}

func DecodeTemporalParam[P any](payload *common.Payload) (P, error) {
	dc := converter.GetDefaultDataConverter()
	var param P
	if err := dc.FromPayload(payload, &param); err != nil {
		return param, err
	}
	return param, nil
}

func convertAnnexPayload(testPayload *testsv1.Payload) *common.Payload {
	return &common.Payload{
		Metadata: testPayload.Metadata,
		Data:     testPayload.Data,
	}
}
