// terminal_recorder.go 终端会话录制器:输入/输出经带缓冲通道异步批量落库,
// 刷盘失败仅记日志——录制永远不影响终端本身。单会话录制上限见
// terminalRecordMaxBytes,超出后置 truncated 并停止收集。
package service

import (
	"context"
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-ops/app/core/service/internal/data"
	"go-wind-ops/app/core/service/internal/data/ent"
	"go-wind-ops/app/core/service/internal/data/ent/terminalchunk"
	appViewer "go-wind-ops/pkg/entgo/viewer"
)

const (
	// terminalRecordMaxBytes 单会话录制字节上限(输入+输出合计)。
	terminalRecordMaxBytes = 8 << 20
	// terminalRecordFlushInterval 批量落库冲刷间隔。
	terminalRecordFlushInterval = 800 * time.Millisecond
	// terminalRecordQueueDepth 单会话待写分块队列深度(满即丢,终端可容忍)。
	terminalRecordQueueDepth = 1024
)

// ── 录制策略(环境变量,启动时读取一次) ──
//
// 终端录制含键盘按键明文(可能带用户在 shell 里敲的密码),属敏感留存:
//   TERMINAL_RECORD_ENABLED=false        整体关闭录制(默认开)
//   TERMINAL_RECORD_INPUT=false          不录键盘输入(默认录)
//   TERMINAL_RECORD_OUTPUT=false         不录回显输出(默认录)
//   TERMINAL_RETENTION_DAYS=30           录制留存天数,0=永久(默认 30)

type terminalRecordPolicy struct {
	enabled bool
	input   bool
	output  bool
}

var (
	terminalRecordPolicyOnce  sync.Once
	terminalRecordPolicyValue terminalRecordPolicy
)

func getTerminalRecordPolicy() terminalRecordPolicy {
	terminalRecordPolicyOnce.Do(func() {
		terminalRecordPolicyValue = terminalRecordPolicy{
			enabled: envBool("TERMINAL_RECORD_ENABLED", true),
			input:   envBool("TERMINAL_RECORD_INPUT", true),
			output:  envBool("TERMINAL_RECORD_OUTPUT", true),
		}
	})
	return terminalRecordPolicyValue
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// TerminalRecorder 单个活跃终端会话的录制器实例。
// 由 TerminalService.Channel 在会话建立时构造,流结束时 Close。
type TerminalRecorder struct {
	sessionRowID uint32
	// tenantID 会话归属租户:分块由后台 goroutine 异步落库,显式带上,
	// 否则落 tenant 0 租户用户看不到回放。
	tenantID    uint32
	sessionRepo *data.TerminalSessionRepo
	chunkRepo   *data.TerminalChunkRepo
	log         *log.Helper

	policy terminalRecordPolicy

	seq       atomic.Int64
	inputB    atomic.Int64
	outputB   atomic.Int64
	chunkCnt  atomic.Int64
	truncated atomic.Bool

	pending chan *ent.TerminalChunk

	mu     sync.Mutex
	closed bool
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewTerminalRecorder 建会话行并启动异步刷盘 goroutine;整体录制关闭或
// 建行失败返回 nil(调用方判空跳过录制)。
func NewTerminalRecorder(ctx context.Context, sessionRepo *data.TerminalSessionRepo, chunkRepo *data.TerminalChunkRepo, log *log.Helper, sessionId, agentId string, nodeID, operatorID uint32) *TerminalRecorder {
	if sessionRepo == nil || chunkRepo == nil {
		return nil
	}
	if !getTerminalRecordPolicy().enabled {
		return nil
	}
	row, err := sessionRepo.Create(ctx, sessionId, agentId, nodeID, operatorID)
	if err != nil || row == nil {
		log.Warnf("terminal record: create session row failed (session=%s): %v", sessionId, err)
		return nil
	}
	rec := &TerminalRecorder{
		sessionRowID: row.ID,
		tenantID:     uint32Val(row.TenantID),
		sessionRepo:  sessionRepo,
		chunkRepo:    chunkRepo,
		log:          log,
		policy:       getTerminalRecordPolicy(),
		pending:      make(chan *ent.TerminalChunk, terminalRecordQueueDepth),
		done:         make(chan struct{}),
	}
	rec.wg.Add(1)
	go rec.flushLoop()
	return rec
}

// Input 记录一段键盘输入。
func (r *TerminalRecorder) Input(data []byte) { r.record(terminalchunk.DirectionInput, data) }

// Output 记录一段回显输出。
func (r *TerminalRecorder) Output(data []byte) { r.record(terminalchunk.DirectionOutput, data) }

func (r *TerminalRecorder) record(dir terminalchunk.Direction, data []byte) {
	if r == nil || len(data) == 0 || r.truncated.Load() {
		return
	}
	if dir == terminalchunk.DirectionInput && !r.policy.input {
		return
	}
	if dir == terminalchunk.DirectionOutput && !r.policy.output {
		return
	}
	total := r.inputB.Load() + r.outputB.Load() + int64(len(data))
	if total > terminalRecordMaxBytes {
		r.truncated.Store(true)
		return
	}
	if dir == terminalchunk.DirectionInput {
		r.inputB.Add(int64(len(data)))
	} else {
		r.outputB.Add(int64(len(data)))
	}
	r.chunkCnt.Add(1)
	select {
	case r.pending <- &ent.TerminalChunk{
		TerminalSessionID: r.sessionRowID,
		Seq:               int(r.seq.Add(1)) - 1,
		Direction:         dir,
		Payload:           base64.StdEncoding.EncodeToString(data),
		ByteLen:           len(data),
	}:
	default: // 队列满即丢帧
	}
}

// Close 会话结束:停刷盘循环,落剩余分块并终态化会话行(幂等)。
func (r *TerminalRecorder) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.done)
	r.mu.Unlock()
	r.wg.Wait()

	// 会话表已挂租户隐私:终态化注入系统 viewer(平台视图绕过租户过滤)
	ctx, cancel := context.WithTimeout(appViewer.NewSystemViewerContext(context.Background()), 10*time.Second)
	defer cancel()
	_ = r.sessionRepo.Finalize(ctx, r.sessionRowID, r.inputB.Load(), r.outputB.Load(),
		int(r.chunkCnt.Load()), r.truncated.Load())
}

// flushLoop 周期把待写分块批量落库。
func (r *TerminalRecorder) flushLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(terminalRecordFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.done:
			r.flush()
			return
		case <-ticker.C:
			r.flush()
		}
	}
}

// flush 取走当前积压分块并批量落库。
func (r *TerminalRecorder) flush() {
	for {
		n := len(r.pending)
		if n == 0 {
			return
		}
		batch := make([]*ent.TerminalChunk, 0, n)
		for i := 0; i < n; i++ {
			batch = append(batch, <-r.pending)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		ctx = tenantScopedCtx(ctx, &r.tenantID)
		if err := r.chunkRepo.CreateBulk(ctx, batch); err != nil {
			r.log.Warnf("terminal record flush failed (session_row=%d count=%d): %s",
				r.sessionRowID, len(batch), err.Error())
		}
		cancel()
	}
}
