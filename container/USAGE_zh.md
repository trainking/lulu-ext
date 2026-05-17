# container 使用说明

`container` 定义了一些在游戏服务中常用的容器数据结构。

## RankBoard 排行榜

按分数降序排列的排行榜容器，支持并发安全访问。分数相同时按插入先后顺序排列（先插入排名靠前）。

### RankEntry 接口

调用方需实现此接口，将业务数据适配到排行榜：

```go
type RankEntry interface {
    GetID() uint64  // 唯一标识
    GetScore() int64 // 排序分数
}
```

实现示例：

```go
type Player struct {
    UID   uint64
    Score int64
    Name  string
}

func (p *Player) GetID() uint64  { return p.UID }
func (p *Player) GetScore() int64 { return p.Score }
```

### 方法一览

| 方法 | 说明 |
|------|------|
| `NewRankBoard()` | 创建排行榜 |
| `AddOrUpdate(entry)` | 添加或更新条目（ID 存在则覆盖并重排） |
| `Remove(id)` | 移除条目 |
| `GetRank(id)` | 获取排名（1-based），不存在返回 0 |
| `GetTop(n)` | 获取前 N 名 |
| `GetRange(start, end)` | 获取区间排名（闭区间） |
| `GetLast()` | 获取最后一名，空榜返回 nil, false |
| `Len()` | 返回条目总数 |

### 快速开始

```go
rb := container.NewRankBoard()

// 添加条目
rb.AddOrUpdate(&Player{UID: 1, Score: 300, Name: "alice"})
rb.AddOrUpdate(&Player{UID: 2, Score: 100, Name: "bob"})
rb.AddOrUpdate(&Player{UID: 3, Score: 200, Name: "charlie"})

// 查询排名
rank := rb.GetRank(1) // alice 排名第1
fmt.Println(rank)     // 1

// 获取前2名
top2 := rb.GetTop(2)
for _, e := range top2 {
    p := e.(*Player)
    fmt.Printf("%s: %d\n", p.Name, p.Score)
}
// alice: 300
// charlie: 200

// 获取第2-3名
range2to3 := rb.GetRange(2, 3)
for _, e := range range2to3 {
    p := e.(*Player)
    fmt.Printf("%s: %d\n", p.Name, p.Score)
}
// charlie: 200
// bob: 100

// 获取最后一名
last, ok := rb.GetLast()
if ok {
    fmt.Println(last.(*Player).Name) // bob
}

// 更新分数
rb.AddOrUpdate(&Player{UID: 2, Score: 400, Name: "bob"})
fmt.Println(rb.GetRank(2)) // 1 (bob 升至第1)

// 移除条目
rb.Remove(3)
fmt.Println(rb.Len()) // 2
```

### 并发安全

所有方法均使用 `sync.RWMutex` 保护，可在多个 goroutine 中安全使用。读操作（`GetRank`、`GetTop`、`GetRange`、`GetLast`、`Len`）使用读锁，写操作（`AddOrUpdate`、`Remove`）使用写锁。

### 复杂度

| 操作 | 时间复杂度 |
|------|-----------|
| `AddOrUpdate` | O(n) |
| `Remove` | O(n) |
| `GetRank` | O(1) |
| `GetTop` | O(k)，k 为返回数量 |
| `GetRange` | O(k) |
| `GetLast` | O(1) |
| `Len` | O(1) |

> 注：排行榜适用于条目数在万级以内的场景。若数据量更大，建议使用跳表或堆实现。
