// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// MembershipNotifierMock is an autogenerated mock type for the MembershipNotifierMock type
type MembershipNotifierMock struct {
	mock.Mock
}

// MembershipChange provides a mock function with given fields:
func (_m *MembershipNotifierMock) MembershipChange() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}
