package matchx

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// testCallback 测试用回调实现
type testCallback struct {
	successNum int
	robotSeq   uint64
	// 用于注入错误的可选字段
	callRobotsErr    error
	callRobotsNum    int // 返回自定义数量，0 表示使用默认行为(返回 num 个)
}

func (c *testCallback) GetSuccessNum() int {
	return c.successNum
}

func (c *testCallback) CallRobots(num int) ([]*MatchPlayer, error) {
	if c.callRobotsErr != nil {
		return nil, c.callRobotsErr
	}
	n := num
	if c.callRobotsNum > 0 {
		n = c.callRobotsNum
	}
	robots := make([]*MatchPlayer, n)
	for i := 0; i < n; i++ {
		id := atomic.AddUint64(&c.robotSeq, 1)
		robots[i] = &MatchPlayer{
			UserID:  id,
			IsRobot: true,
		}
	}
	return robots, nil
}

// collectGroups 从 success channel 收集所有匹配组
func collectGroups(mq *MatchQueue, timeout time.Duration) []*MatchGroup {
	var groups []*MatchGroup
	deadline := time.After(timeout)
	for {
		select {
		case g, ok := <-mq.Success():
			if !ok {
				return groups
			}
			groups = append(groups, g)
		case <-deadline:
			return groups
		}
	}
}

// collectGroupsAsync 异步收集，返回结果 channel
func collectGroupsAsync(mq *MatchQueue) <-chan []*MatchGroup {
	ch := make(chan []*MatchGroup, 1)
	go func() {
		var groups []*MatchGroup
		for g := range mq.Success() {
			groups = append(groups, g)
		}
		ch <- groups
	}()
	return ch
}

// ======================== 构造测试 ========================

func TestNewMatchQueue(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 5, 10*time.Second)

	if mq.callback != cb {
		t.Error("callback not set correctly")
	}
	if cap(mq.success) != 5 {
		t.Errorf("success buffer: want 5, got %d", cap(mq.success))
	}
	if mq.realTimeout != 10*time.Second {
		t.Errorf("realTimeout: want 10s, got %v", mq.realTimeout)
	}
	if mq.matchValue == nil {
		t.Error("matchValue map not initialized")
	}
	if mq.matching == nil {
		t.Error("matching list not initialized")
	}

	mq.Close()
}

// ======================== 真人组队测试 ========================

func TestGroupReal_ExactMatch(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 5*time.Second)
	defer mq.Close()

	// 加入3人，恰好凑够一组
	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})
	mq.Add(&MatchPlayer{UserID: 3})

	groups := collectGroups(mq, 500*time.Millisecond)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(groups))
	}
	if len(groups[0].Players) != 3 {
		t.Errorf("want 3 players in group, got %d", len(groups[0].Players))
	}
}

func TestGroupReal_MultipleRounds(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 10, 5*time.Second)
	defer mq.Close()

	// 加入6人，应组成3组
	for i := uint64(1); i <= 6; i++ {
		mq.Add(&MatchPlayer{UserID: i})
	}

	groups := collectGroups(mq, 500*time.Millisecond)
	if len(groups) != 3 {
		t.Fatalf("want 3 groups, got %d", len(groups))
	}
	for _, g := range groups {
		if len(g.Players) != 2 {
			t.Errorf("want 2 players per group, got %d", len(g.Players))
		}
	}
}

func TestGroupReal_InsufficientPlayers(t *testing.T) {
	cb := &testCallback{successNum: 5}
	mq := NewMatchQueue(cb, 10, 500*time.Millisecond)
	defer mq.Close()

	// 只加入3人，不够一队
	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})
	mq.Add(&MatchPlayer{UserID: 3})

	// 等待超时，应该通过机器人补齐
	groups := collectGroups(mq, time.Second)
	if len(groups) != 3 {
		t.Fatalf("want 3 groups (one per timed-out player), got %d", len(groups))
	}
	for _, g := range groups {
		if len(g.Players) != 5 {
			t.Errorf("want 5 players per group (1 real + 4 bots), got %d", len(g.Players))
		}
		// 每组应包含4个机器人
		robotCount := 0
		for _, p := range g.Players {
			if p.IsRobot {
				robotCount++
			}
		}
		if robotCount != 4 {
			t.Errorf("want 4 robots per group, got %d", robotCount)
		}
	}
}

func TestGroupReal_ExcessPlayersRollback(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 10, 500*time.Millisecond)
	defer mq.Close()

	// 加入3人，只能凑1组+1人剩余
	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})
	mq.Add(&MatchPlayer{UserID: 3})

	// 第一组真人成组，剩余的会在超时后由机器人补齐
	groups := collectGroups(mq, time.Second)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups (1 real + 1 AI), got %d", len(groups))
	}

	// 第一组应该是2个真人
	realCount := 0
	for _, p := range groups[0].Players {
		if !p.IsRobot {
			realCount++
		}
	}
	if realCount != 2 {
		t.Errorf("first group: want 2 real players, got %d", realCount)
	}
}

// ======================== RobotOnly 测试 ========================

func TestRobotOnly_ImmediateMatch(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 5*time.Second)
	defer mq.Close()

	mq.Add(&MatchPlayer{UserID: 1, RobotOnly: true})

	groups := collectGroups(mq, 200*time.Millisecond)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(groups))
	}
	if len(groups[0].Players) != 3 {
		t.Errorf("want 3 players (1 real + 2 bots), got %d", len(groups[0].Players))
	}
	// 确认组内有机器人
	robotCount := 0
	for _, p := range groups[0].Players {
		if p.IsRobot {
			robotCount++
		}
	}
	if robotCount != 2 {
		t.Errorf("want 2 robots, got %d", robotCount)
	}
}

func TestRobotOnly_MixedWithNormal(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 5*time.Second)
	defer mq.Close()

	// RobotOnly 玩家直接与机器人成组
	mq.Add(&MatchPlayer{UserID: 10, RobotOnly: true})

	groups := collectGroups(mq, 200*time.Millisecond)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(groups))
	}
	// 组内应包含 UserID=10 的玩家
	found := false
	for _, p := range groups[0].Players {
		if p.UserID == 10 && !p.IsRobot {
			found = true
			break
		}
	}
	if !found {
		t.Error("RobotOnly player not found in match group")
	}
}

// ======================== Del 测试 ========================

func TestDel_RemoveBeforeMatch(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 500*time.Millisecond)
	defer mq.Close()

	// 先加两人，删除一个，再加第三人不触发 groupReal
	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})
	mq.Del(2)
	mq.Add(&MatchPlayer{UserID: 3})

	// 只剩2人不够成组，等待超时后各自与机器人匹配
	groups := collectGroups(mq, time.Second)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups (players 1 and 3 each with bots), got %d", len(groups))
	}

	// 确认玩家2不在任何组中
	for _, g := range groups {
		for _, p := range g.Players {
			if p.UserID == 2 && !p.IsRobot {
				t.Error("player 2 should not be in any group")
			}
		}
	}
}

func TestDel_NonExistentPlayer(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 500*time.Millisecond)
	defer mq.Close()

	mq.Add(&MatchPlayer{UserID: 1})
	// 删除不存在的玩家不应 panic
	mq.Del(999)

	groups := collectGroups(mq, time.Second)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(groups))
	}
}

func TestDel_SamePlayerTwice(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 500*time.Millisecond)
	defer mq.Close()

	// 先加两人，删除同一个人两次，再加第三人不触发 groupReal
	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})
	mq.Del(2)
	mq.Del(2) // 不应 panic
	mq.Add(&MatchPlayer{UserID: 3})

	groups := collectGroups(mq, time.Second)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
}

// ======================== 超时测试 ========================

func TestTimeout_PlayersMatchWithRobots(t *testing.T) {
	cb := &testCallback{successNum: 5}
	mq := NewMatchQueue(cb, 10, 100*time.Millisecond)
	defer mq.Close()

	mq.Add(&MatchPlayer{UserID: 1})

	groups := collectGroups(mq, 500*time.Millisecond)
	if len(groups) != 1 {
		t.Fatalf("want 1 group (timeout -> AI match), got %d", len(groups))
	}
	if len(groups[0].Players) != 5 {
		t.Errorf("want 5 players (1 real + 4 bots), got %d", len(groups[0].Players))
	}
}

func TestTimeout_EmptyQueueNoOp(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 10, 50*time.Millisecond)
	defer mq.Close()

	// 不添加任何玩家，超时不应产生任何组
	time.Sleep(200 * time.Millisecond)
	groups := collectGroups(mq, 200*time.Millisecond)
	if len(groups) != 0 {
		t.Errorf("empty queue should produce no groups, got %d", len(groups))
	}
}

// ======================== Close 测试 ========================

func TestClose_StopsProcessing(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 10, 5*time.Second)

	mq.Add(&MatchPlayer{UserID: 1})
	mq.Close()

	// Close 后 run() goroutine 退出，不再处理后续请求
	// 已入队的玩家因没达到 successNum 不会成组
	groups := collectGroups(mq, 200*time.Millisecond)
	t.Logf("groups received after close: %d", len(groups))
}

func TestClose_CloseChanBlocksSuccessSend(t *testing.T) {
	cb := &testCallback{successNum: 2}
	// buffer=0，success 无缓冲
	mq := NewMatchQueue(cb, 0, 5*time.Second)

	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})

	// 给一点时间让匹配发生
	groups := collectGroups(mq, 200*time.Millisecond)
	if len(groups) < 1 {
		t.Error("should have at least 1 group sent")
	}
	mq.Close()
}

// ======================== Callback 错误测试 ========================

func TestCallRobots_Error(t *testing.T) {
	cb := &testCallback{
		successNum:     3,
		callRobotsErr:  errors.New("no robots available"),
	}
	mq := NewMatchQueue(cb, 10, 50*time.Millisecond)
	defer mq.Close()

	mq.Add(&MatchPlayer{UserID: 1, RobotOnly: true})

	groups := collectGroups(mq, 300*time.Millisecond)
	if len(groups) != 0 {
		t.Errorf("want 0 groups (error in CallRobots), got %d", len(groups))
	}
}

func TestCallRobots_WrongCount(t *testing.T) {
	cb := &testCallback{
		successNum:     3,
		callRobotsNum:  1, // 返回数量不足 (需要2个机器人，只返回1个)
	}
	mq := NewMatchQueue(cb, 10, 50*time.Millisecond)
	defer mq.Close()

	mq.Add(&MatchPlayer{UserID: 1, RobotOnly: true})

	groups := collectGroups(mq, 300*time.Millisecond)
	if len(groups) != 0 {
		t.Errorf("want 0 groups (wrong robot count), got %d", len(groups))
	}
}

// ======================== Success Channel 测试 ========================

func TestSuccess_ReturnsChannel(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 10, 5*time.Second)
	defer mq.Close()

	ch := mq.Success()
	if ch == nil {
		t.Fatal("Success() returned nil")
	}
}

func TestSuccess_ChannelNotClosed(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 10, 5*time.Second)

	mq.Add(&MatchPlayer{UserID: 1})
	mq.Add(&MatchPlayer{UserID: 2})

	// 成功匹配后 channel 仍然打开
	g, ok := <-mq.Success()
	if !ok {
		t.Fatal("success channel should be open after receiving match result")
	}
	if g == nil {
		t.Fatal("received nil group")
	}

	mq.Close()
}

// ======================== 并发安全测试 ========================

func TestConcurrentAdd(t *testing.T) {
	cb := &testCallback{successNum: 3}
	mq := NewMatchQueue(cb, 100, time.Second)
	defer mq.Close()

	done := make(chan struct{})
	// 多 goroutine 并发添加
	for i := 0; i < 5; i++ {
		go func(base uint64) {
			for j := uint64(0); j < 10; j++ {
				mq.Add(&MatchPlayer{UserID: base*100 + j})
			}
			done <- struct{}{}
		}(uint64(i))
	}

	// 等待所有添加完成
	for i := 0; i < 5; i++ {
		<-done
	}

	// 应产生 50/3 = 16 组真人+2个超时补位
	groups := collectGroups(mq, time.Second)
	if len(groups) < 16 {
		t.Errorf("want at least 16 groups, got %d", len(groups))
	}
	t.Logf("concurrent add produced %d groups", len(groups))
}

func TestConcurrentAddDel(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 100, 300*time.Millisecond)
	defer mq.Close()

	// 并发 Add 和 Del
	go func() {
		for i := uint64(1); i <= 20; i++ {
			mq.Add(&MatchPlayer{UserID: i})
		}
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		for i := uint64(5); i <= 15; i++ {
			mq.Del(i)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	groups := collectGroups(mq, 200*time.Millisecond)
	t.Logf("concurrent add/del produced %d groups", len(groups))
}

// ======================== UserData 透传测试 ========================

func TestUserData_Preserved(t *testing.T) {
	cb := &testCallback{successNum: 2}
	mq := NewMatchQueue(cb, 10, 5*time.Second)
	defer mq.Close()

	type myData struct{ Name string }
	mq.Add(&MatchPlayer{UserID: 1, UserData: &myData{Name: "alice"}})
	mq.Add(&MatchPlayer{UserID: 2, UserData: &myData{Name: "bob"}})

	groups := collectGroups(mq, 200*time.Millisecond)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(groups))
	}
	for _, p := range groups[0].Players {
		d, ok := p.UserData.(*myData)
		if !ok {
			t.Errorf("UserData type mismatch for player %d", p.UserID)
			continue
		}
		if d.Name != "alice" && d.Name != "bob" {
			t.Errorf("unexpected UserData name: %s", d.Name)
		}
	}
}

// ======================== 基准测试 ========================

func BenchmarkAdd(b *testing.B) {
	cb := &testCallback{successNum: 1000}
	mq := NewMatchQueue(cb, 100, 10*time.Second)
	defer mq.Close()

	// 消费 success channel 防止阻塞
	go func() {
		for range mq.Success() {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mq.Add(&MatchPlayer{UserID: uint64(i)})
	}
}

func BenchmarkGroupReal(b *testing.B) {
	for n := 0; n < b.N; n++ {
		cb := &testCallback{successNum: 4}
		mq := NewMatchQueue(cb, 10, 10*time.Second)

		for i := uint64(0); i < 4; i++ {
			mq.Add(&MatchPlayer{UserID: i})
		}

		<-mq.Success()
		mq.Close()
	}
}

func BenchmarkGroupAI(b *testing.B) {
	for n := 0; n < b.N; n++ {
		cb := &testCallback{successNum: 4}
		mq := NewMatchQueue(cb, 10, 10*time.Second)

		mq.Add(&MatchPlayer{UserID: 1, RobotOnly: true})

		<-mq.Success()
		mq.Close()
	}
}

func BenchmarkMatchQueueLifecycle(b *testing.B) {
	for n := 0; n < b.N; n++ {
		cb := &testCallback{successNum: 4}
		mq := NewMatchQueue(cb, 100, 5*time.Second)

		// 消费端
		go func() {
			for range mq.Success() {
			}
		}()

		// 添加玩家
		for i := uint64(0); i < 100; i++ {
			mq.Add(&MatchPlayer{UserID: i})
		}

		// 删除部分玩家
		for i := uint64(50); i < 60; i++ {
			mq.Del(i)
		}

		mq.Close()
	}
}

func BenchmarkConcurrentAdd(b *testing.B) {
	cb := &testCallback{successNum: 4}
	mq := NewMatchQueue(cb, 1000, 10*time.Second)
	defer mq.Close()

	go func() {
		for range mq.Success() {
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var id uint64
		for pb.Next() {
			id++
			mq.Add(&MatchPlayer{UserID: id})
		}
	})
}
