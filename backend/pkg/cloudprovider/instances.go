package cloudprovider

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// Instance 云资源实例(厂商无关快照)。
type Instance struct {
	Region       string
	Zone         string
	InstanceID   string
	InstanceName string
	Status       string
	InstanceType string
	PrivateIP    string
	PublicIP     string
	ExpiredAt    *time.Time
}

// DefaultRegion 各厂商地域枚举/清单 API 的兜底入口地域。
func DefaultRegion(provider string) string {
	switch provider {
	case "ALIYUN":
		return "cn-hangzhou"
	case "TENCENT_CLOUD":
		return "ap-guangzhou"
	case "AWS":
		return "us-east-1"
	default:
		return ""
	}
}

// DescribeRegions 枚举账号可用地域(华为云不支持,返回明确错误)。
func DescribeRegions(ctx context.Context, provider, ak, sk, baseRegion string) ([]string, error) {
	switch provider {
	case "ALIYUN":
		return describeRegionsAliyun(ctx, ak, sk, baseRegion)
	case "TENCENT_CLOUD":
		return describeRegionsTencent(ctx, ak, sk)
	case "AWS":
		return describeRegionsAWS(ctx, ak, sk, baseRegion)
	case "HUAWEI_CLOUD":
		return nil, fmt.Errorf("华为云暂不支持资源同步,请通过控制台查看实例清单")
	default:
		return nil, fmt.Errorf("未知的云厂商: %s", provider)
	}
}

// ListInstancesInRegion 拉取单地域实例清单(单页,常见规模 < 100/地域;超出部分暂截断)。
func ListInstancesInRegion(ctx context.Context, provider, ak, sk, region string) ([]Instance, error) {
	switch provider {
	case "ALIYUN":
		return listInstancesAliyun(ctx, ak, sk, region)
	case "TENCENT_CLOUD":
		return listInstancesTencent(ctx, ak, sk, region)
	case "AWS":
		return listInstancesAWS(ctx, ak, sk, region)
	default:
		return nil, fmt.Errorf("该云厂商暂不支持资源同步")
	}
}

// ---------- 阿里云 ----------

// describeRegionsAliyun ECS DescribeRegions(RPC 签名,同 verify 的签名串)。
func describeRegionsAliyun(ctx context.Context, ak, sk, baseRegion string) ([]string, error) {
	if baseRegion == "" {
		baseRegion = DefaultRegion("ALIYUN")
	}
	params := aliyunSignedParams(ak, "DescribeRegions", "2014-05-26", map[string]string{})
	reqURL := "https://ecs." + baseRegion + ".aliyuncs.com/?" + aliyunSignedQuery(sk, params)
	body, _, err := httpGet(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("DescribeRegions 请求失败: %w", err)
	}
	var resp struct {
		Regions struct {
			Region []struct {
				RegionId string `json:"RegionId"`
			} `json:"Region"`
		} `json:"Regions"`
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("DescribeRegions 响应解析失败: %s", truncate(body))
	}
	if resp.Code != "" {
		return nil, fmt.Errorf("阿里云拒绝: %s(%s)", resp.Message, resp.Code)
	}
	ids := make([]string, 0, len(resp.Regions.Region))
	for _, r := range resp.Regions.Region {
		if r.RegionId != "" {
			ids = append(ids, r.RegionId)
		}
	}
	return ids, nil
}

// listInstancesAliyun ECS DescribeInstances 单页(PageSize=100)。
func listInstancesAliyun(ctx context.Context, ak, sk, region string) ([]Instance, error) {
	params := aliyunSignedParams(ak, "DescribeInstances", "2014-05-26", map[string]string{
		"RegionId":   region,
		"PageSize":   "100",
		"PageNumber": "1",
	})
	reqURL := "https://ecs." + region + ".aliyuncs.com/?" + aliyunSignedQuery(sk, params)
	body, _, err := httpGet(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("DescribeInstances 请求失败: %w", err)
	}
	var resp struct {
		Instances struct {
			Instance []struct {
				InstanceId   string `json:"InstanceId"`
				InstanceName string `json:"InstanceName"`
				Status       string `json:"Status"`
				InstanceType string `json:"InstanceType"`
				ZoneId       string `json:"ZoneId"`
				ExpiredTime  string `json:"ExpiredTime"`
				Vpc          struct {
					PrivateIpAddress struct {
						IpAddress []string `json:"IpAddress"`
					} `json:"PrivateIpAddress"`
				} `json:"VpcAttributes"`
				Public struct {
					IpAddress []string `json:"IpAddress"`
				} `json:"PublicIpAddress"`
				Eip struct {
					IpAddress string `json:"IpAddress"`
				} `json:"EipAddress"`
			} `json:"Instance"`
		} `json:"Instances"`
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("响应解析失败: %s", truncate(body))
	}
	if resp.Code != "" {
		return nil, fmt.Errorf("阿里云拒绝: %s(%s)", resp.Message, resp.Code)
	}

	out := make([]Instance, 0, len(resp.Instances.Instance))
	for _, it := range resp.Instances.Instance {
		ins := Instance{
			Region:       region,
			InstanceID:   it.InstanceId,
			InstanceName: it.InstanceName,
			Status:       it.Status,
			InstanceType: it.InstanceType,
			Zone:         it.ZoneId,
		}
		if len(it.Vpc.PrivateIpAddress.IpAddress) > 0 {
			ins.PrivateIP = it.Vpc.PrivateIpAddress.IpAddress[0]
		}
		if len(it.Public.IpAddress) > 0 {
			ins.PublicIP = it.Public.IpAddress[0]
		}
		if ins.PublicIP == "" {
			ins.PublicIP = it.Eip.IpAddress
		}
		ins.ExpiredAt = parseCloudTime(it.ExpiredTime)
		out = append(out, ins)
	}
	return out, nil
}

// aliyunSignedParams 构造带公共参数的待签名 query(不含 Signature)。
func aliyunSignedParams(ak, action, version string, extra map[string]string) map[string]string {
	params := map[string]string{
		"AccessKeyId":      ak,
		"Action":           action,
		"Format":           "JSON",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   randomNonce(),
		"SignatureVersion": "1.0",
		"Timestamp":        nowUTC().Format("2006-01-02T15:04:05Z"),
		"Version":          version,
	}
	for k, v := range extra {
		params[k] = v
	}
	return params
}

// aliyunSignedQuery 排序拼接 + 计算签名,返回可直接拼接的 query 串。
func aliyunSignedQuery(sk string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sortStrings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, percentEncode(k)+"="+percentEncode(params[k]))
	}
	canonical := strings.Join(pairs, "&")
	signature := hmacSHA1Base64([]byte(sk+"&"), []byte("GET&%2F&"+percentEncode(canonical)))
	return canonical + "&Signature=" + percentEncode(signature)
}

// ---------- 腾讯云 ----------

// describeRegionsTencent CVM DescribeRegions(全局 API,无需地域头)。
func describeRegionsTencent(ctx context.Context, ak, sk string) ([]string, error) {
	body, status, err := tc3Post(ctx, "cvm.tencentcloudapi.com", "cvm", ak, sk, "", "DescribeRegions", "2017-03-12", "{}")
	if err != nil {
		return nil, fmt.Errorf("DescribeRegions 请求失败: %w", err)
	}
	var resp struct {
		Response struct {
			RegionSet []struct {
				Region string `json:"Region"`
			} `json:"RegionSet"`
			Error *tencentError `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("DescribeRegions 响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if resp.Response.Error != nil {
		return nil, fmt.Errorf("腾讯云拒绝: %s(%s)", resp.Response.Error.Message, resp.Response.Error.Code)
	}
	ids := make([]string, 0, len(resp.Response.RegionSet))
	for _, r := range resp.Response.RegionSet {
		if r.Region != "" {
			ids = append(ids, r.Region)
		}
	}
	return ids, nil
}

// listInstancesTencent CVM DescribeInstances 单页(Limit=100)。
func listInstancesTencent(ctx context.Context, ak, sk, region string) ([]Instance, error) {
	body, status, err := tc3Post(ctx, "cvm.tencentcloudapi.com", "cvm", ak, sk, region, "DescribeInstances", "2017-03-12", `{"Limit":100,"Offset":0}`)
	if err != nil {
		return nil, fmt.Errorf("DescribeInstances 请求失败: %w", err)
	}
	var resp struct {
		Response struct {
			InstanceSet []struct {
				InstanceId         string   `json:"InstanceId"`
				InstanceName       string   `json:"InstanceName"`
				InstanceState      string   `json:"InstanceState"`
				InstanceType       string   `json:"InstanceType"`
				ExpiredTime        string   `json:"ExpiredTime"`
				PrivateIpAddresses []string `json:"PrivateIpAddresses"`
				PublicIpAddresses  []string `json:"PublicIpAddresses"`
				Placement          struct {
					Zone string `json:"Zone"`
				} `json:"Placement"`
			} `json:"InstanceSet"`
			Error *tencentError `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if resp.Response.Error != nil {
		return nil, fmt.Errorf("腾讯云拒绝: %s(%s)", resp.Response.Error.Message, resp.Response.Error.Code)
	}

	out := make([]Instance, 0, len(resp.Response.InstanceSet))
	for _, it := range resp.Response.InstanceSet {
		ins := Instance{
			Region:       region,
			InstanceID:   it.InstanceId,
			InstanceName: it.InstanceName,
			Status:       it.InstanceState,
			InstanceType: it.InstanceType,
			Zone:         it.Placement.Zone,
		}
		if len(it.PrivateIpAddresses) > 0 {
			ins.PrivateIP = it.PrivateIpAddresses[0]
		}
		if len(it.PublicIpAddresses) > 0 {
			ins.PublicIP = it.PublicIpAddresses[0]
		}
		ins.ExpiredAt = parseCloudTime(it.ExpiredTime)
		out = append(out, ins)
	}
	return out, nil
}

type tencentError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

// ---------- AWS ----------

// describeRegionsAWS EC2 DescribeRegions(XML)。
func describeRegionsAWS(ctx context.Context, ak, sk, baseRegion string) ([]string, error) {
	if baseRegion == "" {
		baseRegion = DefaultRegion("AWS")
	}
	query := "Action=DescribeRegions&Version=2016-11-15"
	body, status, err := awsSigV4Get(ctx, ak, sk, baseRegion, "ec2", query)
	if err != nil {
		return nil, fmt.Errorf("DescribeRegions 请求失败: %w", err)
	}
	var resp struct {
		RegionInfo struct {
			Items []struct {
				RegionName string `xml:"regionName"`
			} `xml:"regionInfo"`
		} `xml:"regions"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("DescribeRegions 响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if code := xmlTagValue(body, "Code"); code != "" {
		return nil, fmt.Errorf("AWS 拒绝: %s(%s)", xmlTagValue(body, "Message"), code)
	}
	ids := make([]string, 0, len(resp.RegionInfo.Items))
	for _, r := range resp.RegionInfo.Items {
		if r.RegionName != "" {
			ids = append(ids, r.RegionName)
		}
	}
	return ids, nil
}

// listInstancesAWS EC2 DescribeInstances(XML,单响应,忽略 NextToken 截断)。
func listInstancesAWS(ctx context.Context, ak, sk, region string) ([]Instance, error) {
	query := "Action=DescribeInstances&Version=2016-11-15"
	body, status, err := awsSigV4Get(ctx, ak, sk, region, "ec2", query)
	if err != nil {
		return nil, fmt.Errorf("DescribeInstances 请求失败: %w", err)
	}
	if code := xmlTagValue(body, "Code"); code != "" {
		return nil, fmt.Errorf("AWS 拒绝: %s(%s)", xmlTagValue(body, "Message"), code)
	}

	var resp struct {
		Reservations []struct {
			Instances []struct {
				InstanceID   string `xml:"instanceId"`
				InstanceType string `xml:"instanceType"`
				PrivateIP    string `xml:"privateIpAddress"`
				PublicIP     string `xml:"ipAddress"`
				Placement    struct {
					AZ string `xml:"availabilityZone"`
				} `xml:"placement"`
				State struct {
					Name string `xml:"name"`
				} `xml:"instanceState"`
				Tags []struct {
					Key   string `xml:"key"`
					Value string `xml:"value"`
				} `xml:"tagSet>item"`
			} `xml:"instancesSet>item"`
		} `xml:"reservationSet>item"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("响应解析失败(HTTP %d): %s", status, truncate(body))
	}

	out := make([]Instance, 0)
	for _, r := range resp.Reservations {
		for _, it := range r.Instances {
			name := ""
			for _, tg := range it.Tags {
				if tg.Key == "Name" {
					name = tg.Value
					break
				}
			}
			state := it.State.Name
			if state != "" {
				state = strings.ToUpper(state[:1]) + state[1:] // running → Running 对齐其他厂商
			}
			out = append(out, Instance{
				Region:       region,
				Zone:         it.Placement.AZ,
				InstanceID:   it.InstanceID,
				InstanceName: name,
				Status:       state,
				InstanceType: it.InstanceType,
				PrivateIP:    it.PrivateIP,
				PublicIP:     it.PublicIP,
			})
		}
	}
	return out, nil
}

// ---------- 小工具 ----------

// parseCloudTime 解析厂商时间串,零值/非法返回 nil。
func parseCloudTime(s string) *time.Time {
	if s == "" || strings.HasPrefix(s, "0000-00-00") {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
