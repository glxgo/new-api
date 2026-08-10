/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestVirtualMembershipInstanceResponseIncludesLifetimeUsage(t *testing.T) {
	response := virtualMembershipInstanceResponse(&model.UserVirtualMembership{
		Id: 7, WeeklyQuota: 1_000, WeeklyUsed: 20, LifetimeUsed: 12_345,
	})

	require.EqualValues(t, 12_345, response["lifetime_used"])
}
