package http

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

func GetRequest(url string) (body []byte, err error) {
	return request(url, "GET", nil)
}

func PutRequest(url string, jsonData []byte) ([]byte, error) {
	return request(url, "PUT", jsonData)
}

func request(url, method string, jsonData []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if resp == nil {
		return nil, fmt.Errorf("get %s response body is empty", url)
	}
	if err != nil {
		return nil, fmt.Errorf("request %s failed: %v", url, err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			klog.Error(err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("io read error: %v", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code %d, details: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
