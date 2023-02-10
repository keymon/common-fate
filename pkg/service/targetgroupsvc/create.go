package targetgroupsvc

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/common-fate/common-fate/pkg/storage"
	"github.com/common-fate/common-fate/pkg/targetgroup"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/ddb"
	"go.uber.org/zap"
)

func (s *Service) compareAndValidateProviderVersions(ctx context.Context, provider1 string, provider2 string) (bool, error) {
	splitKey := strings.Split(provider1, "/")

	//the target schema we receive should be in the form team/provider/version and split into 3 keys
	if len(splitKey) != 3 {
		return false, errors.New("target schema given in incorrect format")
	}

	provider1Resp, err := s.ProviderRegistryClient.GetProviderWithResponse(ctx, splitKey[0], splitKey[1], splitKey[2])
	if err != nil {
		return false, err
	}

	splitKey = strings.Split(provider2, "/")

	//the target schema we receive should be in the form team/provider/version and split into 3 keys
	if len(splitKey) != 3 {
		return false, errors.New("target schema given in incorrect format")
	}

	provider2Resp, err := s.ProviderRegistryClient.GetProviderWithResponse(ctx, splitKey[0], splitKey[1], splitKey[2])
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(provider1Resp.JSON200.Schema.Target, provider2Resp.JSON200.Schema.Target), nil

}

func (s *Service) CreateTargetGroupLink(ctx context.Context, req types.CreateTargetGroupLink, targetGroupId string) (*targetgroup.TargetGroup, error) {
	log := zap.S()

	//validate target group exists
	q := storage.GetTargetGroup{ID: targetGroupId}

	_, err := s.DB.Query(ctx, &q)

	if err == ddb.ErrNoItems {

		return nil, err
	}

	//lookup deployment
	p := storage.GetTargetGroupDeployment{ID: req.DeploymentId}

	_, err = s.DB.Query(ctx, &p)

	if err == ddb.ErrNoItems {
		return nil, err
	}

	//validate deployment target schema matches target group
	//TODO?

	//check to see if deployment is already linked with a target
	checkDeployment := storage.GetTargetGroupDeploymentWithGroupId{TargetGroupId: targetGroupId}

	_, err = s.DB.Query(ctx, &checkDeployment)

	if err == nil {
		//means that there was a deployment already linked with a target group so this cannot be linked
		return nil, errors.New("target group deployment already linked with a target group")
	} else {
		//dont want to break execution for no items found
		if err != ddb.ErrNoItems {
			return nil, err
		}
	}

	//update the target group assignment on the deployment object
	p.Result.TargetGroupAssignment = targetgroup.TargetGroupAssignment{
		TargetGroupID: p.Result.ID,
		Priority:      req.Priority,
		Diagnostics:   p.Result.Diagnostics,
	}

	log.Debugw("Linking deployment to target group", "group", q.Result.ID)
	// save the request.
	err = s.DB.Put(ctx, &p.Result)
	if err != nil {
		return nil, err
	}

	return &q.Result, nil
}

type Provider struct {
	Publisher string
	Name      string
	Version   string
}

func SplitProviderString(s string) (Provider, error) {
	splitversion := strings.Split(s, "@")
	if len(splitversion) != 2 {
		return Provider{}, errors.New("target schema given in incorrect format")
	}

	splitname := strings.Split(splitversion[0], "/")
	if len(splitname) != 2 {
		return Provider{}, errors.New("target schema given in incorrect format")
	}
	p := Provider{
		Publisher: splitname[0],
		Name:      splitname[1],
		Version:   splitversion[1],
	}
	return p, nil
}

func (s *Service) CreateTargetGroup(ctx context.Context, req types.CreateTargetGroupRequest) (*targetgroup.TargetGroup, error) {
	log := zap.S()

	q := &storage.GetTargetGroup{
		ID: req.ID,
	}

	_, err := s.DB.Query(ctx, q)
	if err == nil {
		return nil, ErrTargetGroupIdAlreadyExists
	}
	if err != nil && err != ddb.ErrNoItems {
		return nil, err
	}
	//look up target schema for the provider version
	provider, err := SplitProviderString(req.TargetSchema)
	if err != nil {
		return nil, err
	}
	result, err := s.ProviderRegistryClient.GetProviderWithResponse(ctx, provider.Publisher, provider.Name, provider.Version)
	if err != nil {
		return nil, err
	}

	if result.StatusCode() != 200 {
		return nil, errors.New(string(result.Body))
	}

	now := s.Clock.Now()
	group := targetgroup.TargetGroup{
		ID:           req.ID,
		TargetSchema: targetgroup.GroupTargetSchema{From: req.TargetSchema, Schema: result.JSON200.Schema.Target},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	//based on the target schema provider type set the Icon

	log.Debugw("saving target group", "group", group)
	// save the request.
	err = s.DB.Put(ctx, &group)
	if err != nil {
		return nil, err
	}
	return &group, nil
}
