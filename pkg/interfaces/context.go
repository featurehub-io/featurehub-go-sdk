package interfaces

import "github.com/featurehub-io/featurehub-go-sdk/pkg/models"

type Context interface {
	RepositoryContext
	Attributes() *models.Context
	Repository() Repository
	WithContext(ctx *models.Context) Context
}
