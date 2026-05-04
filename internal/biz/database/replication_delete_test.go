package database

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestDeleteInstanceReplicaDeletesExistingRelation(t *testing.T) {
	repo := &instanceReplicaRepoForDeleteTest{
		item: &DatabaseInstanceReplica{
			Model:             gorm.Model{ID: 7},
			PrimaryInstanceID: 40,
			ReplicaInstanceID: 42,
			Engine:            DBTypeMySQL,
			ReplicaRole:       DatabaseReplicaRoleRealtime,
		},
	}
	uc := &UseCase{instanceReplicaRepo: repo}

	err := uc.DeleteInstanceReplica(context.Background(), 7, QueryOperator{Username: "admin"})
	if err != nil {
		t.Fatalf("delete instance replica: %v", err)
	}
	if repo.deletedID != 7 {
		t.Fatalf("expected delete id 7, got %d", repo.deletedID)
	}
}

func TestDeleteInstanceReplicaRejectsMissingRelation(t *testing.T) {
	repo := &instanceReplicaRepoForDeleteTest{}
	uc := &UseCase{instanceReplicaRepo: repo}

	err := uc.DeleteInstanceReplica(context.Background(), 7, QueryOperator{Username: "admin"})
	if err == nil {
		t.Fatalf("expected missing relation error")
	}
	if repo.deletedID != 0 {
		t.Fatalf("expected no delete call, got %d", repo.deletedID)
	}
}

func TestDeleteReplicaActionDeletesExistingRecord(t *testing.T) {
	repo := &replicaActionRepoForDeleteTest{
		item: &DatabaseReplicaAction{
			Model:             gorm.Model{ID: 11},
			ReplicaID:         7,
			PrimaryInstanceID: 40,
			ReplicaInstanceID: 42,
			Action:            DatabaseReplicaApplyActionPause,
			Status:            DatabaseQueryStatusSuccess,
		},
	}
	uc := &UseCase{replicaActionRepo: repo}

	err := uc.DeleteReplicaAction(context.Background(), 11, QueryOperator{Username: "admin"})
	if err != nil {
		t.Fatalf("delete replica action: %v", err)
	}
	if repo.deletedID != 11 {
		t.Fatalf("expected delete id 11, got %d", repo.deletedID)
	}
}

func TestDeleteReplicaActionRejectsMissingRecord(t *testing.T) {
	repo := &replicaActionRepoForDeleteTest{}
	uc := &UseCase{replicaActionRepo: repo}

	err := uc.DeleteReplicaAction(context.Background(), 11, QueryOperator{Username: "admin"})
	if err == nil {
		t.Fatalf("expected missing action error")
	}
	if repo.deletedID != 0 {
		t.Fatalf("expected no delete call, got %d", repo.deletedID)
	}
}

type instanceReplicaRepoForDeleteTest struct {
	item      *DatabaseInstanceReplica
	deletedID uint
}

func (r *instanceReplicaRepoForDeleteTest) Create(context.Context, *DatabaseInstanceReplica) error {
	return nil
}

func (r *instanceReplicaRepoForDeleteTest) Update(context.Context, *DatabaseInstanceReplica) error {
	return nil
}

func (r *instanceReplicaRepoForDeleteTest) Delete(_ context.Context, id uint) error {
	r.deletedID = id
	return nil
}

func (r *instanceReplicaRepoForDeleteTest) GetByID(_ context.Context, id uint) (*DatabaseInstanceReplica, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}

func (r *instanceReplicaRepoForDeleteTest) GetByReplicaInstanceID(_ context.Context, replicaInstanceID uint) (*DatabaseInstanceReplica, error) {
	if r.item == nil || r.item.ReplicaInstanceID != replicaInstanceID {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}

func (r *instanceReplicaRepoForDeleteTest) UpsertByReplicaInstance(_ context.Context, item *DatabaseInstanceReplica) (*DatabaseInstanceReplica, error) {
	r.item = item
	return item, nil
}

func (r *instanceReplicaRepoForDeleteTest) List(context.Context, *DatabaseInstanceReplicaListRequest) ([]*DatabaseInstanceReplica, int64, error) {
	return nil, 0, nil
}

type replicaActionRepoForDeleteTest struct {
	item      *DatabaseReplicaAction
	deletedID uint
}

func (r *replicaActionRepoForDeleteTest) Create(context.Context, *DatabaseReplicaAction) error {
	return nil
}

func (r *replicaActionRepoForDeleteTest) Update(context.Context, *DatabaseReplicaAction) error {
	return nil
}

func (r *replicaActionRepoForDeleteTest) Delete(_ context.Context, id uint) error {
	r.deletedID = id
	return nil
}

func (r *replicaActionRepoForDeleteTest) GetByID(_ context.Context, id uint) (*DatabaseReplicaAction, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}

func (r *replicaActionRepoForDeleteTest) List(context.Context, *DatabaseReplicaActionListRequest) ([]*DatabaseReplicaAction, int64, error) {
	return nil, 0, nil
}
