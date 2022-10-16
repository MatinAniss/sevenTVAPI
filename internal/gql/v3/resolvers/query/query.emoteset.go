package query

import (
	"context"

	"github.com/seventv/api/internal/gql/v3/gen/model"
	"github.com/seventv/common/errors"
	"github.com/seventv/common/mongo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (r *Resolver) EmoteSet(ctx context.Context, id primitive.ObjectID) (*model.EmoteSet, error) {
	set, err := r.Ctx.Inst().Loaders.EmoteSetByID().Load(id)
	if err != nil {
		return nil, err
	}

	return r.Ctx.Inst().Modelizer.EmoteSet(set).GQL(), nil
}

func (r *Resolver) NamedEmoteSet(ctx context.Context, name model.EmoteSetName) (*model.EmoteSet, error) {
	var setID primitive.ObjectID

	switch name {
	case model.EmoteSetNameGlobal:
		sys, err := r.Ctx.Inst().Mongo.System(ctx)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, errors.ErrUnknownEmoteSet()
			}

			return nil, errors.ErrInternalServerError()
		}

		setID = sys.EmoteSetID
	}

	if setID.IsZero() {
		return nil, errors.ErrUnknownEmoteSet()
	}

	set, err := r.Ctx.Inst().Loaders.EmoteSetByID().Load(setID)
	if err != nil {
		return nil, err
	}

	return r.Ctx.Inst().Modelizer.EmoteSet(set).GQL(), nil
}
