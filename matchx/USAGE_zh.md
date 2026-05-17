# matchx 使用说明

`matchx` 是一个游戏匹配队列实现，支持真人玩家与 AI 机器人（Robot）混合匹配。

## 核心概念

- **匹配队列**：玩家加入队列后等待凑齐人数，凑齐则立即成组；超时未凑齐则由机器人补齐。
- **真人优先**：每当有新玩家加入，优先尝试与队列中的真人组队。
- **超时兜底**：玩家等待超过 `realTimeout` 后，自动与机器人组成一队，避免无限等待。
- **仅机器人匹配**：设置 `RobotOnly = true` 的玩家不入队等待，直接召唤机器人成组，适用于低活跃度时段或特定场景。

## 类型说明

### MatchPlayer

```go
type MatchPlayer struct {
    UserID    uint64      // 玩家唯一标识
    UserData  interface{} // 玩家自定义数据
    IsRobot   bool        // 是否为机器人
    RobotOnly bool        // 仅与机器人匹配，跳过真人排队
}
```

### MatchGroup

匹配成功后的队伍：

```go
type MatchGroup struct {
    Players []*MatchPlayer
}
```

### MatchCallback

调用方需要实现的回调接口：

```go
type MatchCallback interface {
    // 召唤指定数量的机器人
    CallRobots(num int) ([]*MatchPlayer, error)

    // 返回匹配所需人数（真人+机器人）
    GetSuccessNum() int
}
```

- `GetSuccessNum()` 返回每队需要的总人数。
- `CallRobots(num)` 在真人超时时被调用，用于生成机器人补齐队伍。返回的机器人数量应等于 `num`。

## 快速开始

### 1. 实现 MatchCallback

```go
type MyMatchCallback struct{}

func (c *MyMatchCallback) GetSuccessNum() int {
    return 5 // 每队5人
}

func (c *MyMatchCallback) CallRobots(num int) ([]*MatchPlayer, error) {
    robots := make([]*MatchPlayer, num)
    for i := 0; i < num; i++ {
        robots[i] = &MatchPlayer{
            UserID:   robotIDGen.Next(), // 你的机器人 ID 生成器
            UserData: generateRobotData(),
            IsRobot:  true,
        }
    }
    return robots, nil
}
```

### 2. 创建匹配队列

```go
callback := &MyMatchCallback{}
successBuffer := 10              // 成功匹配缓冲大小
realTimeout := 30 * time.Second  // 真人等待超时时间

mq := matchx.NewMatchQueue(callback, successBuffer, realTimeout)
```

参数说明：
- `callback`：实现了 `MatchCallback` 的对象
- `successBuffer`：成功匹配 channel 的缓冲容量，用于削峰
- `realTimeout`：真人玩家在队列中的最大等待时间

### 3. 消费匹配结果

```go
go func() {
    for group := range mq.Success() {
        handleMatchGroup(group)
    }
}()
```

### 4. 管理玩家

```go
// 玩家加入匹配
mq.Add(&matchx.MatchPlayer{
    UserID:   playerID,
    UserData: playerData,
    IsRobot:  false,
})

// 玩家取消匹配
mq.Del(playerID)

// 关闭匹配队列（一般在服务关闭时调用）
mq.Close()
```

- `Add` 和 `Del` 是并发安全的，可以在任意 goroutine 中调用。
- `Close()` 关闭队列后，内部的 `run()` goroutine 会退出，`Success()` channel 也会随之关闭。
- 设置 `RobotOnly: true` 的玩家不会进入等待队列，而是立即与机器人组成队伍。适用于新手引导、低峰期保底等场景。

```go
// 仅机器人匹配的玩家
mq.Add(&matchx.MatchPlayer{
    UserID:    playerID,
    UserData:  playerData,
    RobotOnly: true,
})
```

## 完整示例

```go
package main

import (
    "fmt"
    "sync/atomic"
    "time"

    "github.com/trainking/lulu-ext/matchx"
)

type Callback struct {
    robotSeq uint64
}

func (c *Callback) GetSuccessNum() int { return 3 }

func (c *Callback) CallRobots(num int) ([]*matchx.MatchPlayer, error) {
    robots := make([]*matchx.MatchPlayer, num)
    for i := 0; i < num; i++ {
        id := atomic.AddUint64(&c.robotSeq, 1)
        robots[i] = &matchx.MatchPlayer{
            UserID:  id,
            IsRobot: true,
        }
    }
    return robots, nil
}

func main() {
    cb := &Callback{}
    mq := matchx.NewMatchQueue(cb, 10, 5*time.Second)

    // 消费匹配结果
    go func() {
        for group := range mq.Success() {
            fmt.Printf("匹配成功: %d 人\n", len(group.Players))
            for _, p := range group.Players {
                if p.IsRobot {
                    fmt.Printf("  机器人 %d\n", p.UserID)
                } else {
                    fmt.Printf("  玩家 %d\n", p.UserID)
                }
            }
        }
    }()

    // 加入3个真人玩家，凑够一队
    mq.Add(&matchx.MatchPlayer{UserID: 1})
    mq.Add(&matchx.MatchPlayer{UserID: 2})
    mq.Add(&matchx.MatchPlayer{UserID: 3})

    // RobotOnly 玩家直接与机器人成组，不入队等待
    mq.Add(&matchx.MatchPlayer{UserID: 4, RobotOnly: true})

    time.Sleep(time.Second) // 等待匹配完成

    mq.Close()
}
```

## 匹配流程

```
玩家加入 → RobotOnly ?
              ├── 是 → 直接召唤机器人成组 → 输出到 Success channel
              └── 否 → 入队 → 队列人数 ≥ successNum ?
                                ├── 是 → 真人成组 → 输出到 Success channel
                                └── 否 → 等待更多玩家或超时

超时定时器触发 → 逐个取出玩家 → 用机器人补齐 → 输出到 Success channel
```

## 注意事项

1. **及时消费 Success channel**：channel 缓冲区满后，发送方会在 `Close()` 时优雅退出而非阻塞，但在此之前匹配结果可能丢失。建议使用独立的 goroutine 持续读取。
2. **CallRobots 需返回足够数量**：返回数量不等于 `num` 时，该次匹配会被丢弃，玩家**不会**回到队列。
3. **Add/Del 不会阻塞调用方**：它们只往 channel 写入，由 `run()` goroutine 异步处理。
