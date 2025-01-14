package eventbridge

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/seventv/api/data/events"
	"github.com/seventv/api/internal/global"
	"github.com/seventv/common/dataloader"
	"github.com/seventv/common/errors"
	"github.com/seventv/common/structures/v3"
	"github.com/seventv/common/utils"
	"go.uber.org/zap"
)

const (
	identifier_foreign_username = "foreign_username"
	identifier_foreign_id       = "foreign_id"
	identifier_username         = "username"
	identifier_id               = "id"
)

var userStateLoader *dataloader.DataLoader[string, structures.User]

func createUserStateLoader(gctx global.Context) {
	userStateLoader = dataloader.New(dataloader.Config[string, structures.User]{
		Fetch: func(keys []string) ([]structures.User, []error) {
			var (
				errs []error
				v    []structures.User
			)

			identifierMap := map[string]utils.Set[string]{
				identifier_foreign_username: {},
				identifier_foreign_id:       {},
				identifier_id:               {},
				identifier_username:         {},
			}

			for _, key := range keys {
				// Identify the target
				keysp := strings.SplitN(key, "|", 2)
				if len(keysp) != 2 {
					continue
				}

				platform := keysp[0]

				idsp := strings.SplitN(keysp[1], ":", 2)
				idType := idsp[0]
				identifier := idsp[1]

				// Platform specified: find by connection
				if platform != "" {
					switch idType {
					case "id":
						identifierMap["foreign_id"].Add(platform + ":" + identifier)
					case "username":
						identifierMap["foreign_username"].Add(platform + ":" + identifier)
					}
				} else { // no platform means app user
					switch idType {
					case "id":
						identifierMap["id"].Add(identifier)
					case "username":
						identifierMap["username"].Add(identifier)
					}
				}
			}

			wg := sync.WaitGroup{}
			mx := sync.Mutex{}

			for idType, identifiers := range identifierMap {
				if len(identifiers) == 0 {
					continue
				}

				wg.Add(1)

				go func(idType string, identifiers utils.Set[string]) {
					defer wg.Done()

					var users = []structures.User{}

					switch idType {
					case identifier_foreign_id, identifier_foreign_username:
						l := utils.Ternary(idType == identifier_foreign_id, gctx.Inst().Loaders.UserByConnectionID, gctx.Inst().Loaders.UserByConnectionUsername)

						m := make(map[structures.UserConnectionPlatform][]string)

						for _, id := range identifiers.Values() {
							idsp := strings.SplitN(id, ":", 2)
							if len(idsp) != 2 {
								continue
							}

							platform := structures.UserConnectionPlatform((idsp[0]))
							id := idsp[1]

							m[platform] = append(m[platform], id)
						}

						for p, ids := range m {
							users, errs = l(p).LoadAll(ids)
						}
					case identifier_id:
						//iden := identifiers.Values()
						//idList := utils.Map(iden, func(x string) primitive.ObjectID {
						//	oid, err := primitive.ObjectIDFromHex(x)
						//	if err != nil {
						//		return primitive.NilObjectID
						//	}

						//	return oid
						//})

						// v, errs = gctx.Inst().Loaders.UserByID().LoadAll(idList)
					case identifier_username:
						// v, errs = gctx.Inst().Loaders.UserByUsername().LoadAll(identifiers.Values())
					}

					mx.Lock()

					v = append(v, users...)

					mx.Unlock()

					for _, err := range errs {
						if err == nil || errors.Compare(err, errors.ErrUnknownUser()) {
							continue
						}

						zap.S().Errorw("failed to load users for bridged cosmetics request command", "error", err)

						break
					}
				}(idType, identifiers)
			}

			wg.Wait()

			return v, errs
		},
		Wait: 250 * time.Millisecond,
	})
}

func handleUserState(gctx global.Context, ctx context.Context, body events.UserStateCommandBody) ([]events.Message[json.RawMessage], error) {
	keys := make([]string, len(body.Identifiers))

	for i, id := range body.Identifiers {
		params := strings.Builder{}
		params.WriteString(string(body.Platform))
		params.WriteString("|")
		params.WriteString(id)

		keys[i] = params.String()
	}

	users, _ := userStateLoader.LoadAll(keys)

	var sid string
	switch t := ctx.Value(SESSION_ID_KEY).(type) {
	case string:
		sid = t
	}

	if sid == "" {
		zap.S().Errorw("failed to get session id from context")
		return nil, nil
	}

	result := []events.Message[json.RawMessage]{}

	// Dispatch user avatar
	for _, user := range users {
		if (user.Avatar != nil || user.AvatarID != "") &&
			user.HasPermission(structures.RolePermissionFeatureProfilePictureAnimation) {
			av := utils.ToJSON(gctx.Inst().Modelizer.Avatar(user))

			result = append(result, events.NewMessage(events.OpcodeDispatch, events.DispatchPayload{
				Type: events.EventTypeCreateCosmetic,
				Body: events.ChangeMap{
					ID:         user.ID,
					Kind:       structures.ObjectKindCosmetic,
					Contextual: true,
					Object:     av,
				},
				Whisper: sid,
			}).ToRaw())
		}
	}

	return result, nil
}
