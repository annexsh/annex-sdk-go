package test

import (
	testv1 "github.com/annexhq/annex-proto/gen/go/type/test/v1"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

func DecodeParam[P any](payload *testv1.Payload) (P, error) {
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

func convertAnnexPayload(testPayload *testv1.Payload) *common.Payload {
	return &common.Payload{
		Metadata: testPayload.Metadata,
		Data:     testPayload.Data,
	}
}
