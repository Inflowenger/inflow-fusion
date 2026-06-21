package etc

import (
	"context"
	"fmt"
	"time"

	gohttpclient "github.com/bozd4g/go-http-client"
	"github.com/bytedance/sonic"
	json "github.com/bytedance/sonic"
)

func SendHttpPost(c context.Context, headers map[string]string, address string, data any) (*gohttpclient.Response, error) {
	opts := []gohttpclient.ClientOption{
		gohttpclient.WithDefaultHeaders(),
		gohttpclient.WithTimeout(time.Second * 3),
	}
	client := gohttpclient.New(address, opts...)
	json, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	reqOpts := []gohttpclient.Option{
		gohttpclient.WithBody(json),
		gohttpclient.WithHeader("Content-type", "application/json"),
	}
	for hKey, hv := range headers {
		reqOpts = append(reqOpts, gohttpclient.WithHeader(hKey, hv))
	}

	response, err := client.Post(c, "", reqOpts...)
	if err != nil {
		return nil, err
	}
	if response.Get().StatusCode != 200 {

		return nil, fmt.Errorf("registeration failed with status code %d , resposne: %s", response.Get().StatusCode,response.Body())
	}
	return response, err
}
func SendHttpGet[T any](c context.Context, headers map[string]string, address string, responseModel T) (*T, error) {
	opts := []gohttpclient.ClientOption{
		gohttpclient.WithDefaultHeaders(),
		gohttpclient.WithTimeout(time.Second * 5),
	}
	client := gohttpclient.New(address, opts...)

	reqOpts := []gohttpclient.Option{
		gohttpclient.WithHeader("Content-type", "application/json"),
	}
	for hKey, hv := range headers {
		reqOpts = append(reqOpts, gohttpclient.WithHeader(hKey, hv))
	}

	response, err := client.Get(c, "", reqOpts...)
	if err != nil {
		return nil, err
	}
	var data T
	sonic.Unmarshal(response.Body(), &data)
	return &data, err
}

