package cloudprovider

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

// MonthBill 单账号单月账单总额(账号级,不细分服务)。
type MonthBill struct {
	Amount   float64
	Currency string
}

// GetMonthBill 拉取指定月份的账号账单总额(直接 upsert 进成本台账用)。
// month 形如 2026-09。华为云不支持(需 HWS 签名/SDK),显式报错。
func GetMonthBill(ctx context.Context, provider, ak, sk, month string) (*MonthBill, error) {
	switch provider {
	case "ALIYUN":
		return monthBillAliyun(ctx, ak, sk, month)
	case "TENCENT_CLOUD":
		return monthBillTencent(ctx, ak, sk, month)
	case "AWS":
		return monthBillAWS(ctx, ak, sk, month)
	case "HUAWEI_CLOUD":
		return nil, fmt.Errorf("华为云暂不支持账单拉取,请手工录入")
	default:
		return nil, fmt.Errorf("未知的云厂商: %s", provider)
	}
}

// ---------- 阿里云 ----------

// monthBillAliyun BSS QueryAccountBill(月账单,按 owner 条目求和)。
// BSS OpenAPI 域名是 business.aliyuncs.com(bss.aliyuncs.com 会报 InvalidVersion)。
func monthBillAliyun(ctx context.Context, ak, sk, month string) (*MonthBill, error) {
	query := aliyunSignedQuery(sk, aliyunSignedParams(ak, "QueryAccountBill", "2017-12-14", map[string]string{
		"BillingCycle": month,
		"Granularity":  "MONTH",
	}))
	body, status, err := httpGet(ctx, "https://business.aliyuncs.com/?"+query)
	if err != nil {
		return nil, fmt.Errorf("QueryAccountBill 请求失败: %w", err)
	}

	var resp struct {
		AccountBillList struct {
			Items []struct {
				PretaxAmount string `json:"PretaxAmount"`
				Currency     string `json:"Currency"`
				AccountName  string `json:"AccountName"`
			} `json:"Items"`
		} `json:"Data"`
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("账单响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if resp.Code != "" && resp.Code != "Success" {
		return nil, fmt.Errorf("阿里云拒绝: %s(%s)", resp.Message, resp.Code)
	}
	if len(resp.AccountBillList.Items) == 0 {
		return nil, fmt.Errorf("阿里云账单为空(月份 %s 可能未出账或无消费)", month)
	}

	bill := &MonthBill{}
	for _, it := range resp.AccountBillList.Items {
		v, pErr := strconv.ParseFloat(it.PretaxAmount, 64)
		if pErr != nil {
			continue
		}
		bill.Amount += v
		if it.Currency != "" {
			bill.Currency = it.Currency
		}
	}
	if bill.Currency == "" {
		bill.Currency = "CNY"
	}
	return bill, nil
}

// ---------- 腾讯云 ----------

// monthBillTencent Billing DescribeBillSummary(按月汇总,取账户总费用)。
func monthBillTencent(ctx context.Context, ak, sk, month string) (*MonthBill, error) {
	body, status, err := tc3Post(ctx, "billing.tencentcloudapi.com", "billing", ak, sk, "", "DescribeBillSummary", "2018-07-09", `{"Month":"`+month+`"}`)
	if err != nil {
		return nil, fmt.Errorf("DescribeBillSummary 请求失败: %w", err)
	}

	var resp struct {
		Response struct {
			TotalCost       json.Number `json:"TotalCost"`
			SummaryOverview *struct {
				TotalCost json.Number `json:"TotalCost"`
			} `json:"SummaryOverview"`
			Error *tencentError `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("账单响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	if resp.Response.Error != nil {
		return nil, fmt.Errorf("腾讯云拒绝: %s(%s)", resp.Response.Error.Message, resp.Response.Error.Code)
	}

	// 优先取账户总费用字段,形状变化时给出明确提示
	amount := 0.0
	switch {
	case resp.Response.TotalCost != "":
		amount, _ = resp.Response.TotalCost.Float64()
	case resp.Response.SummaryOverview != nil:
		amount, _ = resp.Response.SummaryOverview.TotalCost.Float64()
	default:
		return nil, fmt.Errorf("腾讯云账单响应缺少 TotalCost 字段(HTTP %d): %s", status, truncate(body))
	}
	// 腾讯国内站计费币种为 CNY
	return &MonthBill{Amount: amount, Currency: "CNY"}, nil
}

// ---------- AWS ----------

// monthBillAWS Cost Explorer GetCostAndUsage(月度 UnblendedCost 总额)。
// CE 仅 JSON RPC(POST),全局端点 us-east-1;TimePeriod End 为开区间。
func monthBillAWS(ctx context.Context, ak, sk, month string) (*MonthBill, error) {
	start := month + "-01"
	end := nextMonth(month) + "-01"
	payload := `{"TimePeriod":{"Start":"` + start + `","End":"` + end + `"},"Granularity":"MONTHLY","Metrics":["UnblendedCost"]}`

	body, status, err := awsSigV4Post(ctx, ak, sk, "us-east-1", "ce", "AWSCostExplorerService.GetCostAndUsage", payload)
	if err != nil {
		return nil, fmt.Errorf("GetCostAndUsage 请求失败: %w", err)
	}

	var resp struct {
		ResultsByTime []struct {
			Total map[string]struct {
				Amount string `json:"Amount"`
				Unit   string `json:"Unit"`
			} `json:"Total"`
		} `json:"ResultsByTime"`
		Error struct {
			Code    string `json:"__type"`
			Message string `json:"Message"`
		} `json:"-"`
	}
	if jsonErr := json.Unmarshal(body, &resp); jsonErr != nil {
		return nil, fmt.Errorf("账单响应解析失败(HTTP %d): %s", status, truncate(body))
	}
	// CE 错误形如 {"__type":"...AccessDeniedException","Message":"..."}
	if code := jsonGet(body, "__type"); code != "" {
		return nil, fmt.Errorf("AWS 拒绝: %s(%s)", jsonGet(body, "Message"), code)
	}
	if len(resp.ResultsByTime) == 0 || resp.ResultsByTime[0].Total == nil {
		return nil, fmt.Errorf("AWS 账单为空(月份 %s 可能未出账或 Cost Explorer 未启用)", month)
	}

	metric, ok := resp.ResultsByTime[0].Total["UnblendedCost"]
	if !ok {
		return nil, fmt.Errorf("AWS 账单响应缺少 UnblendedCost(HTTP %d): %s", status, truncate(body))
	}
	amount, pErr := strconv.ParseFloat(metric.Amount, 64)
	if pErr != nil {
		return nil, fmt.Errorf("AWS 金额解析失败: %s", metric.Amount)
	}
	return &MonthBill{Amount: amount, Currency: metric.Unit}, nil
}

// awsSigV4Post SigV4 签名 POST(JSON RPC 协议:body 参与签名,Target 头寻址)。
func awsSigV4Post(ctx context.Context, ak, sk, region, service, target, payload string) ([]byte, int, error) {
	host := service + "." + region + ".amazonaws.com"

	amzDate := nowUTC().Format("20060102T150405Z")
	date := amzDate[:8]

	canonicalHeaders := "content-type:application/x-amz-json-1.1\nhost:" + host + "\nx-amz-date:" + amzDate + "\nx-amz-target:" + target + "\n"
	signedHeaders := "content-type;host;x-amz-date;x-amz-target"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + sha256Hex([]byte(payload))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + date + "/" + region + "/" + service + "/aws4_request\n" + sha256Hex([]byte(canonicalRequest))

	kDate := hmacSHA256([]byte("AWS4"+sk), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authorization := "AWS4-HMAC-SHA256 Credential=" + ak + "/" + date + "/" + region + "/" + service + "/aws4_request" +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature

	return httpDo(ctx, "POST", "https://"+host+"/", payload, map[string]string{
		"Content-Type":  "application/x-amz-json-1.1",
		"X-Amz-Date":    amzDate,
		"X-Amz-Target":  target,
		"Authorization": authorization,
	})
}

// ---------- 小工具 ----------

// nextMonth 2026-09 → 2026-10(跨年进位)。
func nextMonth(month string) string {
	y, _ := strconv.Atoi(month[:4])
	m, _ := strconv.Atoi(month[5:7])
	m++
	if m > 12 {
		y++
		m = 1
	}
	return fmt.Sprintf("%04d-%02d", y, m)
}

// jsonGet 顶层字符串字段极简取值(CE 错误体足够)。
func jsonGet(data []byte, field string) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	if v, ok := m[field].(string); ok {
		return v
	}
	return ""
}
