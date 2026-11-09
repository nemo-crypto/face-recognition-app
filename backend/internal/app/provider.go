package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"
)

type Result struct {
	Decision string
	Score    *float64
	Message  string
}
type Provider interface {
	Mode() string
	Verify(context.Context, string, string) (Result, error)
}
type DemoProvider struct{}

func (DemoProvider) Mode() string { return "demo" }
func (DemoProvider) Verify(ctx context.Context, _, _ string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return Result{Decision: "demo", Message: "演示流程已完成，未进行身份识别或活体检测。"}, nil
}

type RemoteProvider struct {
	URL, Token string
	Threshold  float64
	Client     *http.Client
}

func NewRemoteProvider(endpoint, token string, threshold float64) (*RemoteProvider, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("模型地址必须是无凭据、无查询参数的 HTTP(S) URL")
	}
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1 {
		return nil, errors.New("模型阈值必须在 0..1 内")
	}
	return &RemoteProvider{URL: endpoint, Token: token, Threshold: threshold, Client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (*RemoteProvider) Mode() string { return "remote" }
func (p *RemoteProvider) Verify(ctx context.Context, reference, probe string) (Result, error) {
	body, _ := json.Marshal(map[string]string{"reference_image": reference, "probe_image": probe})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return Result{}, errors.New("模型请求创建失败")
	}
	req.Header.Set("Content-Type", "application/json")
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return Result{}, errors.New("模型服务不可用或超时")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, errors.New("模型服务返回异常状态")
	}
	var v struct {
		Matched   *bool    `json:"matched"`
		Score     *float64 `json:"score"`
		FaceCount *int     `json:"face_count"`
		Liveness  string   `json:"liveness"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 65536))
	if dec.Decode(&v) != nil || dec.Decode(new(any)) != io.EOF || v.Matched == nil || v.Score == nil || v.FaceCount == nil || *v.FaceCount < 0 || math.IsNaN(*v.Score) || math.IsInf(*v.Score, 0) || *v.Score < 0 || *v.Score > 1 {
		return Result{}, errors.New("模型响应不符合协议")
	}
	if v.Liveness != "passed" && v.Liveness != "failed" && v.Liveness != "unknown" {
		return Result{}, errors.New("模型活体状态无效")
	}
	if *v.FaceCount != 1 || v.Liveness != "passed" {
		return Result{Decision: "review", Message: "单人脸或活体条件未满足，请人工复核。"}, nil
	}
	if !*v.Matched || *v.Score < p.Threshold {
		return Result{Decision: "not_matched", Score: v.Score, Message: "模型比对未通过，请重新采集或人工复核。"}, nil
	}
	return Result{Decision: "matched", Score: v.Score, Message: "模型比对通过；此结果仅用于当前 Demo。"}, nil
}
