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
	deleteErr           error
	validateErr         error
	validatedRoleID     uint
	validatedInstanceID uint
	upserted            *dbbiz.DatabaseInstancePermission
	deletedID           uint
	byID                *dbbiz.DatabaseInstancePermissionVO
	byRoleInstance      *dbbiz.DatabaseInstancePermissionVO
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
	if r.admin {
		return dbbiz.DatabasePermissionAll, nil
	}
	return r.permissions[instanceID], nil
}

func (r *fakeDatabasePermissionRepo) GetUserAccessibleInstanceIDs(context.Context, uint, uint) ([]uint, error) {
	return r.allowedIDs, r.err
}

func (r *fakeDatabasePermissionRepo) List(context.Context, *dbbiz.DatabaseInstancePermissionListRequest) ([]*dbbiz.DatabaseInstancePermissionVO, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *fakeDatabasePermissionRepo) GetByID(context.Context, uint) (*dbbiz.DatabaseInstancePermissionVO, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.byID == nil {
		return nil, nil
	}
	item := *r.byID
	return &item, nil
}

func (r *fakeDatabasePermissionRepo) GetByRoleInstance(context.Context, uint, uint) (*dbbiz.DatabaseInstancePermissionVO, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.byRoleInstance == nil {
		return nil, nil
	}
	item := *r.byRoleInstance
	return &item, nil
}

func (r *fakeDatabasePermissionRepo) ValidateTarget(_ context.Context, roleID, instanceID uint) error {
	r.validatedRoleID = roleID
	r.validatedInstanceID = instanceID
	return r.validateErr
}

func (r *fakeDatabasePermissionRepo) Upsert(_ context.Context, item *dbbiz.DatabaseInstancePermission) error {
	r.upserted = item
	if r.byRoleInstance == nil {
		r.byRoleInstance = &dbbiz.DatabaseInstancePermissionVO{
			ID:           99,
			RoleID:       item.RoleID,
			RoleName:     "DBA",
			RoleCode:     "dba",
			InstanceID:   item.InstanceID,
			InstanceName: "prod-mysql",
		}
	}
	r.byRoleInstance.Permissions = item.Permissions
	return r.err
}

func (r *fakeDatabasePermissionRepo) Delete(_ context.Context, id uint) error {
	r.deletedID = id
	return r.deleteErr
}

type fakeDatabaseQueryAuditRepo struct {
	created []*dbbiz.DatabaseQueryAudit
	err     error
}

func (r *fakeDatabaseQueryAuditRepo) Create(_ context.Context, item *dbbiz.DatabaseQueryAudit) error {
	if r.err != nil {
		return r.err
	}
	r.created = append(r.created, item)
	return nil
}

func (r *fakeDatabaseQueryAuditRepo) Update(context.Context, *dbbiz.DatabaseQueryAudit) error {
	return nil
}

func (r *fakeDatabaseQueryAuditRepo) List(context.Context, *dbbiz.DatabaseQueryAuditListRequest) ([]*dbbiz.DatabaseQueryAudit, int64, error) {
	return nil, 0, nil
}

func (r *fakeDatabaseQueryAuditRepo) ListHistory(context.Context, uint, *dbbiz.DatabaseQueryHistoryRequest) ([]*dbbiz.DatabaseQueryAudit, error) {
	return nil, nil
}

func newPermissionTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Set(rbacservice.UserIdKey, uint(7))
	c.Set(rbacservice.UsernameKey, "alice")
	return c, recorder
}

func newPermissionAuditUseCase(auditRepo dbbiz.QueryAuditRepo) *dbbiz.UseCase {
	return dbbiz.NewUseCase(nil, nil, nil, nil, nil, nil, nil, nil, auditRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestEnsureInstancePermissionAllowsLegacyWhenNoRules(t *testing.T) {
	c, recorder := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{hasRules: false})

	if !service.ensureInstancePermission(c, 42, dbbiz.DatabasePermissionWrite) {
		t.Fatalf("expected legacy no-rule mode to allow instance operation, response=%s", recorder.Body.String())
	}
}

func TestAllowUnlimitedQueryRowsRequiresAdminOrExplicitPermission(t *testing.T) {
	c, _ := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{hasRules: false})
	if service.allowUnlimitedQueryRows(c, 42) {
		t.Fatal("expected no-rule non-admin user to be denied unlimited query rows")
	}

	c, _ = newPermissionTestContext()
	service = NewService(nil, &fakeDatabasePermissionRepo{hasRules: false, admin: true})
	if !service.allowUnlimitedQueryRows(c, 42) {
		t.Fatal("expected admin to be allowed unlimited query rows")
	}

	c, _ = newPermissionTestContext()
	service = NewService(nil, &fakeDatabasePermissionRepo{
		hasRules: true,
		permissions: map[uint]uint{
			42: dbbiz.DatabasePermissionQuery | dbbiz.DatabasePermissionQueryUnlimited,
		},
	})
	if !service.allowUnlimitedQueryRows(c, 42) {
		t.Fatal("expected explicit unlimited query permission to allow unlimited query rows")
	}
}

func TestAllowWriteExplainQueryRequiresAdminOrExplicitPermission(t *testing.T) {
	c, _ := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{hasRules: false})
	if service.allowWriteExplainQuery(c, 42) {
		t.Fatal("expected no-rule non-admin user to be denied write explain")
	}

	c, _ = newPermissionTestContext()
	service = NewService(nil, &fakeDatabasePermissionRepo{hasRules: false, admin: true})
	if !service.allowWriteExplainQuery(c, 42) {
		t.Fatal("expected admin to be allowed write explain")
	}

	c, _ = newPermissionTestContext()
	service = NewService(nil, &fakeDatabasePermissionRepo{
		hasRules: true,
		permissions: map[uint]uint{
			42: dbbiz.DatabasePermissionQuery | dbbiz.DatabasePermissionWriteExplain,
		},
	})
	if !service.allowWriteExplainQuery(c, 42) {
		t.Fatal("expected explicit write explain permission to allow write explain")
	}
}

func TestEnsureInstancePermissionWhitelistModeRejectsNoRules(t *testing.T) {
	c, recorder := newPermissionTestContext()
	service := NewService(nil, &fakeDatabasePermissionRepo{hasRules: false}, func(context.Context) (string, error) {
		return "whitelist", nil
	})

	if service.ensureInstancePermission(c, 42, dbbiz.DatabasePermissionWrite) {
		t.Fatal("expected whitelist no-rule mode to reject instance operation")
	}
	if !strings.Contains(recorder.Body.String(), "权限不足") {
		t.Fatalf("expected permission error response, got %s", recorder.Body.String())
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

func TestUpsertInstancePermissionRecordsAudit(t *testing.T) {
	c, recorder := newPermissionTestContext()
	c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(`{"roleId":3,"instanceId":42,"permissions":3}`))
	c.Request.Header.Set("Content-Type", "application/json")
	repo := &fakeDatabasePermissionRepo{
		byRoleInstance: &dbbiz.DatabaseInstancePermissionVO{
			ID:           11,
			RoleID:       3,
			RoleName:     "DBA",
			RoleCode:     "dba",
			InstanceID:   42,
			InstanceName: "prod-mysql",
			Permissions:  dbbiz.DatabasePermissionView,
		},
	}
	auditRepo := &fakeDatabaseQueryAuditRepo{}
	service := NewService(newPermissionAuditUseCase(auditRepo), repo)

	service.UpsertInstancePermission(c)

	if repo.upserted == nil {
		t.Fatalf("expected permission to be saved, response=%s", recorder.Body.String())
	}
	if len(auditRepo.created) != 1 {
		t.Fatalf("expected one audit record, got %d", len(auditRepo.created))
	}
	audit := auditRepo.created[0]
	if audit.AuditAction != dbbiz.DatabaseAuditActionPermissionUpsert {
		t.Fatalf("unexpected audit action: %s", audit.AuditAction)
	}
	if audit.InstanceID != 42 || audit.OperatorID != 7 || audit.OperatorName != "alice" {
		t.Fatalf("unexpected audit identity: %#v", audit)
	}
	if audit.SQLType != "PERMISSION" || audit.RiskLevel != dbbiz.DatabaseQueryRiskHigh || audit.Status != dbbiz.DatabaseQueryStatusSuccess {
		t.Fatalf("unexpected audit metadata: %#v", audit)
	}
	if !strings.Contains(audit.SQLText, `"beforePermissions":1`) || !strings.Contains(audit.SQLText, `"afterPermissions":3`) || !strings.Contains(audit.SQLText, `"roleName":"DBA"`) {
		t.Fatalf("unexpected audit payload: %s", audit.SQLText)
	}
}

func TestDeleteInstancePermissionRecordsAudit(t *testing.T) {
	c, recorder := newPermissionTestContext()
	c.Params = gin.Params{{Key: "id", Value: "5"}}
	repo := &fakeDatabasePermissionRepo{
		byID: &dbbiz.DatabaseInstancePermissionVO{
			ID:           5,
			RoleID:       3,
			RoleName:     "DBA",
			RoleCode:     "dba",
			InstanceID:   42,
			InstanceName: "prod-mysql",
			Permissions:  dbbiz.DatabasePermissionQuery,
		},
	}
	auditRepo := &fakeDatabaseQueryAuditRepo{}
	service := NewService(newPermissionAuditUseCase(auditRepo), repo)

	service.DeleteInstancePermission(c)

	if repo.deletedID != 5 {
		t.Fatalf("expected permission 5 to be deleted, got %d, response=%s", repo.deletedID, recorder.Body.String())
	}
	if len(auditRepo.created) != 1 {
		t.Fatalf("expected one audit record, got %d", len(auditRepo.created))
	}
	audit := auditRepo.created[0]
	if audit.AuditAction != dbbiz.DatabaseAuditActionPermissionDelete {
		t.Fatalf("unexpected audit action: %s", audit.AuditAction)
	}
	if !strings.Contains(audit.SQLText, `"beforePermissions":2`) || !strings.Contains(audit.SQLText, `"afterPermissions":0`) {
		t.Fatalf("unexpected audit payload: %s", audit.SQLText)
	}
}
