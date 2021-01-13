// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	billingpb "github.com/paysuper/paysuper-proto/go/billingpb"

	mock "github.com/stretchr/testify/mock"
)

// MerchantDocumentRepositoryInterface is an autogenerated mock type for the MerchantDocumentRepositoryInterface type
type MerchantDocumentRepositoryInterface struct {
	mock.Mock
}

// CountByMerchantId provides a mock function with given fields: _a0, _a1
func (_m *MerchantDocumentRepositoryInterface) CountByMerchantId(_a0 context.Context, _a1 string) (int64, error) {
	ret := _m.Called(_a0, _a1)

	var r0 int64
	if rf, ok := ret.Get(0).(func(context.Context, string) int64); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetById provides a mock function with given fields: _a0, _a1
func (_m *MerchantDocumentRepositoryInterface) GetById(_a0 context.Context, _a1 string) (*billingpb.MerchantDocument, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *billingpb.MerchantDocument
	if rf, ok := ret.Get(0).(func(context.Context, string) *billingpb.MerchantDocument); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.MerchantDocument)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetByMerchantId provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *MerchantDocumentRepositoryInterface) GetByMerchantId(_a0 context.Context, _a1 string, _a2 int64, _a3 int64) ([]*billingpb.MerchantDocument, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 []*billingpb.MerchantDocument
	if rf, ok := ret.Get(0).(func(context.Context, string, int64, int64) []*billingpb.MerchantDocument); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.MerchantDocument)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, int64, int64) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Insert provides a mock function with given fields: _a0, _a1
func (_m *MerchantDocumentRepositoryInterface) Insert(_a0 context.Context, _a1 *billingpb.MerchantDocument) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billingpb.MerchantDocument) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}