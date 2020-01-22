// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import billingpb "github.com/paysuper/paysuper-proto/go/billingpb"
import mock "github.com/stretchr/testify/mock"
import proto "github.com/golang/protobuf/proto"

// PaymentSystem is an autogenerated mock type for the PaymentSystem type
type PaymentSystem struct {
	mock.Mock
}

// CreatePayment provides a mock function with given fields: order, successUrl, failUrl, requisites
func (_m *PaymentSystem) CreatePayment(order *billingpb.Order, successUrl string, failUrl string, requisites map[string]string) (string, error) {
	ret := _m.Called(order, successUrl, failUrl, requisites)

	var r0 string
	if rf, ok := ret.Get(0).(func(*billingpb.Order, string, string, map[string]string) string); ok {
		r0 = rf(order, successUrl, failUrl, requisites)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*billingpb.Order, string, string, map[string]string) error); ok {
		r1 = rf(order, successUrl, failUrl, requisites)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateRefund provides a mock function with given fields: order, refund
func (_m *PaymentSystem) CreateRefund(order *billingpb.Order, refund *billingpb.Refund) error {
	ret := _m.Called(order, refund)

	var r0 error
	if rf, ok := ret.Get(0).(func(*billingpb.Order, *billingpb.Refund) error); ok {
		r0 = rf(order, refund)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetRecurringId provides a mock function with given fields: request
func (_m *PaymentSystem) GetRecurringId(request proto.Message) string {
	ret := _m.Called(request)

	var r0 string
	if rf, ok := ret.Get(0).(func(proto.Message) string); ok {
		r0 = rf(request)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// IsRecurringCallback provides a mock function with given fields: request
func (_m *PaymentSystem) IsRecurringCallback(request proto.Message) bool {
	ret := _m.Called(request)

	var r0 bool
	if rf, ok := ret.Get(0).(func(proto.Message) bool); ok {
		r0 = rf(request)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// ProcessPayment provides a mock function with given fields: order, message, raw, signature
func (_m *PaymentSystem) ProcessPayment(order *billingpb.Order, message proto.Message, raw string, signature string) error {
	ret := _m.Called(order, message, raw, signature)

	var r0 error
	if rf, ok := ret.Get(0).(func(*billingpb.Order, proto.Message, string, string) error); ok {
		r0 = rf(order, message, raw, signature)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ProcessRefund provides a mock function with given fields: order, refund, message, raw, signature
func (_m *PaymentSystem) ProcessRefund(order *billingpb.Order, refund *billingpb.Refund, message proto.Message, raw string, signature string) error {
	ret := _m.Called(order, refund, message, raw, signature)

	var r0 error
	if rf, ok := ret.Get(0).(func(*billingpb.Order, *billingpb.Refund, proto.Message, string, string) error); ok {
		r0 = rf(order, refund, message, raw, signature)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
