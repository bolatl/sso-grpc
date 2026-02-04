package auth

import ssov1 "github.com/bolatl/protos/gen/go/sso"

type serverAPI struct {
	ssov1.UnimplementedAuthServer
}
