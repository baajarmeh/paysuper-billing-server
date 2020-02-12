// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import billingpb "github.com/paysuper/paysuper-proto/go/billingpb"
import context "context"
import mock "github.com/stretchr/testify/mock"
import pkg "github.com/paysuper/paysuper-billing-server/internal/pkg"

import time "time"

// AccountingEntryRepositoryInterface is an autogenerated mock type for the AccountingEntryRepositoryInterface type
type AccountingEntryRepositoryInterface struct {
	mock.Mock
}

// ApplyObjectSource provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4, _a5
func (_m *AccountingEntryRepositoryInterface) ApplyObjectSource(_a0 context.Context, _a1 string, _a2 string, _a3 string, _a4 string, _a5 *billingpb.AccountingEntry) error {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4, _a5)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string, *billingpb.AccountingEntry) error); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4, _a5)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// BulkWrite provides a mock function with given fields: _a0, _a1
func (_m *AccountingEntryRepositoryInterface) BulkWrite(_a0 context.Context, _a1 []*billingpb.AccountingEntry) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []*billingpb.AccountingEntry) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FindBySource provides a mock function with given fields: _a0, _a1, _a2
func (_m *AccountingEntryRepositoryInterface) FindBySource(_a0 context.Context, _a1 string, _a2 string) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, string) []*billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindByTypeCountryDates provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *AccountingEntryRepositoryInterface) FindByTypeCountryDates(_a0 context.Context, _a1 string, _a2 []string, _a3 time.Time, _a4 time.Time) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, time.Time, time.Time) []*billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, []string, time.Time, time.Time) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetById provides a mock function with given fields: _a0, _a1
func (_m *AccountingEntryRepositoryInterface) GetById(_a0 context.Context, _a1 string) (*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string) *billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.AccountingEntry)
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

// GetByObjectSource provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *AccountingEntryRepositoryInterface) GetByObjectSource(_a0 context.Context, _a1 string, _a2 string, _a3 string) (*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 *billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) *billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetByTypeWithTaxes provides a mock function with given fields: _a0, _a1
func (_m *AccountingEntryRepositoryInterface) GetByTypeWithTaxes(_a0 context.Context, _a1 []string) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCorrectionsForRoyaltyReport provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *AccountingEntryRepositoryInterface) GetCorrectionsForRoyaltyReport(_a0 context.Context, _a1 string, _a2 string, _a3 time.Time, _a4 time.Time) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, string, time.Time, time.Time) []*billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, time.Time, time.Time) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDistinctBySourceId provides a mock function with given fields: _a0
func (_m *AccountingEntryRepositoryInterface) GetDistinctBySourceId(_a0 context.Context) ([]string, error) {
	ret := _m.Called(_a0)

	var r0 []string
	if rf, ok := ret.Get(0).(func(context.Context) []string); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRollingReserveForBalance provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *AccountingEntryRepositoryInterface) GetRollingReserveForBalance(_a0 context.Context, _a1 string, _a2 string, _a3 []string, _a4 time.Time) ([]*pkg.ReserveQueryResItem, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4)

	var r0 []*pkg.ReserveQueryResItem
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []string, time.Time) []*pkg.ReserveQueryResItem); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*pkg.ReserveQueryResItem)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, []string, time.Time) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRollingReservesForRoyaltyReport provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4, _a5
func (_m *AccountingEntryRepositoryInterface) GetRollingReservesForRoyaltyReport(_a0 context.Context, _a1 string, _a2 string, _a3 []string, _a4 time.Time, _a5 time.Time) ([]*billingpb.AccountingEntry, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4, _a5)

	var r0 []*billingpb.AccountingEntry
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []string, time.Time, time.Time) []*billingpb.AccountingEntry); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4, _a5)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*billingpb.AccountingEntry)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, []string, time.Time, time.Time) error); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4, _a5)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MultipleInsert provides a mock function with given fields: _a0, _a1
func (_m *AccountingEntryRepositoryInterface) MultipleInsert(_a0 context.Context, _a1 []*billingpb.AccountingEntry) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []*billingpb.AccountingEntry) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
