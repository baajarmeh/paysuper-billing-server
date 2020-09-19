// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// CentrifugoInterface is an autogenerated mock type for the CentrifugoInterface type
type CentrifugoInterface struct {
	mock.Mock
}

// GetChannelToken provides a mock function with given fields: subject, expire
func (_m *CentrifugoInterface) GetChannelToken(subject string, expire int64) string {
	ret := _m.Called(subject, expire)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, int64) string); ok {
		r0 = rf(subject, expire)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Publish provides a mock function with given fields: _a0, _a1, _a2
func (_m *CentrifugoInterface) Publish(_a0 context.Context, _a1 string, _a2 interface{}) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, interface{}) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
