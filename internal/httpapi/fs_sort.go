package httpapi

import (
	"sort"
	"unicode"

	"rsyncgui/internal/app"
)

// 对外暴露一个函数：HTTP 层在返回 JSON 前调用它
func sortEntriesWindowsLike(entries []app.FSEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]

		// 1) 目录优先
		if a.IsDir != b.IsDir {
			return a.IsDir && !b.IsDir
		}

		// 2) 同类自然排序（不区分大小写）
		if naturalLessWindows(a.Name, b.Name) {
			return true
		}
		if naturalLessWindows(b.Name, a.Name) {
			return false
		}

		// 3) 完全相等：兜底稳定
		return a.Name < b.Name
	})
}

type natChunk struct {
	isNum bool
	s     string
	trim  string // 数字段去掉前导0后的串（可能为空，表示全0）
}

func splitNatural(s string) []natChunk {
	if s == "" {
		return nil
	}
	r := []rune(s)
	out := make([]natChunk, 0, 8)

	i := 0
	for i < len(r) {
		j := i
		isNum := unicode.IsDigit(r[i])
		for j < len(r) && unicode.IsDigit(r[j]) == isNum {
			j++
		}
		part := string(r[i:j])
		ch := natChunk{isNum: isNum, s: part}
		if isNum {
			k := 0
			// 注意：这里按 ASCII '0' 处理即可（数字段只会是 0-9）
			for k < len(part) && part[k] == '0' {
				k++
			}
			ch.trim = part[k:] // 可能为空
		}
		out = append(out, ch)
		i = j
	}
	return out
}

func foldLower(s string) string {
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}

func naturalLessWindows(a, b string) bool {
	if a == b {
		return false
	}
	aa := splitNatural(a)
	bb := splitNatural(b)

	n := len(aa)
	if len(bb) < n {
		n = len(bb)
	}

	for i := 0; i < n; i++ {
		x, y := aa[i], bb[i]

		// 数字段：按数值比较（长度优先，再字典序）
		if x.isNum && y.isNum {
			xt := x.trim
			yt := y.trim

			// 1) 有效数字长度（更长 => 数值更大）
			if len(xt) != len(yt) {
				return len(xt) < len(yt)
			}
			// 2) 有效数字字典序
			if xt != yt {
				return xt < yt
			}
			// 3) 数值相等：前导0少的排前（更像资源管理器）
			if len(x.s) != len(y.s) {
				return len(x.s) < len(y.s)
			}
			continue
		}

		// 非数字：不区分大小写比较
		xs := foldLower(x.s)
		ys := foldLower(y.s)
		if xs != ys {
			return xs < ys
		}

		// case-fold 相同但原串不同：兜底
		if x.s != y.s {
			return x.s < y.s
		}
	}

	// 前缀相同：短的排前
	if len(aa) != len(bb) {
		return len(aa) < len(bb)
	}
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}
