// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import billingpb "github.com/paysuper/paysuper-proto/go/billingpb"
import context "context"
import mock "github.com/stretchr/testify/mock"

import time "time"

// AccountingServiceInterface is an autogenerated mock type for the AccountingServiceInterface type
type AccountingServiceInterface struct {
	mock.Mock
}

// GetCorrectionsForRoyaltyReport provides a mock function with given fields: ctx, merchantId, currency, from, to
func (_m *AccountingServiceInterface) GetCorrectionsForRoyaltyReport(ctx context.Context, merchantId string, currency string, from time.Time, to time.Time) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(ctx, merchantId, currency, from, to)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, string, time.Time, time.Time) []*billingpb.AccountingEntry); ok {
		r0 = rf(ctx, merchantId, currency, from, to)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, time.Time, time.Time) error); ok {
		r1 = rf(ctx, merchantId, currency, from, to)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRollingReservesForRoyaltyReport provides a mock function with given fields: ctx, merchantId, currency, from, to
func (_m *AccountingServiceInterface) GetRollingReservesForRoyaltyReport(ctx context.Context, merchantId string, currency string, from time.Time, to time.Time) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(ctx, merchantId, currency, from, to)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, string, time.Time, time.Time) []*billingpb.AccountingEntry); ok {
		r0 = rf(ctx, merchantId, currency, from, to)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, time.Time, time.Time) error); ok {
		r1 = rf(ctx, merchantId, currency, from, to)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
