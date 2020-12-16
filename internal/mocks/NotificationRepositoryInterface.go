// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	billingpb "github.com/paysuper/paysuper-proto/go/billingpb"

	mock "github.com/stretchr/testify/mock"
)

// NotificationRepositoryInterface is an autogenerated mock type for the NotificationRepositoryInterface type
type NotificationRepositoryInterface struct {
	mock.Mock
}

// Find provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4, _a5, _a6
func (_m *NotificationRepositoryInterface) Find(_a0 context.Context, _a1 string, _a2 string, _a3 int32, _a4 []string, _a5 int64, _a6 int64) ([]*billingpb.Notification, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4, _a5, _a6)

	var r0 []*billingpb.Notification
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int32, []string, int64, int64) []*billingpb.Notification); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4, _a5, _a6)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.Notification)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, int32, []string, int64, int64) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4, _a5, _a6)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindCount provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *NotificationRepositoryInterface) FindCount(_a0 context.Context, _a1 string, _a2 string, _a3 int32) (int64, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 int64
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int32) int64); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, int32) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetById provides a mock function with given fields: _a0, _a1
func (_m *NotificationRepositoryInterface) GetById(_a0 context.Context, _a1 string) (*billingpb.Notification, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *billingpb.Notification
	if rf, ok := ret.Get(0).(func(context.Context, string) *billingpb.Notification); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.Notification)
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

// Insert provides a mock function with given fields: _a0, _a1
func (_m *NotificationRepositoryInterface) Insert(_a0 context.Context, _a1 *billingpb.Notification) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billingpb.Notification) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Update provides a mock function with given fields: _a0, _a1
func (_m *NotificationRepositoryInterface) Update(_a0 context.Context, _a1 *billingpb.Notification) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billingpb.Notification) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
