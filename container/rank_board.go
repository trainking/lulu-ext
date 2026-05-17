package container

import (
	"sort"
	"sync"
)

// RankEntry 排行榜条目接口，调用方需要实现此接口来定义排行榜数据
type RankEntry interface {
	GetID() uint64
	GetScore() int64
}

// RankBoard 排行榜容器，按分数降序排列（分数相同按插入顺序排列）
type RankBoard struct {
	mu      sync.RWMutex
	entries []RankEntry
	index   map[uint64]int // key: entry ID, value: 在 entries 中的位置
}

// NewRankBoard 创建排行榜
func NewRankBoard() *RankBoard {
	return &RankBoard{
		index: make(map[uint64]int),
	}
}

// AddOrUpdate 添加或更新条目。若 ID 已存在则更新分数并重新排序；否则插入新条目
func (rb *RankBoard) AddOrUpdate(entry RankEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	id := entry.GetID()
	score := entry.GetScore()

	if pos, ok := rb.index[id]; ok {
		// 更新已有条目
		oldScore := rb.entries[pos].GetScore()
		rb.entries[pos] = entry
		if score != oldScore {
			rb.resortEntry(pos)
		}
		return
	}

	// 插入新条目：二分查找插入位置，保持降序，同分时按插入先后排列
	pos := sort.Search(len(rb.entries), func(i int) bool {
		return rb.entries[i].GetScore() < score
	})

	rb.entries = append(rb.entries, nil)
	copy(rb.entries[pos+1:], rb.entries[pos:])
	rb.entries[pos] = entry

	// 更新被移动条目的索引
	for i := pos + 1; i < len(rb.entries); i++ {
		rb.index[rb.entries[i].GetID()] = i
	}
	rb.index[id] = pos
}

// Remove 移除指定 ID 的条目
func (rb *RankBoard) Remove(id uint64) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	pos, ok := rb.index[id]
	if !ok {
		return
	}

	rb.entries = append(rb.entries[:pos], rb.entries[pos+1:]...)
	delete(rb.index, id)

	// 更新后续条目的索引
	for i := pos; i < len(rb.entries); i++ {
		rb.index[rb.entries[i].GetID()] = i
	}
}

// GetRank 获取指定 ID 的排名（1-based，最高分为第1名）。不存在返回 0
func (rb *RankBoard) GetRank(id uint64) int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	pos, ok := rb.index[id]
	if !ok {
		return 0
	}
	return pos + 1
}

// GetTop 获取前 N 名条目。N 超过总数时返回全部
func (rb *RankBoard) GetTop(n int) []RankEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n > len(rb.entries) {
		n = len(rb.entries)
	}

	result := make([]RankEntry, n)
	copy(result, rb.entries[:n])
	return result
}

// GetRange 获取排名区间 [start, end] 的条目（1-based，闭区间）
// start 小于 1 时取 1，end 超过总数时取最后一名。start > end 或区间无效时返回空
func (rb *RankBoard) GetRange(start, end int) []RankEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if start < 1 {
		start = 1
	}
	if end > len(rb.entries) {
		end = len(rb.entries)
	}
	if start > end {
		return nil
	}

	result := make([]RankEntry, end-start+1)
	copy(result, rb.entries[start-1:end])
	return result
}

// Len 返回排行榜条目总数
func (rb *RankBoard) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.entries)
}

// GetLast 返回最后一名条目。排行榜为空时返回 nil, false
func (rb *RankBoard) GetLast() (RankEntry, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil, false
	}
	return rb.entries[len(rb.entries)-1], true
}

// resortEntry 重新调整指定位置条目的排序位置（调用方需持有写锁）
func (rb *RankBoard) resortEntry(pos int) {
	entry := rb.entries[pos]
	score := entry.GetScore()
	id := entry.GetID()

	// 移除旧位置
	rb.entries = append(rb.entries[:pos], rb.entries[pos+1:]...)

	// 二分查找新位置
	newPos := sort.Search(len(rb.entries), func(i int) bool {
		return rb.entries[i].GetScore() < score
	})

	rb.entries = append(rb.entries, nil)
	copy(rb.entries[newPos+1:], rb.entries[newPos:])
	rb.entries[newPos] = entry

	// 更新受影响区间的索引
	from, to := pos, newPos
	if newPos < pos {
		from, to = newPos, pos
	}
	for i := from; i <= to; i++ {
		rb.index[rb.entries[i].GetID()] = i
	}
	rb.index[id] = newPos
}
