package etc

import (
	"context"
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

	return  client.Post(c, "", reqOpts...)
	// if err != nil {
	// 	return nil, err
	// }
	// if response.Get().StatusCode != 200 {

	// 	return nil, fmt.Errorf("registeration failed with status code %d , resposne: %s", response.Get().StatusCode,response.Body())
	// }
	// return response, err
}
// SendHttpGetRaw performs a GET and returns the raw response so the caller can
// inspect the status code — unlike SendHttpGet, which only unmarshals the body.
// It is what the resource liveness probe uses to tell a healthy resource (200)
// from one that is reachable but wrong (auth drift, a different service on the
// host); a transport failure (unreachable/moved host) returns an error.
func SendHttpGetRaw(c context.Context, headers map[string]string, address string, timeout time.Duration) (*gohttpclient.Response, error) {
	opts := []gohttpclient.ClientOption{
		gohttpclient.WithDefaultHeaders(),
		gohttpclient.WithTimeout(timeout),
	}
	client := gohttpclient.New(address, opts...)
	reqOpts := []gohttpclient.Option{
		gohttpclient.WithHeader("Content-type", "application/json"),
	}
	for hKey, hv := range headers {
		reqOpts = append(reqOpts, gohttpclient.WithHeader(hKey, hv))
	}
	return client.Get(c, "", reqOpts...)
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

