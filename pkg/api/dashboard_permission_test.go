package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/accesscontrol/actest"
	"github.com/grafana/grafana/pkg/services/dashboards"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web/webtest"
)

func TestHTTPServer_GetDashboardPermissionList(t *testing.T) {
	t.Run("should not be able to list acl when user does not have permission to do so", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {})

		res, err := server.Send(webtest.RequestWithSignedInUser(server.NewGetRequest("/api/dashboards/uid/1/permissions"), userWithPermissions(1, nil)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})

	t.Run("should be able to list acl with correct permission", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {
			svc := dashboards.NewFakeDashboardService(t)
			svc.On("GetDashboard", mock.Anything, mock.Anything).Return(&dashboards.Dashboard{ID: 1, UID: "1"}, nil)
			hs.DashboardService = svc
			hs.dashboardPermissionsService = &actest.FakePermissionsService{
				ExpectedPermissions: []accesscontrol.ResourcePermission{},
			}
		})

		res, err := server.Send(webtest.RequestWithSignedInUser(server.NewGetRequest("/api/dashboards/uid/1/permissions"), userWithPermissions(1, []accesscontrol.Permission{
			{Action: dashboards.ActionDashboardsPermissionsRead, Scope: "dashboards:uid:1"},
		})))

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})

	t.Run("should filter out hidden users from acl", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {
			cfg := setting.NewCfg()
			cfg.HiddenUsers = map[string]struct{}{"hidden": {}}
			hs.Cfg = cfg

			svc := dashboards.NewFakeDashboardService(t)
			svc.On("GetDashboard", mock.Anything, mock.Anything).Return(&dashboards.Dashboard{ID: 1, UID: "1"}, nil)

			hs.DashboardService = svc
			hs.dashboardPermissionsService = &actest.FakePermissionsService{
				ExpectedPermissions: []accesscontrol.ResourcePermission{
					{UserID: 1, UserLogin: "regular", IsManaged: true},
					{UserID: 2, UserLogin: "hidden", IsManaged: true},
				},
			}
		})

		res, err := server.Send(webtest.RequestWithSignedInUser(server.NewGetRequest("/api/dashboards/uid/1/permissions"), userWithPermissions(1, []accesscontrol.Permission{
			{Action: dashboards.ActionDashboardsPermissionsRead, Scope: "dashboards:uid:1"},
		})))

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		var result []dashboards.DashboardACLInfoDTO
		require.NoError(t, json.NewDecoder(res.Body).Decode(&result))

		assert.Len(t, result, 1)
		assert.Equal(t, result[0].UserLogin, "regular")
		require.NoError(t, res.Body.Close())
	})
}

func TestHTTPServer_UpdateDashboardPermissions(t *testing.T) {
	t.Run("should not be able to update acl when user does not have permission to do so", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {})

		res, err := server.Send(webtest.RequestWithSignedInUser(server.NewPostRequest("/api/dashboards/uid/1/permissions", nil), userWithPermissions(1, nil)))
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})

	t.Run("should be able to update acl with correct permissions", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {
			svc := dashboards.NewFakeDashboardService(t)
			svc.On("GetDashboard", mock.Anything, mock.Anything).Return(&dashboards.Dashboard{ID: 1, UID: "1"}, nil)

			hs.DashboardService = svc
			hs.dashboardPermissionsService = &actest.FakePermissionsService{}
		})

		body := `{"items": []}`
		res, err := server.SendJSON(webtest.RequestWithSignedInUser(server.NewPostRequest("/api/dashboards/uid/1/permissions", strings.NewReader(body)), userWithPermissions(1, []accesscontrol.Permission{
			{Action: dashboards.ActionDashboardsPermissionsWrite, Scope: "dashboards:uid:1"},
		})))

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})

	t.Run("should not be able to specify team and user in same acl", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {
			hs.DashboardService = dashboards.NewFakeDashboardService(t)
			hs.dashboardPermissionsService = &actest.FakePermissionsService{}
		})

		body := `{"items": [{ userId:1, teamId: 2 }]}`
		res, err := server.SendJSON(webtest.RequestWithSignedInUser(server.NewPostRequest("/api/dashboards/uid/1/permissions", strings.NewReader(body)), userWithPermissions(1, []accesscontrol.Permission{
			{Action: dashboards.ActionDashboardsPermissionsWrite, Scope: "dashboards:uid:1"},
		})))

		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})

	t.Run("should not be able to specify team and role in same acl", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {
			hs.DashboardService = dashboards.NewFakeDashboardService(t)
			hs.dashboardPermissionsService = &actest.FakePermissionsService{}
		})

		body := `{"items": [{ teamId:1, role: "Admin" }]}`
		res, err := server.SendJSON(webtest.RequestWithSignedInUser(server.NewPostRequest("/api/dashboards/uid/1/permissions", strings.NewReader(body)), userWithPermissions(1, []accesscontrol.Permission{
			{Action: dashboards.ActionDashboardsPermissionsWrite, Scope: "dashboards:uid:1"},
		})))

		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})

	t.Run("should not be able to specify user and role in same acl", func(t *testing.T) {
		server := SetupAPITestServer(t, func(hs *HTTPServer) {
			hs.DashboardService = dashboards.NewFakeDashboardService(t)
			hs.dashboardPermissionsService = &actest.FakePermissionsService{}
		})

		body := `{"items": [{ userId:1, role: "Admin" }]}`
		res, err := server.SendJSON(webtest.RequestWithSignedInUser(server.NewPostRequest("/api/dashboards/uid/1/permissions", strings.NewReader(body)), userWithPermissions(1, []accesscontrol.Permission{
			{Action: dashboards.ActionDashboardsPermissionsWrite, Scope: "dashboards:uid:1"},
		})))

		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		require.NoError(t, res.Body.Close())
	})
}
