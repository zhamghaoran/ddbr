package middware

import (
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"reflect"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-gateway/service"
)

// AuthorityMiddleware req reflect refer to https://www.cloudwego.io/zh/docs/kitex/tutorials/framework-exten/middleware/#%E5%A6%82%E4%BD%95%E5%9C%A8%E4%B8%AD%E9%97%B4%E4%BB%B6%E8%8E%B7%E5%8F%96%E5%88%B0%E7%9C%9F%E5%AE%9E%E7%9A%84-request--response
func AuthorityMiddleware(next endpoint.Endpoint) endpoint.Endpoint {
	return func(ctx context.Context, request, response interface{}) error {
		// get and check password by reflect `Password` field
		reqV := reflect.ValueOf(request).MethodByName("GetReq").Call(nil)[0].Interface()
		fmt.Printf("%+v", reqV)
		if reflect.ValueOf(reqV).Elem().FieldByName("Password").IsValid() {
			commonPassword := reflect.ValueOf(reqV).Elem().FieldByName("Password").Interface()
			req, ok := commonPassword.(*common.Password)
			if ok {
				reqPassword := req.Password
				fmt.Println("req password is :" + reqPassword)
				if reqPassword != service.GetPassword() {
					return errors.New("password invalid")
				}
			}
		}
		err := next(ctx, request, response)
		return err
	}
}
