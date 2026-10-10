package main

// workagent record —— 把一次结论固化成**可公开、可累积**的公开记录。
//
// 这是本项目唯一会随时间增值的东西：编号 + 结论 + 证据摘要在仓库里留档，
// 而原始输入不进仓库（只留 sha256 摘要），所以记录可以公开。
//
// 编号规则：records/ 下已有的最大编号 + 1，四位补零，只增不改。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WmjXiaoJun/workagent/internal/i18n"
	"github.com/WmjXiaoJun/workagent/internal/render"
)

var recordNameRe = regexp.MustCompile(`^(\d{4})-(.+)-(\d{8})\.md$`)

// recordFileDigest 计算单个文件的 sha256（十六进制，全长）。
func recordFileDigest(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func shortDigest(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// recordNextNumber 扫描 records/ 取最大编号 + 1。
func recordNextNumber(root string) int {
	dir := filepath.Join(root, "records")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range ents {
		m := recordNameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// parseFrontMatter 读记录文件头部 --- 之间的 key: value。
func parseFrontMatter(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	out := map[string]string{}
	for _, ln := range lines[1:] {
		if strings.TrimSpace(ln) == "---" {
			break
		}
		if i := strings.Index(ln, ":"); i > 0 {
			k := strings.TrimSpace(ln[:i])
			v := strings.TrimSpace(ln[i+1:])
			v = strings.Trim(v, "\"")
			out[k] = v
		}
	}
	return out
}

// rebuildRecordIndex 从 records/*.md 的头部信息重建 INDEX.md。
// 索引是**生成物**：手改会丢，改记录才是改源头。
func rebuildRecordIndex(root string) (int, error) {
	dir := filepath.Join(root, "records")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	type row struct {
		num, date, caseID, title, shape, verdict, level, outDigest, name string
	}
	var rows []row
	for _, e := range ents {
		if !recordNameRe.MatchString(e.Name()) {
			continue
		}
		fm := parseFrontMatter(filepath.Join(dir, e.Name()))
		if fm == nil {
			continue
		}
		rows = append(rows, row{
			num: fm["record"], date: fm["date_short"], caseID: fm["case"],
			title: fm["title"], shape: fm["shape"], verdict: fm["verdict"],
			level: fm["level"], outDigest: fm["output_digest"], name: e.Name(),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name > rows[j].name })

	ver := versionOf(root)
	var b strings.Builder
	b.WriteString(i18n.T("# 公开记录索引\n\n", "# Public record index\n\n"))
	b.WriteString(i18n.T("> 每一次结论 + 证据在这里留档。**这是本项目唯一随时间增值的资产。**\n\n",
		"> Every conclusion and its evidence is archived here. **This is the only asset that compounds over time.**\n\n"))
	b.WriteString(i18n.T("记录是**只增不改**的：每次 `workagent record` 生成一条新编号，",
		"Records are **append-only**: each `workagent record` allocates a new number, and "))
	b.WriteString(i18n.T("`INDEX.md` 由 `records/*.md` 自动重建，手改会丢。\n\n",
		"`INDEX.md` is rebuilt from `records/*.md`; hand edits are lost.\n\n"))
	b.WriteString(i18n.T("原始输入**不进仓库**（只留 sha256 摘要），所以记录可以公开。\n\n",
		"Raw inputs **never enter the repository** (only their sha256), which is why these records can be public.\n\n"))
	if len(rows) == 0 {
		b.WriteString(i18n.T("_暂无记录。用 `workagent record --case <ID>` 生成第一条。_\n",
			"_No records yet. Create the first with `workagent record --case <ID>`._\n"))
	} else {
		b.WriteString(i18n.Tf("共 **%d** 条记录 ｜ 包版本 `%s`\n\n",
			"**%d** records ｜ package version `%s`\n\n", len(rows), ver))
		b.WriteString(i18n.T("| 编号 | 日期 | 场景 | 形态 | 判定 | 复现 | 输出摘要 |\n",
			"| # | Date | Scenario | Form | Verdict | Repro | Output digest |\n"))
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("| [%s](%s) | %s | `%s` | %s | **%s** | %s | `%s` |\n",
				r.num, r.name, r.date, r.caseID, i18n.Shape(r.shape), r.verdict, r.level, shortDigest(r.outDigest)))
		}
	}
	return len(rows), os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte(b.String()), 0o644)
}

// cmdRecord 生成一条公开记录。
//
// live=false 时对带模型的场景**默认走重放** —— 公开记录必须是谁都能离线复现的，
// 依赖云端 key 的记录对读者毫无价值（第一次实现时正是这么错的：
// 三个 L2/L1 场景因为没有 API key 直接记录失败）。
func cmdRecord(root, caseID, inputArg, note string, live bool) int {
	if strings.TrimSpace(caseID) == "" {
		fmt.Fprintln(os.Stderr, i18n.T("record 需要 --case 参数", "record needs a --case argument"))
		return 2
	}
	m := mustManifest(root)
	c, found := findCase(m, caseID)
	if !found {
		fmt.Fprintln(os.Stderr, i18n.T("找不到场景: ", "Scenario not found: ")+caseID+i18n.T("（用 workagent cases 查看全部）", " (see all with: workagent cases)"))
		return 2
	}

	// 先跑一次，保证记录对应的是可复现的当前结论
	replay := !live
	if code := cmdRun(root, caseID, inputArg, replay, "", ""); code != 0 {
		fmt.Fprintln(os.Stderr, i18n.T("记录失败：场景执行未成功", "record failed: the scenario did not run successfully"))
		return code
	}

	// 从 trace 取本次运行的摘要信息
	m2 := mustManifest(root)
	traceRel := asString(asMap(m2["provenance"])["trace"])
	tracePath := filepath.Join(root, filepath.FromSlash(traceRel))
	var step map[string]any
	if tr := readJSONMap(tracePath); tr != nil {
		if list, ok := tr["steps"].([]any); ok {
			for i := len(list) - 1; i >= 0; i-- {
				s := asMap(list[i])
				if asString(s["case"]) == caseID {
					step = s
					break
				}
			}
		}
	}
	if step == nil {
		fmt.Fprintln(os.Stderr, i18n.T("记录失败：trace 中找不到该场景的运行记录", "record failed: no run record for this scenario in the trace"))
		return 1
	}
	evidenceID := asString(step["evidence_id"])
	inputFile := resolveInputPath(root, asString(step["input_file"]))
	inputDigest := asString(step["input_digest"])
	outputDigest := asString(step["output_digest"])
	at := asString(step["at"])

	// 读证据包里的结论
	outPath := filepath.Join(root, "out", "evidence", evidenceID, "output.json")
	out := readJSONMap(outPath)
	// 记录是给人看的，走渲染副本；证据包里的原始输出不动。
	if cat := render.Load(root); cat != nil {
		if r := cat.Apply(i18n.Lang(), caseID, out); r != nil {
			if rm, isMap := r.(map[string]any); isMap {
				out = rm
			}
		}
	}
	verdict := asString(out["verdict"])
	conclusion := asString(out["conclusion"])
	frac, _ := out["findings"].([]any)

	// 执行体摘要
	execDigest := ""
	if len(c.Executor) > 1 {
		execDigest = recordFileDigest(filepath.Join(root, filepath.FromSlash(c.Executor[1])))
	}

	num := recordNextNumber(root)
	stamp := time.Now()
	numStr := fmt.Sprintf("%04d", num)
	dateShort := stamp.Format("2006-01-02")
	relName := numStr + "-" + caseID + "-" + stamp.Format("20060102") + ".md"
	relInput := relToRoot(root, inputFile)

	level := "L3"
	if len(c.LLMSteps) > 0 {
		level = "L2/L1"
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "record: \"%s\"\n", numStr)
	fmt.Fprintf(&b, "case: %s\n", caseID)
	fmt.Fprintf(&b, "title: %s\n", c.Title)
	fmt.Fprintf(&b, "shape: %s\n", c.Shape)
	fmt.Fprintf(&b, "level: %s\n", level)
	fmt.Fprintf(&b, "date: %s\n", stamp.Format(time.RFC3339))
	fmt.Fprintf(&b, "date_short: %s\n", dateShort)
	fmt.Fprintf(&b, "verdict: %s\n", verdict)
	fmt.Fprintf(&b, "input: %s\n", relInput)
	fmt.Fprintf(&b, "input_digest: %s\n", inputDigest)
	fmt.Fprintf(&b, "output_digest: %s\n", outputDigest)
	fmt.Fprintf(&b, "evidence_id: %s\n", evidenceID)
	fmt.Fprintf(&b, "executor_digest: %s\n", execDigest)
	fmt.Fprintf(&b, "package_version: %s\n", versionOf(root))
	b.WriteString("---\n\n")

	fmt.Fprintf(&b, "# %s · %s\n\n", numStr, c.Title)
	fmt.Fprintf(&b, i18n.T("**结论**：%s\n\n", "**Conclusion**: %s\n\n"), conclusion)
	b.WriteString(i18n.T("| 项 | 值 |\n|---|---|\n", "| Item | Value |\n|---|---|\n"))
	fmt.Fprintf(&b, i18n.T("| 场景 | `%s`（%s） |\n", "| Scenario | `%s` (%s) |\n"), caseID, i18n.Shape(c.Shape))
	fmt.Fprintf(&b, i18n.T("| 复现等级 | %s |\n", "| Reproducibility | %s |\n"), level)
	fmt.Fprintf(&b, i18n.T("| 判定 | **%s** |\n", "| Verdict | **%s** |\n"), verdict)
	fmt.Fprintf(&b, i18n.T("| 输入 | `%s` |\n", "| Input | `%s` |\n"), relInput)
	fmt.Fprintf(&b, i18n.T("| 输入摘要 | `%s` |\n", "| Input digest | `%s` |\n"), shortDigest(inputDigest))
	fmt.Fprintf(&b, i18n.T("| 输出摘要 | `%s` |\n", "| Output digest | `%s` |\n"), shortDigest(outputDigest))
	fmt.Fprintf(&b, i18n.T("| 执行体摘要 | `%s` |\n", "| Executor digest | `%s` |\n"), shortDigest(execDigest))
	fmt.Fprintf(&b, i18n.T("| 证据包编号 | `%s` |\n", "| Evidence bundle | `%s` |\n"), evidenceID)
	fmt.Fprintf(&b, i18n.T("| 包版本 | %s |\n", "| Package version | %s |\n"), versionOf(root))
	fmt.Fprintf(&b, i18n.T("| 记录时间 | %s |\n\n", "| Recorded at | %s |\n\n"), at)

	if note != "" {
		b.WriteString(i18n.T("## 备注\n\n", "## Note\n\n") + note + "\n\n")
	}

	b.WriteString(i18n.T("## 发现\n\n", "## Findings\n\n"))
	if len(frac) == 0 {
		b.WriteString(i18n.T("_无。_\n\n", "_None._\n\n"))
	} else {
		for _, f := range frac {
			fm := asMap(f)
			fmt.Fprintf(&b, "- `[%s] %s` %s\n", asString(fm["level"]), asString(fm["code"]), asString(fm["message"]))
		}
		b.WriteString("\n")
	}

	b.WriteString(i18n.T("## 复现\n\n```bash\n", "## Reproduce\n\n```bash\n"))
	fmt.Fprintf(&b, "workagent run --case %s --input %s .\n", caseID, relInput)
	b.WriteString("workagent verify .\n```\n")

	dir := filepath.Join(root, "records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("无法创建 records/：", "cannot create records/: ")+err.Error())
		return 1
	}
	path := filepath.Join(dir, relName)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("写入记录失败：", "failed to write the record: ")+err.Error())
		return 1
	}
	n, err := rebuildRecordIndex(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.T("重建索引失败：", "failed to rebuild the index: ")+err.Error())
		return 1
	}
	fmt.Println("")
	fmt.Println(i18n.T("record —— 已留档", "record — archived"))
	fmt.Println(i18n.T("  [OK]   记录编号 ", "  [OK]   Record number ") + numStr)
	fmt.Println(i18n.T("  [OK]   记录文件 records/", "  [OK]   Record file records/") + relName)
	fmt.Println(i18n.T("  [OK]   索引 records/INDEX.md（共 ", "  [OK]   Index records/INDEX.md (") + strconv.Itoa(n) + i18n.T(" 条）", " entries)"))
	fmt.Println(i18n.T("  [OK]   结论 ", "  [OK]   Conclusion ") + conclusion)
	mode := i18n.T("重放已录制响应（离线，任何人都能复现）", "replaying a recorded response (offline — anyone can reproduce this)")
	if live {
		mode = i18n.T("真实调用模型（结果依赖云端，仅记录可重放）", "calling the model for real (the result depends on the cloud; only the record is replayable)")
	}
	fmt.Println(i18n.T("  [--]   模型步骤 ", "  [--]   Model steps: ") + mode)
	fmt.Println(i18n.T("  [--]   原始输入不进仓库，只留摘要 ", "  [--]   Raw input stays out of the repository; only the digest ") + shortDigest(inputDigest))
	return 0
}

// readJSONMap 读一个 JSON 对象，失败返回 nil。
func readJSONMap(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}
