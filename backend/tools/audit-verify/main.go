// audit-verify 审计日志完整性核验工具:走哈希链 + 验 ECDSA 签名。
//
// 核验两件事(6 张审计表逐一):
//  1. 哈希链——按 id 序,prev_log_hash 必须等于前一行 log_hash(尾行无哈希
//     时为空串语义);prev 为 NULL 的行属"链启用前历史",跳过链校验。
//     链中任何篡改/删除/重排都会在此暴露。
//  2. ECDSA 签名——对携带签名的行,复现 BFF 中间件的签名内容
//     (sha256(JSON{tenant_id,user_id,username,sec,nanos,log_hash}),sec/nanos
//     恒为 0——签名发生在 core 落库之前,当时 created_at 尚未赋值),用
//     AUDIT_SIGN_KEY_FILE 私钥内的公钥验签。permission/policy 两表无
//     username 列,按无 username 形态验;其余表两种形态(含/不含 username)
//     任一通过即可——历史上两类签名内容并存。
//
// 用法:
//   DATABASE_DSN="postgres://postgres:***@127.0.0.1:5432/wind_ops" \
//     go run ./tools/audit-verify [-key data/audit-signing-key.pem] [-table sys_operation_audit_logs]
//
// 退出码:0=全部通过;1=存在断链/坏签名/无法读取。
package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strconv"

	"github.com/lib/pq"
)

// auditTable 核验目标:表名 + 用户标识列(permission/policy 用 operator_id)。
type auditTable struct {
	name    string
	userCol string
}

// errSkipMissing 表不存在时的跳过哨兵。
var errSkipMissing = errors.New("table missing")

var _ = pq.Error{}

var tables = []auditTable{
	{"sys_api_audit_logs", "user_id"},
	{"sys_login_audit_logs", "user_id"},
	{"sys_operation_audit_logs", "user_id"},
	{"sys_data_access_audit_logs", "user_id"},
	{"sys_permission_audit_logs", "operator_id"},
	{"sys_policy_evaluation_logs", "user_id"},
}

type row struct {
	id       int64
	tenantID uint32
	userID   uint32
	username string
	logHash  *string
	prevHash *string
	sig      []byte
}

type tableReport struct {
	name        string
	total       int
	legacy      int // prev 为 NULL:链启用前历史
	chainBroken []int64
	sigChecked  int
	sigFailed   []int64
}

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_DSN"), "postgres DSN(缺省读 DATABASE_DSN)")
	keyFile := flag.String("key", os.Getenv("AUDIT_SIGN_KEY_FILE"), "审计签名私钥 PEM(缺省读 AUDIT_SIGN_KEY_FILE/data/audit-signing-key.pem)")
	only := flag.String("table", "", "只核验指定表(缺省全部 6 张)")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "缺少 DSN:-dsn 或 DATABASE_DSN")
		os.Exit(1)
	}
	if *keyFile == "" {
		*keyFile = "data/audit-signing-key.pem"
	}

	pub := loadPublicKey(*keyFile)

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	if err = db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v\n", err)
		os.Exit(1)
	}

	failed := false
	for _, t := range tables {
		if *only != "" && t.name != *only {
			continue
		}
		rep, err := verifyTable(db, t, pub)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: 核验失败: %v\n", t.name, err)
			failed = true
			continue
		}
		rep.print()
		if len(rep.chainBroken) > 0 || len(rep.sigFailed) > 0 {
			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
	fmt.Println("\n全部通过。")
}

// verifyTable 走链 + 验签单表。
func verifyTable(db *sql.DB, t auditTable, pub *ecdsa.PublicKey) (*tableReport, error) {
	query := fmt.Sprintf(`
		SELECT id, COALESCE(tenant_id,0), COALESCE(%s,0), COALESCE(username,''),
		       log_hash, prev_log_hash, signature
		FROM %s ORDER BY id`, t.userCol, t.name)
	// permission/policy 两表无 username 列
	if t.userCol == "operator_id" {
		query = fmt.Sprintf(`
		SELECT id, COALESCE(tenant_id,0), COALESCE(%s,0), '',
		       log_hash, prev_log_hash, signature
		FROM %s ORDER BY id`, t.userCol, t.name)
	}

	rows, err := db.Query(query)
	if err != nil {
		// 表不存在(部分部署未挂全部审计面):跳过而非判失败
		var pe *pq.Error
		if errors.As(err, &pe) && pe.Code == "42P01" {
			return nil, errSkipMissing
		}
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	rep := &tableReport{name: t.name}
	var prev *row
	for rows.Next() {
		var r row
		if err = rows.Scan(&r.id, &r.tenantID, &r.userID, &r.username,
			&r.logHash, &r.prevHash, &r.sig); err != nil {
			return nil, err
		}
		rep.total++

		if r.prevHash == nil {
			rep.legacy++
		} else {
			expected := ""
			if prev != nil && prev.logHash != nil {
				expected = *prev.logHash
			}
			if *r.prevHash != expected {
				rep.chainBroken = append(rep.chainBroken, r.id)
			}
		}

		if len(r.sig) > 0 && pub != nil {
			rep.sigChecked++
			if !verifySignature(r, pub, t.userCol != "operator_id") {
				rep.sigFailed = append(rep.sigFailed, r.id)
			}
		}
		prev = &r
	}
	return rep, rows.Err()
}

// verifySignature 复现 BFF 中间件两代签名内容,任一匹配即通过。
// api 形态的结构体无 Username 键;login 形态恒含 Username 键(空串也序列化)。
// 两表无 username 列(permission/policy)只可能对应 api 形态。
func verifySignature(r row, pub *ecdsa.PublicKey, hasUsernameCol bool) bool {
	hashStr := ""
	if r.logHash != nil {
		hashStr = *r.logHash
	}

	// api 形态:无 Username 键
	apiForm, err := json.Marshal(struct {
		TenantID uint32 `json:"tenant_id"`
		UserID   uint32 `json:"user_id"`
		Sec      int64  `json:"sec"`
		Nanos    int32  `json:"nanos"`
		LogHash  string `json:"log_hash"`
	}{r.tenantID, r.userID, 0, 0, hashStr})
	if err == nil && checkSig(pub, apiForm, r.sig) {
		return true
	}

	// login 形态:恒含 Username 键
	if hasUsernameCol {
		loginForm, err := json.Marshal(struct {
			TenantID uint32 `json:"tenant_id"`
			UserID   uint32 `json:"user_id"`
			Username string `json:"username"`
			Sec      int64  `json:"sec"`
			Nanos    int32  `json:"nanos"`
			LogHash  string `json:"log_hash"`
		}{r.tenantID, r.userID, r.username, 0, 0, hashStr})
		if err == nil && checkSig(pub, loginForm, r.sig) {
			return true
		}
	}
	return false
}

// derSig ASN.1 ECDSA-Sig-Value。
type derSig struct {
	R, S *big.Int
}

func checkSig(pub *ecdsa.PublicKey, content, sig []byte) bool {
	digest := sha256.Sum256(content)
	var ds derSig
	if _, err := asn1.Unmarshal(sig, &ds); err != nil {
		return false
	}
	return ecdsa.Verify(pub, digest[:], ds.R, ds.S)
}

func loadPublicKey(path string) *ecdsa.PublicKey {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "未找到签名密钥 %s:跳过验签,只核验哈希链\n", path)
			return nil
		}
		fmt.Fprintf(os.Stderr, "读密钥失败 %s: %v(跳过验签)\n", path, err)
		return nil
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		fmt.Fprintf(os.Stderr, "密钥格式非 EC PRIVATE KEY PEM: %s(跳过验签)\n", path)
		return nil
	}
	priv, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析密钥失败: %v(跳过验签)\n", err)
		return nil
	}
	return &priv.PublicKey
}

func (r *tableReport) print() {
	ids := func(a []int64) string {
		out := ""
		for i, v := range a {
			if i >= 10 {
				out += fmt.Sprintf(" ...共 %d 处", len(a))
				break
			}
			if i > 0 {
				out += ","
			}
			out += strconv.FormatInt(v, 10)
		}
		return out
	}
	fmt.Printf("\n[%s]\n  总行数 %d(链前历史 %d,链上 %d)\n",
		r.name, r.total, r.legacy, r.total-r.legacy)
	if len(r.chainBroken) > 0 {
		fmt.Printf("  ✗ 断链 %d 处: id %s\n", len(r.chainBroken), ids(r.chainBroken))
	} else {
		fmt.Printf("  ✓ 哈希链完整\n")
	}
	if r.sigChecked > 0 {
		if len(r.sigFailed) > 0 {
			fmt.Printf("  ✗ 签名核验 %d 条,失败 %d: id %s\n",
				r.sigChecked, len(r.sigFailed), ids(r.sigFailed))
		} else {
			fmt.Printf("  ✓ 签名核验 %d 条全部通过\n", r.sigChecked)
		}
	} else {
		fmt.Printf("  - 无签名或未提供密钥,跳过验签\n")
	}
}
