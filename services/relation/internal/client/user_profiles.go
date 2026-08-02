package client

import (
	"context"

	pb "SuIM/proto/userpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UserProfiles fetches display fields for friend list enrichment.
type UserProfiles struct {
	client pb.UserServiceClient
}

func NewUserProfiles(conn grpc.ClientConnInterface) *UserProfiles {
	return &UserProfiles{client: pb.NewUserServiceClient(conn)}
}

type Profile struct {
	Nickname  string
	AvatarURL string
}

func (c *UserProfiles) GetByIDs(ctx context.Context, userIDs []string) (map[string]Profile, error) {
	out := make(map[string]Profile, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	resp, err := c.client.GetUsersByIDs(forwardMD(ctx), &pb.GetUsersByIDsReq{UserIds: userIDs})
	if err != nil {
		return out, err
	}
	for _, u := range resp.GetUsers() {
		if u == nil || u.UserId == "" {
			continue
		}
		out[u.UserId] = Profile{Nickname: u.Nickname, AvatarURL: u.AvatarUrl}
	}
	return out, nil
}

// LookupProfiles adapts GetByIDs to the gRPC handler ProfileMap shape.
func (c *UserProfiles) LookupProfiles(ctx context.Context, userIDs []string) (map[string]struct {
	Nickname  string
	AvatarURL string
}, error) {
	src, err := c.GetByIDs(ctx, userIDs)
	out := make(map[string]struct {
		Nickname  string
		AvatarURL string
	}, len(src))
	for id, p := range src {
		out[id] = struct {
			Nickname  string
			AvatarURL string
		}{Nickname: p.Nickname, AvatarURL: p.AvatarURL}
	}
	return out, err
}

func forwardMD(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}
