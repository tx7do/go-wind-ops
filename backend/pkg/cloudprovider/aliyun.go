package cloudprovider

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// verifyAliyun 阿里云:RPC 签名(HMAC-SHA1)调 STS GetCallerIdentity。
// 签名串 = GET&%2F&percentEncode(排序后 query);密钥 = SK + "&"。
func verifyAliyun(ctx context.Context, ak, sk string) (*VerifyResult, error) {
	query := aliyunSignedQuery(sk, aliyunSignedParams(ak, "GetCallerIdentity", "2015-04-01", nil))
	reqURL := "https://sts.aliyuncs.com/?" + query

	body, status, err := httpGet(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("阿里云校验请求失败: %w", err)
	}

	var resp struct {
		AccountID string `json:"AccountId"`
		Arn       string `json:"Arn"`
		Code      string `json:"Code"`
		Message   string `json:"Message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("阿里云响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if resp.Code != "" {
		return nil, fmt.Errorf("阿里云拒绝: %s(%s)", resp.Message, resp.Code)
	}
	if resp.AccountID == "" {
		return nil, fmt.Errorf("阿里云校验未返回账号ID(HTTP %d): %s", status, truncate(body))
	}
	return &VerifyResult{AccountID: resp.AccountID, AccountName: arnTail(resp.Arn)}, nil
}

// percentEncode 阿里云 RPC 规范编码:QueryEscape 后 +→%20、*→%2A、%7E→~。
func percentEncode(s string) string {
	s = url.QueryEscape(s)
	s = strings.ReplaceAll(s, "+", "%20")
	s = strings.ReplaceAll(s, "*", "%2A")
	s = strings.ReplaceAll(s, "%7E", "~")
	return s
}

// verifyTencentCloud 腾讯云:TC3-HMAC-SHA256 签名调 STS GetCallerIdentity。
func verifyTencentCloud(ctx context.Context, ak, sk string) (*VerifyResult, error) {
	body, status, err := tc3Post(ctx, "sts.tencentcloudapi.com", "sts", ak, sk, "", "GetCallerIdentity", "2018-08-13", "{}")
	if err != nil {
		return nil, fmt.Errorf("腾讯云校验请求失败: %w", err)
	}

	var resp struct {
		Response struct {
			AccountID any           `json:"AccountId"`
			Arn       string        `json:"Arn"`
			Error     *tencentError `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("腾讯云响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if resp.Response.Error != nil {
		return nil, fmt.Errorf("腾讯云拒绝: %s(%s)", resp.Response.Error.Message, resp.Response.Error.Code)
	}
	accountID := fmt.Sprintf("%v", resp.Response.AccountID)
	if accountID == "" || accountID == "<nil>" {
		return nil, fmt.Errorf("腾讯云校验未返回账号ID(HTTP %d): %s", status, truncate(body))
	}
	return &VerifyResult{AccountID: accountID, AccountName: arnTail(resp.Response.Arn)}, nil
}

// tc3Post TC3-HMAC-SHA256 签名 POST(region 为空表示全局 API)。
// service 为签名 Credential 的服务段(sts/cvm/billing/...),随产品而变,由调用方给出。
func tc3Post(ctx context.Context, host, service, ak, sk, region, action, version, payload string) ([]byte, int, error) {
	ts := nowUTC().Unix()
	// TC3 的 Credential 日期段为 ISO 格式(带横杠,如 2026-10-19),与 AWS 的 YYYYMMDD 不同
	date := nowUTC().Format("2006-01-02")

	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host +
		"\nx-tc-action:" + strings.ToLower(action) + "\n"
	signedHeaders := "content-type;host;x-tc-action"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + sha256Hex([]byte(payload))
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(ts, 10) + "\n" + date + "/" + service + "/tc3_request\n" + sha256Hex([]byte(canonicalRequest))

	kDate := hmacSHA256([]byte("TC3"+sk), []byte(date))
	kService := hmacSHA256(kDate, []byte(service))
	kSigning := hmacSHA256(kService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authorization := "TC3-HMAC-SHA256 Credential=" + ak + "/" + date + "/" + service + "/tc3_request" +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature

	headers := map[string]string{
		"Content-Type":   "application/json; charset=utf-8",
		"X-TC-Action":    action,
		"X-TC-Version":   version,
		"X-TC-Timestamp": strconv.FormatInt(ts, 10),
		"Authorization":  authorization,
	}
	if region != "" {
		headers["X-TC-Region"] = region
	}
	return httpDo(ctx, "POST", "https://"+host+"/", payload, headers)
}

// verifyAWS 亚马逊云:SigV4 签名调 STS GetCallerIdentity(query 协议,XML 响应)。
func verifyAWS(ctx context.Context, ak, sk, region string) (*VerifyResult, error) {
	if region == "" {
		region = "us-east-1"
	}
	body, status, err := awsSigV4Get(ctx, ak, sk, region, "sts", "Action=GetCallerIdentity&Version=2011-06-15")
	if err != nil {
		return nil, fmt.Errorf("AWS 校验请求失败: %w", err)
	}

	accountID := xmlTagValue(body, "Account")
	if code := xmlTagValue(body, "Code"); code != "" {
		return nil, fmt.Errorf("AWS 拒绝: %s(%s)", xmlTagValue(body, "Message"), code)
	}
	if accountID == "" {
		return nil, fmt.Errorf("AWS 校验未返回账号ID(HTTP %d): %s", status, truncate(body))
	}
	return &VerifyResult{AccountID: accountID, AccountName: arnTail(xmlTagValue(body, "Arn"))}, nil
}

// awsSigV4Get SigV4 签名 GET(query 协议,XML 响应)。
func awsSigV4Get(ctx context.Context, ak, sk, region, service, query string) ([]byte, int, error) {
	host := service + "." + region + ".amazonaws.com"

	amzDate := nowUTC().Format("20060102T150405Z")
	date := amzDate[:8]

	canonicalRequest := "GET\n/\n" + query + "\nhost:" + host + "\nx-amz-date:" + amzDate + "\n\nhost;x-amz-date\n" + sha256Hex(nil)
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + date + "/" + region + "/" + service + "/aws4_request\n" + sha256Hex([]byte(canonicalRequest))

	kDate := hmacSHA256([]byte("AWS4"+sk), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authorization := "AWS4-HMAC-SHA256 Credential=" + ak + "/" + date + "/" + region + "/" + service + "/aws4_request" +
		", SignedHeaders=host;x-amz-date" + ", Signature=" + signature

	return httpDo(ctx, "GET", "https://"+host+"/?"+query, "", map[string]string{
		"X-Amz-Date":    amzDate,
		"Authorization": authorization,
	})
}

// ---------- 小工具 ----------

func httpGet(ctx context.Context, url string) ([]byte, int, error) {
	return httpDo(ctx, "GET", url, "", nil)
}

func httpDo(ctx context.Context, method, url, body string, headers map[string]string) ([]byte, int, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// arnTail 从 ARN(acs:ram::1234:root / arn:aws:iam::1234:root)提取可读尾段,无则空。
func arnTail(arn string) string {
	if arn == "" {
		return ""
	}
	if i := strings.LastIndex(arn, ":"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}

// xmlTagValue 极简 XML 取值(取首个命中,不递归),STS 错误/结果节点足够。
func xmlTagValue(data []byte, tag string) string {
	open, close := "<"+tag+">", "</"+tag+">"
	s := string(data)
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func truncate(data []byte) string {
	s := string(data)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// sortStrings 原地排序(签名 canonical query 用)。
func sortStrings(s []string) {
	sort.Strings(s)
}
