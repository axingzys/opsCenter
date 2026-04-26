package database

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	dbbiz "github.com/ydcloud-dy/opshub/internal/biz/database"
	rbacservice "github.com/ydcloud-dy/opshub/internal/service/rbac"
)

type fakeDatabasePermissionRepo struct {
	hasRules            bool
	admin               bool
	allowedIDs          []uint
	permissions         map[uint]uint
	err                 error
	validateErr         error
	validatedRoleID     uint
	validatedInstanceID uint
	upserted            *dbbiz.DatabaseInstancePermission
}

func (r *fakeDatabasePermissionRepo) HasAnyRules(context.Context) (bool, error) {
	return r.hasRules, r.err
}

func (r *fakeDatabasePermissionRepo) IsAdmin(context.Context, uint) (bool, error) {
	return r.admin, r.err
}

func (r *fakeDatabasePermissionRepo) GetUserInstancePermissions(_ context.Context, _ uint, instanceID uint) (uint, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.permissions[instanceID], nil
}

func (r *fakeDatabasePermissionRepo) GetUserAccessibleInstanceIDs(context.Context, uint, uint) ([]uint, error) {
	return r.allowedIDs, r.err
}

func (r *fakeDatabasePermissionRepo) List(context.Context, *dbbiz.DatabaseInstancePermissionListRequest) ([]*dbbiz.DatabaseInstancePermissionVO, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *fakeDatabasePermissionRepo) ValidateTarget(_ context.Context, roleID, instanceID uint) error {
	r.validatedRoleID = roleID
	r.validatedInstanceID = instanceID
	return r.validateErr
}

func (r *fakeDatabasePermissionRepo) Upsert(_ context.Context, item *dbbiz.DatabaseInstancePermission) error {
	r.upserted = item
	return r.err
}

func (r *fakeDatabasePermissionRepo) Delete(context.Context, uint) error {
	return errors.New("not implemented")
}

func newPermissionTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Set(rbacservice.UserIdKey, uint(7))
	return c, recorder
}

func TestEnsureInstancePermissionAllowsLegacyWhenNoRules(t *testing.T) {
	c, recorder := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{hasRules: false})

	if !service.ensureInstancePermission(c, 42, dbbiz.DatabasePermissionWrite) {
		t.Fatalf("expected legacy no-rule mode to allow instance operation, response=%s", recorder.Body.String())
	}
}

func TestEnsureInstancePermissionRejectsMissingBit(t *testing.T) {
	c, recorder := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{
		hasRules: true,
		permissions: map[uint]uint{
			42: dbbiz.DatabasePermissionView,
		},
	})

	if service.ensureInstancePermission(c, 42, dbbiz.DatabasePermissionWrite) {
		t.Fatal("expected write operation to be rejected without write permission")
	}
	if !strings.Contains(recorder.Body.String(), "权限不足") {
		t.Fatalf("expected permission error response, got %s", recorder.Body.String())
	}
}

func TestPermissionScopeRestrictsListRequests(t *testing.T) {
	c, recorder := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{
		hasRules:   true,
		allowedIDs: []uint{2, 4},
	})

	scope, ok := service.databasePermissionScope(c, dbbiz.DatabasePermissionQuery)
	if !ok {
		t.Fatalf("expected scope resolution to succeed, response=%s", recorder.Body.String())
	}
	req := &dbbiz.DatabaseQueryAuditListRequest{}
	applyAllowedInstanceScope(req, scope)
	if !req.RestrictToAllowed {
		t.Fatal("expected list request to be restricted")
	}
	if len(req.AllowedInstanceIDs) != 2 || req.AllowedInstanceIDs[0] != 2 || req.AllowedInstanceIDs[1] != 4 {
		t.Fatalf("unexpected allowed ids: %#v", req.AllowedInstanceIDs)
	}
}

func TestUpsertInstancePermissionValidatesTargetBeforeSaving(t *testing.T) {
	c, recorder := newPermissionTestContext()
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(`{"roleId":3,"instanceId":42,"permissions":3}`))
	c.Request.Header.Set("Content-Type", "application/json")
	repo := &fakeDatabasePermissionRepo{}
	service := NewService(nil, repo)

	service.UpsertInstancePermission(c)

	if repo.validatedRoleID != 3 || repo.validatedInstanceID != 42 {
		t.Fatalf("expected target validation for role 3 and instance 42, got role=%d instance=%d", repo.validatedRoleID, repo.validatedInstanceID)
	}
	if repo.upserted == nil {
		t.Fatalf("expected permission to be saved, response=%s", recorder.Body.String())
	}
	if repo.upserted.Permissions != 3 {
		t.Fatalf("unexpected saved permission mask: %d", repo.upserted.Permissions)
	}
}

func TestUpsertInstancePermissionRejectsInvalidTarget(t *testing.T) {
	c, recorder := newPermissionTestContext()
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(`{"roleId":3,"instanceId":42,"permissions":3}`))
	c.Request.Header.Set("Content-Type", "application/json")
	repo := &fakeDatabasePermissionRepo{validateErr: errors.New("角色不存在或已禁用")}
	service := NewService(nil, repo)

	service.UpsertInstancePermission(c)

	if repo.upserted != nil {
		t.Fatal("expected invalid target to stop before saving")
	}
	if !strings.Contains(recorder.Body.String(), "角色不存在或已禁用") {
		t.Fatalf("expected target validation error response, got %s", recorder.Body.String())
	}
}
