package accesssvc

import (
	"context"

	"github.com/benbjohnson/clock"

	"github.com/common-fate/common-fate/pkg/cache"
	"github.com/common-fate/common-fate/pkg/gevent"
	"github.com/common-fate/common-fate/pkg/identity"
	"github.com/common-fate/common-fate/pkg/requests"
	"github.com/common-fate/common-fate/pkg/rule"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/ddb"
)

// Service holds business logic relating to Access Requests.
type Service struct {
	Clock       clock.Clock
	DB          ddb.Storage
	EventPutter EventPutter
	Cache       CacheService
	AHClient    AHClient
	Rules       AccessRuleService
	Workflow    Workflow
}

type CreateGrantOpts struct {
	ID          string
	With        map[string]string
	AccessRule  rule.AccessRule
	RequestedBy identity.User
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/workflow.go -package=mocks . Workflow
type Workflow interface {
	Grant(ctx context.Context, request requests.AccessGroup, subject string) ([]requests.Grantv2, error)
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/eventputter.go -package=mocks . EventPutter
type EventPutter interface {
	Put(ctx context.Context, detail gevent.EventTyper) error
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/cache.go -package=mocks . CacheService
type CacheService interface {
	RefreshCachedProviderArgOptions(ctx context.Context, providerId string, argId string) (bool, []cache.ProviderOption, []cache.ProviderArgGroupOption, error)
	LoadCachedProviderArgOptions(ctx context.Context, providerId string, argId string) (bool, []cache.ProviderOption, []cache.ProviderArgGroupOption, error)
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_accessrule_service.go -package=mocks . AccessRuleService

// AccessRuleService can create and get rules
type AccessRuleService interface {
	// RequestArguments(ctx context.Context, accessRuleTarget rule.Target) (map[string]types.RequestArgument, error)
}

type AHClient interface {
	types.ClientWithResponsesInterface
}
