package api

import (
	"errors"
	"net/http"

	"github.com/common-fate/apikit/apio"
	"github.com/common-fate/common-fate/pkg/auth"
	"github.com/common-fate/common-fate/pkg/cache"

	"github.com/common-fate/common-fate/pkg/service/preflightsvc"
	"github.com/common-fate/common-fate/pkg/storage"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/ddb"
)

// List Requests
// (GET /api/v1/requests)
func (a *API) UserListRequests(w http.ResponseWriter, r *http.Request, params types.UserListRequestsParams) {
	ctx := r.Context()
	u := auth.UserFromContext(ctx)
	q := storage.ListRequestWithGroupsWithTargetsForUser{UserID: u.ID}
	var opts []func(*ddb.QueryOpts)
	if params.NextToken != nil {
		opts = append(opts, ddb.Page(*params.NextToken))
	}

	qo, err := a.DB.Query(ctx, &q, opts...)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	res := types.ListRequestsResponse{
		Requests: []types.Request{},
	}
	if qo.NextPage != "" {
		res.Next = &qo.NextPage
	}

	for _, request := range q.Result {
		res.Requests = append(res.Requests, request.ToAPI())
	}

	apio.JSON(ctx, w, res, http.StatusOK)
}

// Get Request
// (GET /api/v1/requests/{requestId})
func (a *API) UserGetRequest(w http.ResponseWriter, r *http.Request, requestId string) {
	ctx := r.Context()
	u := auth.UserFromContext(ctx)
	q := storage.GetRequestWithGroupsWithTargetsForUser{UserID: u.ID, RequestID: requestId}
	_, err := a.DB.Query(ctx, &q)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	apio.JSON(ctx, w, q.Result.ToAPI(), http.StatusOK)
}

// List Entitlements
// (GET /api/v1/entitlements)
func (a *API) UserListEntitlements(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	q := storage.ListTargetGroups{}
	_, err := a.DB.Query(ctx, &q)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	res := types.ListTargetGroupResponse{
		TargetGroups: []types.TargetGroup{},
	}

	for _, e := range q.Result {
		res.TargetGroups = append(res.TargetGroups, e.ToAPI())
	}
	apio.JSON(ctx, w, res, http.StatusOK)

}

// (POST /api/v1/preflight)
func (a *API) UserRequestPreflight(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var createPreflightRequest types.CreatePreflightRequest
	err := apio.DecodeJSONBody(w, r, &createPreflightRequest)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}
	user := auth.UserFromContext(ctx)

	out, err := a.PreflightService.ProcessPreflight(ctx, *user, createPreflightRequest)
	if err == preflightsvc.ErrDuplicateTargetIDsRequested {
		apio.Error(ctx, w, apio.NewRequestError(err, http.StatusBadRequest))
		return
	}
	if err == preflightsvc.ErrUserNotAuthorisedForRequestedTarget {
		apio.Error(ctx, w, apio.NewRequestError(err, http.StatusUnauthorized))
		return
	}
	if err == ddb.ErrNoItems {
		apio.Error(ctx, w, apio.NewRequestError(err, http.StatusNotFound))
		return
	}
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	apio.JSON(ctx, w, out.ToAPI(), http.StatusOK)
}

// (POST /api/v1/requests)
func (a *API) UserPostRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := auth.UserFromContext(ctx)

	var createRequest types.CreateAccessRequestRequest
	err := apio.DecodeJSONBody(w, r, &createRequest)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	//check preflight exists for user here and return if not found
	preflight := storage.GetPreflight{
		ID:     createRequest.PreflightId,
		UserId: u.ID,
	}

	_, err = a.DB.Query(ctx, &preflight)
	if err == ddb.ErrNoItems {
		apio.Error(ctx, w, &apio.APIError{Err: errors.New("preflight not found"), Status: http.StatusNotFound})
		return
	}
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	//request create service takes a preflight request, validates its fields and initiates the granding process
	//on all of the entitlements in the preflight
	_, err = a.Access.CreateRequest(ctx, createRequest)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}
	//do we need to return anything via this api?
	apio.JSON(ctx, w, nil, http.StatusOK)
}

func (a *API) UserRevokeRequest(w http.ResponseWriter, r *http.Request, requestID string) {
	// ctx := r.Context()
	// isAdmin := auth.IsAdmin(ctx)
	// u := auth.UserFromContext(ctx)
	// var req requests.Requestv2
	// q := storage.GetRequestV2{ID: requestID}
	// _, err := a.DB.Query(ctx, &q)
	// if err == ddb.ErrNoItems {
	// 	//grant not found return 404
	// 	apio.Error(ctx, w, apio.NewRequestError(errors.New("request not found or you don't have access to it"), http.StatusNotFound))
	// 	return
	// }
	// if err != nil {
	// 	apio.Error(ctx, w, err)
	// 	return
	// }
	// // user can revoke their own request and admins can revoke any request
	// if q.Result.RequestedBy.ID == u.ID || isAdmin {
	// 	req = *q.Result
	// } else { // reviewers can revoke reviewable requests
	// 	q := storage.GetRequestReviewer{RequestID: requestID, ReviewerID: u.Email}
	// 	_, err := a.DB.Query(ctx, &q)
	// 	if err == ddb.ErrNoItems {
	// 		//grant not found return 404
	// 		apio.Error(ctx, w, apio.NewRequestError(errors.New("request not found or you don't have access to it"), http.StatusNotFound))
	// 		return
	// 	}
	// 	if err != nil {
	// 		apio.Error(ctx, w, err)
	// 		return
	// 	}
	// 	// req = q.Result.Request
	// }

	// _, err = a.Workflow.Revoke(ctx, req, u.ID, u.Email)
	// if err == workflowsvc.ErrGrantInactive {
	// 	apio.Error(ctx, w, apio.NewRequestError(err, http.StatusBadRequest))
	// 	return
	// }
	// if err == workflowsvc.ErrNoGrant {
	// 	apio.Error(ctx, w, apio.NewRequestError(err, http.StatusBadRequest))
	// 	return
	// }
	// if err != nil {
	// 	apio.Error(ctx, w, err)
	// 	return
	// }

	// // analytics.FromContext(ctx).Track(&analytics.RequestRevoked{
	// // 	RequestedBy: req.RequestedBy,
	// // 	RevokedBy:   u.ID,
	// // 	RuleID:      req.Rule,
	// // 	Timing:      req.RequestedTiming.ToAnalytics(),
	// // 	HasReason:   req.HasReason(),
	// // })

	// apio.JSON(ctx, w, nil, http.StatusOK)
}

func (a *API) UserListEntitlementTargets(w http.ResponseWriter, r *http.Request, params types.UserListEntitlementTargetsParams) {
	ctx := r.Context()
	q := storage.ListCachedTargets{}
	var opts []func(*ddb.QueryOpts)
	if params.NextToken != nil {
		opts = append(opts, ddb.Page(*params.NextToken))
	}

	qo, err := a.DB.Query(ctx, &q, opts...)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	res := types.ListTargetsResponse{}
	if qo.NextPage != "" {
		res.Next = &qo.NextPage
	}

	user := auth.UserFromContext(ctx)
	filter := cache.NewFilterTargetsByGroups(user.Groups)
	filter.Filter(q.Result)
	targets := filter.Dump()
	for _, target := range targets {
		res.Targets = append(res.Targets, target.ToAPI())
	}

	apio.JSON(ctx, w, res, http.StatusOK)

}

// Your GET endpoint
// (GET /api/v1/requests/past)
func (a *API) UserListRequestsPast(w http.ResponseWriter, r *http.Request, params types.UserListRequestsPastParams) {

}

// Your GET endpoint
// (GET /api/v1/requests/upcoming)
func (a *API) UserListRequestsUpcoming(w http.ResponseWriter, r *http.Request, params types.UserListRequestsUpcomingParams) {

}

// Your GET endpoint
// (GET /api/v1/requests/upcoming)
func (a *API) AdminListRequests(w http.ResponseWriter, r *http.Request, params types.AdminListRequestsParams) {
	ctx := r.Context()
	q := storage.ListRequestWithGroupsWithTargets{}
	var opts []func(*ddb.QueryOpts)
	if params.NextToken != nil {
		opts = append(opts, ddb.Page(*params.NextToken))
	}

	qo, err := a.DB.Query(ctx, &q, opts...)
	if err != nil {
		apio.Error(ctx, w, err)
		return
	}

	res := types.ListRequestsResponse{
		Requests: []types.Request{},
	}
	if qo.NextPage != "" {
		res.Next = &qo.NextPage
	}

	for _, request := range q.Result {
		res.Requests = append(res.Requests, request.ToAPI())
	}

	apio.JSON(ctx, w, res, http.StatusOK)

}
