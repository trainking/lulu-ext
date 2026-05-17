package container

import (
	"fmt"
	"sync"
	"testing"
)

// player 测试用排行榜条目
type player struct {
	id    uint64
	score int64
	name  string
}

func (p *player) GetID() uint64    { return p.id }
func (p *player) GetScore() int64  { return p.score }

func TestNewRankBoard(t *testing.T) {
	rb := NewRankBoard()
	if rb == nil {
		t.Fatal("NewRankBoard returned nil")
	}
	if rb.Len() != 0 {
		t.Errorf("empty board: want 0, got %d", rb.Len())
	}
}

// ======================== AddOrUpdate 测试 ========================

func TestAddOrUpdate_InsertOrder(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.AddOrUpdate(&player{id: 2, score: 300})
	rb.AddOrUpdate(&player{id: 3, score: 200})

	if rb.Len() != 3 {
		t.Fatalf("want 3 entries, got %d", rb.Len())
	}

	top := rb.GetTop(3)
	// 应降序: 300, 200, 100
	if top[0].GetID() != 2 || top[0].GetScore() != 300 {
		t.Errorf("rank 1: want id=2 score=300, got id=%d score=%d", top[0].GetID(), top[0].GetScore())
	}
	if top[1].GetID() != 3 || top[1].GetScore() != 200 {
		t.Errorf("rank 2: want id=3 score=200, got id=%d score=%d", top[1].GetID(), top[1].GetScore())
	}
	if top[2].GetID() != 1 || top[2].GetScore() != 100 {
		t.Errorf("rank 3: want id=1 score=100, got id=%d score=%d", top[2].GetID(), top[2].GetScore())
	}
}

func TestAddOrUpdate_UpdateExisting(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.AddOrUpdate(&player{id: 2, score: 200})
	rb.AddOrUpdate(&player{id: 3, score: 300})

	// 更新玩家1的分数到 250，应升至第2名
	rb.AddOrUpdate(&player{id: 1, score: 250, name: "updated"})

	if rb.Len() != 3 {
		t.Fatalf("want 3 entries, got %d", rb.Len())
	}

	top := rb.GetTop(3)
	if top[0].GetID() != 3 || top[0].GetScore() != 300 {
		t.Errorf("rank 1: want id=3 score=300, got id=%d score=%d", top[0].GetID(), top[0].GetScore())
	}
	if top[1].GetID() != 1 || top[1].GetScore() != 250 {
		t.Errorf("rank 2: want id=1 score=250, got id=%d score=%d", top[1].GetID(), top[1].GetScore())
	}
	if top[2].GetID() != 2 || top[2].GetScore() != 200 {
		t.Errorf("rank 3: want id=2 score=200, got id=%d score=%d", top[2].GetID(), top[2].GetScore())
	}

	// 验证更新后的数据
	p, _ := top[1].(*player)
	if p.name != "updated" {
		t.Errorf("player data not updated: want 'updated', got '%s'", p.name)
	}
}

func TestAddOrUpdate_UpdateSameScore(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.AddOrUpdate(&player{id: 2, score: 200})
	// 更新同分数，位置不变
	rb.AddOrUpdate(&player{id: 1, score: 100, name: "same"})

	if rb.GetRank(1) != 2 {
		t.Errorf("same score update should keep rank 2, got %d", rb.GetRank(1))
	}
}

func TestAddOrUpdate_EqualScoresStable(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.AddOrUpdate(&player{id: 2, score: 100})
	rb.AddOrUpdate(&player{id: 3, score: 100})

	top := rb.GetTop(3)
	// 同分按插入顺序: 1, 2, 3
	for i := 0; i < 3; i++ {
		if top[i].GetID() != uint64(i+1) {
			t.Errorf("rank %d: want id=%d, got id=%d", i+1, i+1, top[i].GetID())
		}
	}
}

func TestAddOrUpdate_DuplicateID(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.AddOrUpdate(&player{id: 1, score: 200})

	if rb.Len() != 1 {
		t.Fatalf("duplicate ID should not increase count: want 1, got %d", rb.Len())
	}
	if rb.GetRank(1) != 1 {
		t.Errorf("updated entry should be rank 1, got %d", rb.GetRank(1))
	}
}

// ======================== Remove 测试 ========================

func TestRemove_Existing(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 300})
	rb.AddOrUpdate(&player{id: 2, score: 200})
	rb.AddOrUpdate(&player{id: 3, score: 100})

	rb.Remove(2)

	if rb.Len() != 2 {
		t.Fatalf("want 2 entries, got %d", rb.Len())
	}
	if rb.GetRank(2) != 0 {
		t.Errorf("removed entry should have rank 0, got %d", rb.GetRank(2))
	}

	top := rb.GetTop(2)
	if top[0].GetID() != 1 || top[1].GetID() != 3 {
		t.Errorf("remove broke order: want [1, 3], got [%d, %d]", top[0].GetID(), top[1].GetID())
	}
}

func TestRemove_NonExistent(t *testing.T) {
	rb := NewRankBoard()
	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.Remove(999) // 不应 panic
	if rb.Len() != 1 {
		t.Errorf("remove non-existent should not change count: want 1, got %d", rb.Len())
	}
}

func TestRemove_LastEntry(t *testing.T) {
	rb := NewRankBoard()
	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.Remove(1)

	if rb.Len() != 0 {
		t.Errorf("want 0 entries, got %d", rb.Len())
	}
	if len(rb.GetTop(10)) != 0 {
		t.Error("GetTop should return empty slice")
	}
}

// ======================== GetRank 测试 ========================

func TestGetRank(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 300})
	rb.AddOrUpdate(&player{id: 2, score: 200})
	rb.AddOrUpdate(&player{id: 3, score: 100})

	if r := rb.GetRank(1); r != 1 {
		t.Errorf("highest score: want rank 1, got %d", r)
	}
	if r := rb.GetRank(2); r != 2 {
		t.Errorf("middle score: want rank 2, got %d", r)
	}
	if r := rb.GetRank(3); r != 3 {
		t.Errorf("lowest score: want rank 3, got %d", r)
	}
	if r := rb.GetRank(999); r != 0 {
		t.Errorf("non-existent: want rank 0, got %d", r)
	}
}

func TestGetRank_Empty(t *testing.T) {
	rb := NewRankBoard()
	if r := rb.GetRank(1); r != 0 {
		t.Errorf("empty board: want rank 0, got %d", r)
	}
}

// ======================== GetTop 测试 ========================

func TestGetTop(t *testing.T) {
	rb := NewRankBoard()

	for i := uint64(1); i <= 10; i++ {
		rb.AddOrUpdate(&player{id: i, score: int64(i * 10)})
	}

	top3 := rb.GetTop(3)
	if len(top3) != 3 {
		t.Fatalf("want 3, got %d", len(top3))
	}
	if top3[0].GetID() != 10 || top3[1].GetID() != 9 || top3[2].GetID() != 8 {
		t.Errorf("top3 order wrong")
	}
}

func TestGetTop_ExceedsCount(t *testing.T) {
	rb := NewRankBoard()
	rb.AddOrUpdate(&player{id: 1, score: 100})
	rb.AddOrUpdate(&player{id: 2, score: 200})

	top := rb.GetTop(10)
	if len(top) != 2 {
		t.Errorf("want 2, got %d", len(top))
	}
}

func TestGetTop_Empty(t *testing.T) {
	rb := NewRankBoard()
	top := rb.GetTop(10)
	if len(top) != 0 {
		t.Errorf("empty board: want 0, got %d", len(top))
	}
	if top == nil {
		// 应该返回空切片而非 nil？两种都可以接受，这里不强制
		t.Log("GetTop on empty board returns nil")
	}
}

// ======================== GetRange 测试 ========================

func TestGetRange(t *testing.T) {
	rb := NewRankBoard()

	for i := uint64(1); i <= 10; i++ {
		rb.AddOrUpdate(&player{id: i, score: int64(i * 10)})
	}

	// 获取第 4-6 名
	range4to6 := rb.GetRange(4, 6)
	if len(range4to6) != 3 {
		t.Fatalf("want 3, got %d", len(range4to6))
	}
	if range4to6[0].GetID() != 7 {
		t.Errorf("rank 4: want id=7, got id=%d", range4to6[0].GetID())
	}
	if range4to6[1].GetID() != 6 {
		t.Errorf("rank 5: want id=6, got id=%d", range4to6[1].GetID())
	}
	if range4to6[2].GetID() != 5 {
		t.Errorf("rank 6: want id=5, got id=%d", range4to6[2].GetID())
	}
}

func TestGetRange_Clamping(t *testing.T) {
	rb := NewRankBoard()
	rb.AddOrUpdate(&player{id: 1, score: 300})
	rb.AddOrUpdate(&player{id: 2, score: 200})
	rb.AddOrUpdate(&player{id: 3, score: 100})

	// start < 1
	r := rb.GetRange(0, 2)
	if len(r) != 2 || r[0].GetID() != 1 {
		t.Errorf("clamped start: want 2 entries from rank 1")
	}

	// end > total
	r = rb.GetRange(2, 10)
	if len(r) != 2 || r[0].GetID() != 2 {
		t.Errorf("clamped end: want 2 entries from rank 2")
	}
}

func TestGetRange_Invalid(t *testing.T) {
	rb := NewRankBoard()
	rb.AddOrUpdate(&player{id: 1, score: 100})

	r := rb.GetRange(3, 5) // 超出范围
	if len(r) != 0 {
		t.Errorf("invalid range: want 0, got %d", len(r))
	}
}

// ======================== GetLast 测试 ========================

func TestGetLast(t *testing.T) {
	rb := NewRankBoard()

	rb.AddOrUpdate(&player{id: 1, score: 300})
	rb.AddOrUpdate(&player{id: 2, score: 200})
	rb.AddOrUpdate(&player{id: 3, score: 100})

	last, ok := rb.GetLast()
	if !ok {
		t.Fatal("non-empty board: want true, got false")
	}
	if last.GetID() != 3 || last.GetScore() != 100 {
		t.Errorf("want id=3 score=100, got id=%d score=%d", last.GetID(), last.GetScore())
	}
}

func TestGetLast_Empty(t *testing.T) {
	rb := NewRankBoard()
	last, ok := rb.GetLast()
	if ok {
		t.Error("empty board: want false, got true")
	}
	if last != nil {
		t.Error("empty board: want nil entry")
	}
}

// ======================== Len 测试 ========================

func TestLen(t *testing.T) {
	rb := NewRankBoard()
	if rb.Len() != 0 {
		t.Errorf("empty: want 0, got %d", rb.Len())
	}

	rb.AddOrUpdate(&player{id: 1, score: 100})
	if rb.Len() != 1 {
		t.Errorf("after add: want 1, got %d", rb.Len())
	}

	rb.Remove(1)
	if rb.Len() != 0 {
		t.Errorf("after remove: want 0, got %d", rb.Len())
	}
}

// ======================== 并发安全测试 ========================

func TestConcurrent(t *testing.T) {
	rb := NewRankBoard()
	var wg sync.WaitGroup

	// 并发添加
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				id := uint64(base*100 + j)
				rb.AddOrUpdate(&player{id: id, score: int64(j)})
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				rb.GetTop(10)
				rb.GetRank(uint64(j))
				rb.Len()
			}
		}()
	}

	wg.Wait()

	// 不应 panic，且数据一致
	t.Logf("concurrent test: final count=%d", rb.Len())
}

// ======================== 基准测试 ========================

func BenchmarkAddOrUpdate(b *testing.B) {
	rb := NewRankBoard()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.AddOrUpdate(&player{id: uint64(i), score: int64(i)})
	}
}

func BenchmarkAddOrUpdate_Update(b *testing.B) {
	rb := NewRankBoard()
	// 预填充
	for i := 0; i < 1000; i++ {
		rb.AddOrUpdate(&player{id: uint64(i), score: int64(i)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.AddOrUpdate(&player{id: uint64(i % 1000), score: int64(i)})
	}
}

func BenchmarkGetRank(b *testing.B) {
	rb := NewRankBoard()
	for i := 0; i < 1000; i++ {
		rb.AddOrUpdate(&player{id: uint64(i), score: int64(i)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.GetRank(uint64(i % 1000))
	}
}

func BenchmarkGetTop(b *testing.B) {
	rb := NewRankBoard()
	for i := 0; i < 1000; i++ {
		rb.AddOrUpdate(&player{id: uint64(i), score: int64(i)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.GetTop(50)
	}
}

func BenchmarkConcurrentAddOrUpdate(b *testing.B) {
	rb := NewRankBoard()
	b.RunParallel(func(pb *testing.PB) {
		var id uint64
		for pb.Next() {
			id++
			rb.AddOrUpdate(&player{id: id, score: int64(id % 1000)})
		}
	})
}

func BenchmarkLargeScale(b *testing.B) {
	for n := 0; n < b.N; n++ {
		rb := NewRankBoard()
		// 插入 10000 条数据
		for i := 0; i < 10000; i++ {
			rb.AddOrUpdate(&player{id: uint64(i), score: int64(10000 - i)})
		}
		// 查询 top 100
		_ = rb.GetTop(100)
		// 随机查询排名
		_ = rb.GetRank(5000)
	}
}

// ======================== 示例 ========================

func ExampleRankBoard() {
	rb := NewRankBoard()

	// 添加条目
	rb.AddOrUpdate(&player{id: 1, score: 100, name: "alice"})
	rb.AddOrUpdate(&player{id: 2, score: 300, name: "bob"})
	rb.AddOrUpdate(&player{id: 3, score: 200, name: "charlie"})

	// 获取排名
	fmt.Println(rb.GetRank(2)) // 最高分，排名第1

	// 获取前2名
	top := rb.GetTop(2)
	for _, e := range top {
		p := e.(*player)
		fmt.Printf("%s: %d\n", p.name, p.GetScore())
	}

	// 获取区间排名
	range2to3 := rb.GetRange(2, 3)
	for _, e := range range2to3 {
		p := e.(*player)
		fmt.Printf("%s: %d\n", p.name, p.GetScore())
	}

	// Output:
	// 1
	// bob: 300
	// charlie: 200
	// charlie: 200
	// alice: 100
}
