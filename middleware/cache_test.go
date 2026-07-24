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
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "root document", path: "/", want: "no-cache"},
		{name: "api", path: "/api/status", want: "no-cache"},
		{
			name: "fingerprinted javascript",
			path: "/static/js/index.daa3ede12a.js",
			want: "public, max-age=31536000, immutable",
		},
		{
			name: "fingerprinted stylesheet",
			path: "/static/css/index.3d61b7517a.css",
			want: "public, max-age=31536000, immutable",
		},
		{
			name: "non fingerprinted asset",
			path: "/logo.png",
			want: "public, max-age=604800",
		},
		{name: "spa route", path: "/pricing/", want: "no-cache"},
		{
			name: "file-like spa route",
			path: "/pricing/model.abcdef1234.test",
			want: "no-cache",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, test.path, nil)

			Cache()(context)

			if got := recorder.Header().Get("Cache-Control"); got != test.want {
				t.Fatalf("Cache-Control = %q, want %q", got, test.want)
			}
		})
	}
}
