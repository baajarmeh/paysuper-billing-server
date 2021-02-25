// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	billingpb "github.com/paysuper/paysuper-proto/go/billingpb"

	mock "github.com/stretchr/testify/mock"

	time "time"
)

// RecurringSubscriptionRepositoryInterface is an autogenerated mock type for the RecurringSubscriptionRepositoryInterface type
type RecurringSubscriptionRepositoryInterface struct {
	mock.Mock
}

// Find provides a mock function with given fields: ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo, limit, offset
func (_m *RecurringSubscriptionRepositoryInterface) Find(ctx context.Context, userId string, merchantId string, status string, quickFilter string, dateFrom *time.Time, dateTo *time.Time, limit int64, offset int64) ([]*billingpb.RecurringSubscription, error) {
	ret := _m.Called(ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo, limit, offset)

	var r0 []*billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string, *time.Time, *time.Time, int64, int64) []*billingpb.RecurringSubscription); ok {
		r0 = rf(ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo, limit, offset)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.RecurringSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string, *time.Time, *time.Time, int64, int64) error); ok {
		r1 = rf(ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo, limit, offset)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindByCustomerId provides a mock function with given fields: _a0, _a1
func (_m *RecurringSubscriptionRepositoryInterface) FindByCustomerId(_a0 context.Context, _a1 string) ([]*billingpb.RecurringSubscription, error) {
	ret := _m.Called(_a0, _a1)

	var r0 []*billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string) []*billingpb.RecurringSubscription); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.RecurringSubscription)
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

// FindByMerchantIdCustomerId provides a mock function with given fields: ctx, merchantId, customerId
func (_m *RecurringSubscriptionRepositoryInterface) FindByMerchantIdCustomerId(ctx context.Context, merchantId string, customerId string) ([]*billingpb.RecurringSubscription, error) {
	ret := _m.Called(ctx, merchantId, customerId)

	var r0 []*billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string, string) []*billingpb.RecurringSubscription); ok {
		r0 = rf(ctx, merchantId, customerId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.RecurringSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, merchantId, customerId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindByPlanId provides a mock function with given fields: ctx, planId
func (_m *RecurringSubscriptionRepositoryInterface) FindByPlanId(ctx context.Context, planId string) ([]*billingpb.RecurringSubscription, error) {
	ret := _m.Called(ctx, planId)

	var r0 []*billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string) []*billingpb.RecurringSubscription); ok {
		r0 = rf(ctx, planId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.RecurringSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, planId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindCount provides a mock function with given fields: ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo
func (_m *RecurringSubscriptionRepositoryInterface) FindCount(ctx context.Context, userId string, merchantId string, status string, quickFilter string, dateFrom *time.Time, dateTo *time.Time) (int64, error) {
	ret := _m.Called(ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo)

	var r0 int64
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string, *time.Time, *time.Time) int64); ok {
		r0 = rf(ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo)
	} else {
		r0 = ret.Get(0).(int64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string, *time.Time, *time.Time) error); ok {
		r1 = rf(ctx, userId, merchantId, status, quickFilter, dateFrom, dateTo)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindExpired provides a mock function with given fields: ctx, expireAt
func (_m *RecurringSubscriptionRepositoryInterface) FindExpired(ctx context.Context, expireAt time.Time) ([]*billingpb.RecurringSubscription, error) {
	ret := _m.Called(ctx, expireAt)

	var r0 []*billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, time.Time) []*billingpb.RecurringSubscription); ok {
		r0 = rf(ctx, expireAt)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.RecurringSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, time.Time) error); ok {
		r1 = rf(ctx, expireAt)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetActiveByPlanIdCustomerId provides a mock function with given fields: ctx, planId, customerId
func (_m *RecurringSubscriptionRepositoryInterface) GetActiveByPlanIdCustomerId(ctx context.Context, planId string, customerId string) (*billingpb.RecurringSubscription, error) {
	ret := _m.Called(ctx, planId, customerId)

	var r0 *billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *billingpb.RecurringSubscription); ok {
		r0 = rf(ctx, planId, customerId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.RecurringSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, planId, customerId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetById provides a mock function with given fields: _a0, _a1
func (_m *RecurringSubscriptionRepositoryInterface) GetById(_a0 context.Context, _a1 string) (*billingpb.RecurringSubscription, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string) *billingpb.RecurringSubscription); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.RecurringSubscription)
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

// GetByPlanIdCustomerId provides a mock function with given fields: ctx, planId, customerId
func (_m *RecurringSubscriptionRepositoryInterface) GetByPlanIdCustomerId(ctx context.Context, planId string, customerId string) (*billingpb.RecurringSubscription, error) {
	ret := _m.Called(ctx, planId, customerId)

	var r0 *billingpb.RecurringSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *billingpb.RecurringSubscription); ok {
		r0 = rf(ctx, planId, customerId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.RecurringSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, planId, customerId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Insert provides a mock function with given fields: _a0, _a1
func (_m *RecurringSubscriptionRepositoryInterface) Insert(_a0 context.Context, _a1 *billingpb.RecurringSubscription) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billingpb.RecurringSubscription) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Update provides a mock function with given fields: _a0, _a1
func (_m *RecurringSubscriptionRepositoryInterface) Update(_a0 context.Context, _a1 *billingpb.RecurringSubscription) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *billingpb.RecurringSubscription) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}