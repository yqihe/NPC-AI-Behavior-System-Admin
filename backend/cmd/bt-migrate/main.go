package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// ============================================================
// BT 存量迁移脚本（T7）
//
// 读：直连 MySQL（一次性运维，避免 service 层初始化样板）
// 写：通过 REST API PUT /api/v1/bt-trees/:id（强制走新 validator + 自动清缓存）
//
// 流程：
//   1. 连 MySQL，查 deleted=0 的 bt_trees
//   2. 逐棵 transformNode 规则化
//   3. 打印 BEFORE / AFTER / WARNINGS
//   4. 若 --apply：PUT 写入；任一失败立即 os.Exit(1)（不 continue，避免半迁移状态）
//   5. 结尾打印 Summary
// ============================================================

var (
	flagDSN      = flag.String("dsn", "root:root@tcp(127.0.0.1:3306)/npc_ai_admin?charset=utf8mb4&parseTime=true&loc=Local", "MySQL DSN")
	flagAdminURL = flag.String("admin-url", "http://127.0.0.1:9821", "ADMIN backend base URL")
	flagApply    = flag.Bool("apply", false, "false=dry-run（默认），true=通过 REST API 写入")
	flagTreeID   = flag.Int64("tree-id", 0, "0=所有未删除树，>0=仅迁移此 ID")
)

// btRow 从 bt_trees 读取的行
type btRow struct {
	ID          int64           `db:"id"`
	Name        string          `db:"name"`
	DisplayName string          `db:"display_name"`
	Description string          `db:"description"`
	Config      json.RawMessage `db:"config"`
	Version     int             `db:"version"`
	Enabled     bool            `db:"enabled"`
}

// updateBtTreeRequest 对应 model.UpdateBtTreeRequest（本脚本独立定义避免跨包 import 复杂度）
type updateBtTreeRequest struct {
	ID          int64           `json:"id"`
	Version     int             `json:"version"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Config      json.RawMessage `json:"config"`
}

// adminResp ADMIN 统一响应格式
type adminResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sqlx.ConnectContext(ctx, "mysql", *flagDSN)
	if err != nil {
		fatal("连接 MySQL 失败：%v", err)
	}
	defer db.Close()

	rows, err := loadRows(ctx, db, *flagTreeID)
	if err != nil {
		fatal("读取 bt_trees 失败：%v", err)
	}
	if len(rows) == 0 {
		fmt.Println("无待处理行为树（deleted=0 查询结果为空）")
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	transformed := 0
	applied := 0

	for _, row := range rows {
		fmt.Printf("\n=== Tree #%d  %s  (version=%d, enabled=%v) ===\n", row.ID, row.Name, row.Version, row.Enabled)

		var root map[string]any
		if err := json.Unmarshal(row.Config, &root); err != nil {
			fatal("tree #%d config 非 JSON 对象：%v", row.ID, err)
		}

		newNode, warnings, err := transformNode(root, row.Name, "$")
		if err != nil {
			fatal("tree #%d transformNode 失败：%v", row.ID, err)
		}
		transformed++

		newConfig, err := json.Marshal(newNode)
		if err != nil {
			fatal("tree #%d 重新编码失败：%v", row.ID, err)
		}

		printDiff(row.Config, newConfig, warnings)

		if !*flagApply {
			continue
		}

		if err := putTree(ctx, httpClient, *flagAdminURL, row, newConfig); err != nil {
			fatal("tree #%d PUT 写入失败：%v", row.ID, err)
		}
		applied++
		fmt.Printf("[APPLIED] tree #%d 写入成功\n", row.ID)
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("%d/%d trees transformed", transformed, len(rows))
	if *flagApply {
		fmt.Printf(", %d applied\n", applied)
	} else {
		fmt.Printf(" (dry-run — 未写入 DB；加 --apply 生效)\n")
	}
}

func loadRows(ctx context.Context, db *sqlx.DB, treeID int64) ([]btRow, error) {
	query := `SELECT id, name, display_name, description, config, version, enabled
	          FROM bt_trees WHERE deleted = 0`
	args := []any{}
	if treeID > 0 {
		query += ` AND id = ?`
		args = append(args, treeID)
	}
	query += ` ORDER BY id ASC`

	var rows []btRow
	if err := db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func putTree(ctx context.Context, client *http.Client, adminURL string, row btRow, newConfig json.RawMessage) error {
	body := updateBtTreeRequest{
		ID:          row.ID,
		Version:     row.Version,
		DisplayName: row.DisplayName,
		Description: row.Description,
		Config:      newConfig,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/bt-trees/%d", adminURL, row.ID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var r adminResp
	if err := json.Unmarshal(respBody, &r); err != nil {
		return fmt.Errorf("parse response %q: %w", string(respBody), err)
	}
	if r.Code != 0 {
		return fmt.Errorf("admin returned code=%d message=%q", r.Code, r.Message)
	}
	return nil
}

func printDiff(before, after json.RawMessage, warnings []string) {
	var beforePretty, afterPretty bytes.Buffer
	if err := json.Indent(&beforePretty, before, "  ", "  "); err != nil {
		beforePretty.WriteString(string(before))
	}
	if err := json.Indent(&afterPretty, after, "  ", "  "); err != nil {
		afterPretty.WriteString(string(after))
	}

	fmt.Println("[BEFORE]")
	fmt.Println("  " + beforePretty.String())
	fmt.Println("[AFTER]")
	fmt.Println("  " + afterPretty.String())

	if len(warnings) > 0 {
		fmt.Printf("[CHANGES] %d\n", len(warnings))
		for _, w := range warnings {
			fmt.Println("  - " + w)
		}
	} else {
		fmt.Println("[CHANGES] 0 (already canonical)")
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[FATAL] "+format+"\n", args...)
	os.Exit(1)
}
